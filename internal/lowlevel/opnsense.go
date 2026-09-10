package lowlevel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
)

// OpnsenseApiClient provides HTTP client functionality for the OPNsense REST API.
//
// Unlike the pfSense backend, which targets the third-party pfSense-API package
// (https://pfrest.org/), the endpoints used here are part of OPNsense core: a
// stock install answers /api/wireguard/* with no plugin to install.
//
// Two conventions differ from pfSense and drive the shape of this client:
//
//  1. Authentication is an API key/secret pair sent as HTTP Basic credentials,
//     not a single header value.
//  2. Model-backed controllers are asymmetric between reads and writes. A
//     getXxx returns "select" fields as maps keyed by option, each carrying a
//     `selected` flag; a POST expects the same field as a comma-joined scalar,
//     and posting back what getXxx returned fails with an opaque HTTP 500.
//     FlattenForWrite converts between the two. Note the searchXxx endpoints
//     are the exception: they already return the flattened form, which is why
//     the controller reads through search and only needs FlattenForWrite where
//     it round-trips a whole record back into a write.

// region models

const (
	OpnsenseApiStatusOk    = "ok"
	OpnsenseApiStatusError = "error"
)

const (
	OpnsenseApiErrorCodeUnknown = iota + 800
	OpnsenseApiErrorCodeRequestPreparationFailed
	OpnsenseApiErrorCodeRequestFailed
	OpnsenseApiErrorCodeResponseDecodeFailed
	OpnsenseApiErrorCodeValidationFailed
)

type OpnsenseApiResponse[T any] struct {
	Status string
	Code   int
	Data   T
	Error  *OpnsenseApiError
}

type OpnsenseApiError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"detail,omitempty"`
}

func (e *OpnsenseApiError) String() string {
	if e == nil {
		return "no error"
	}
	return fmt.Sprintf("API error %d: %s - %s", e.Code, e.Message, e.Details)
}

// OpnsenseSearchResult models the bootgrid-style payload returned by the
// searchXxx endpoints.
type OpnsenseSearchResult struct {
	Rows     []GenericJsonObject `json:"rows"`
	RowCount int                 `json:"rowCount"`
	Total    int                 `json:"total"`
	Current  int                 `json:"current"`
}

// endregion models

// region select-field conversion

// FlattenForWrite converts an object as returned by a getXxx endpoint into the
// form a setXxx/addXxx endpoint accepts.
//
// OPNsense renders select fields as:
//
//	"tunneladdress": {"10.0.0.1/24": {"value": "10.0.0.1/24", "selected": 1}}
//
// but only accepts them on write as:
//
//	"tunneladdress": "10.0.0.1/24"
//
// Feeding the read form straight back produces HTTP 500 with
// "Unexpected error, check log for details", which gives no hint as to the
// cause. Selected keys are joined with "," in sorted order so that writes are
// deterministic; Go map iteration would otherwise reorder multi-value fields
// on every save.
func FlattenForWrite(obj GenericJsonObject) GenericJsonObject {
	out := make(GenericJsonObject, len(obj))
	for key, value := range obj {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = joinSelected(typed)
		case []any:
			parts := make([]string, 0, len(typed))
			for _, item := range typed {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			out[key] = strings.Join(parts, ",")
		default:
			out[key] = value
		}
	}
	return out
}

// joinSelected extracts the keys of a select map whose option is marked
// selected. The empty key is OPNsense's "nothing chosen" placeholder and is
// never a real value.
func joinSelected(options map[string]any) string {
	selected := make([]string, 0, len(options))
	for key, raw := range options {
		if key == "" {
			continue
		}
		option, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if isSelected(option["selected"]) {
			selected = append(selected, key)
		}
	}
	sort.Strings(selected)
	return strings.Join(selected, ",")
}

// isSelected copes with the several shapes OPNsense uses for the flag: it
// arrives as a JSON number through encoding/json, but has also been observed as
// a bare boolean and as a quoted string.
func isSelected(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}

// SelectedKeys returns the selected option keys of a select field read from a
// getXxx response, in sorted order. Returns nil when the field is absent or is
// not a select map.
func SelectedKeys(obj GenericJsonObject, field string) []string {
	raw, ok := obj[field]
	if !ok {
		return nil
	}
	options, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	joined := joinSelected(options)
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ",")
}

// SelectedValue returns the single selected key of a select field, or "" when
// nothing is selected. Convenience for fields that are logically scalar.
func SelectedValue(obj GenericJsonObject, field string) string {
	keys := SelectedKeys(obj, field)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// endregion select-field conversion

// region API-client

type OpnsenseApiClient struct {
	coreCfg *config.Config
	cfg     *config.BackendOpnsense

	client *http.Client
	log    *slog.Logger
}

func NewOpnsenseApiClient(coreCfg *config.Config, cfg *config.BackendOpnsense) (*OpnsenseApiClient, error) {
	if cfg.ApiUrl == "" {
		return nil, fmt.Errorf("no API URL configured for OPNsense backend %s", cfg.Id)
	}
	if cfg.ApiKey == "" || cfg.ApiSecret == "" {
		return nil, fmt.Errorf("both api_key and api_secret are required for OPNsense backend %s", cfg.Id)
	}

	c := &OpnsenseApiClient{
		coreCfg: coreCfg,
		cfg:     cfg,
	}

	if err := c.setup(); err != nil {
		return nil, err
	}

	c.debugLog("OPNsense api client created", "api_url", cfg.ApiUrl)

	return c, nil
}

func (o *OpnsenseApiClient) setup() error {
	o.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !o.cfg.ApiVerifyTls,
			},
		},
		Timeout: o.cfg.GetApiTimeout(),
	}

	if o.cfg.Debug {
		o.log = slog.New(internal.GetLoggingHandler("debug",
			o.coreCfg.Advanced.LogPretty,
			o.coreCfg.Advanced.LogJson).
			WithAttrs([]slog.Attr{
				{
					Key: "opnsense-bid", Value: slog.StringValue(o.cfg.Id),
				},
			}))
	}

	return nil
}

func (o *OpnsenseApiClient) debugLog(msg string, args ...any) {
	if o.log != nil {
		o.log.Debug("[OPN-API] "+msg, args...)
	}
}

func (o *OpnsenseApiClient) getFullPath(command string) (string, error) {
	// url.JoinPath treats its arguments as path elements and percent-encodes
	// "?" and "&", which would turn a query string into a literal (and
	// therefore unroutable) path segment. Split the query off, join the path,
	// then re-attach it.
	rawPath, rawQuery, hasQuery := strings.Cut(command, "?")

	// url.JoinPath also *resolves* "." and ".." segments. Callers interpolate
	// record UUIDs taken from firewall responses into these commands, so a
	// crafted response could otherwise walk an authenticated POST out of the
	// /api/wireguard/ namespace and aim it at an unrelated endpoint. Callers
	// escape those values; refuse traversal here as well so that a single
	// missed call site cannot turn into a redirected request.
	for _, segment := range strings.Split(rawPath, "/") {
		if segment == ".." {
			return "", fmt.Errorf("refusing path traversal in API command %q", command)
		}
	}

	path, err := url.JoinPath(o.cfg.ApiUrl, rawPath)
	if err != nil {
		return "", fmt.Errorf("failed to build request URL for %q: %w", command, err)
	}
	if hasQuery && rawQuery != "" {
		path += "?" + rawQuery
	}
	return path, nil
}

func (o *OpnsenseApiClient) prepareRequest(
	ctx context.Context,
	method, fullUrl string,
	payload any,
) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		o.debugLog("prepared payload", "payload", redactOpnsenseSecrets(payload))
		body = bytes.NewReader(payloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullUrl, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// OPNsense API keys authenticate as HTTP Basic key:secret.
	req.SetBasicAuth(o.cfg.ApiKey, o.cfg.ApiSecret)

	return req, nil
}

// opnsenseSecretFields are the request fields whose values must never reach a
// log. The WireGuard interface private key and the per-peer preshared key are
// both stored encrypted at rest by wg-portal (see the `serializer:encstr` tags
// in internal/domain), so writing them to a log stream in cleartext would put
// them at a lower level of protection than the database they came from.
var opnsenseSecretFields = map[string]struct{}{
	"privkey": {},
	"psk":     {},
}

// redactOpnsenseSecrets renders a request payload for logging with secret
// values replaced. It reports the field names, since knowing *which* fields
// were sent is the useful part when debugging a rejected write.
func redactOpnsenseSecrets(payload any) string {
	obj, ok := payload.(GenericJsonObject)
	if !ok {
		return "REDACTED-non-object-payload"
	}

	safe := make(GenericJsonObject, len(obj))
	for key, value := range obj {
		// The models nest the record under a single key, e.g. {"server": {...}}.
		if nested, isNested := value.(GenericJsonObject); isNested {
			inner := make(GenericJsonObject, len(nested))
			for field, fieldValue := range nested {
				if _, secret := opnsenseSecretFields[field]; secret && fmt.Sprintf("%v", fieldValue) != "" {
					inner[field] = "REDACTED"
				} else {
					inner[field] = fieldValue
				}
			}
			safe[key] = inner
			continue
		}
		if _, secret := opnsenseSecretFields[key]; secret && fmt.Sprintf("%v", value) != "" {
			safe[key] = "REDACTED"
			continue
		}
		safe[key] = value
	}

	encoded, err := json.Marshal(safe)
	if err != nil {
		return "REDACTED-unrenderable-payload"
	}
	return string(encoded)
}

func errToOpnsenseApiResponse[T any](code int, message string, err error) OpnsenseApiResponse[T] {
	details := ""
	if err != nil {
		details = err.Error()
	}
	return OpnsenseApiResponse[T]{
		Status: OpnsenseApiStatusError,
		Code:   code,
		Error: &OpnsenseApiError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// parseOpnsenseHttpResponse decodes a response body into T.
//
// OPNsense does not wrap payloads in a common envelope the way the pfSense REST
// package does: getXxx returns the record directly, addXxx/setXxx return
// {"result": "saved"} or {"result": "failed", "validations": {...}}. A
// validation failure is reported with HTTP 200, so the result field has to be
// inspected rather than relying on the status code alone.
func parseOpnsenseHttpResponse[T any](resp *http.Response, err error) OpnsenseApiResponse[T] {
	if err != nil {
		return errToOpnsenseApiResponse[T](OpnsenseApiErrorCodeRequestFailed, "failed to execute request", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("failed to close response body", "error", closeErr)
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errToOpnsenseApiResponse[T](OpnsenseApiErrorCodeResponseDecodeFailed,
			"failed to read response body", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(bodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return errToOpnsenseApiResponse[T](resp.StatusCode,
			fmt.Sprintf("HTTP %d from %s", resp.StatusCode, resp.Request.URL.Path),
			fmt.Errorf("%s", preview))
	}

	if len(bodyBytes) == 0 {
		return OpnsenseApiResponse[T]{Status: OpnsenseApiStatusOk, Code: resp.StatusCode}
	}

	var data T
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		preview := string(bodyBytes)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		slog.Error("failed to decode OPNsense API response",
			"status_code", resp.StatusCode,
			"content_type", resp.Header.Get("Content-Type"),
			"url", resp.Request.URL.String(),
			"method", resp.Request.Method,
			"body_preview", preview,
			"error", err)
		return errToOpnsenseApiResponse[T](OpnsenseApiErrorCodeResponseDecodeFailed,
			fmt.Sprintf("failed to decode response (status %d)", resp.StatusCode), err)
	}

	return OpnsenseApiResponse[T]{Status: OpnsenseApiStatusOk, Code: resp.StatusCode, Data: data}
}

// checkMutationResult inspects the {"result": ...} body common to add/set/del
// and toggles the response to an error when OPNsense reports a failure. These
// arrive as HTTP 200, so without this a failed validation looks like success.
func checkMutationResult(resp OpnsenseApiResponse[GenericJsonObject]) OpnsenseApiResponse[GenericJsonObject] {
	if resp.Status != OpnsenseApiStatusOk {
		return resp
	}
	if resp.Data == nil {
		return resp
	}

	result := resp.Data.GetString("result")
	switch result {
	case "saved", "deleted", "ok", "":
		return resp
	}

	details := result
	if validations, ok := resp.Data["validations"]; ok {
		if encoded, err := json.Marshal(validations); err == nil {
			details = fmt.Sprintf("%s: %s", result, string(encoded))
		}
	}

	return OpnsenseApiResponse[GenericJsonObject]{
		Status: OpnsenseApiStatusError,
		Code:   OpnsenseApiErrorCodeValidationFailed,
		Data:   resp.Data,
		Error: &OpnsenseApiError{
			Code:    OpnsenseApiErrorCodeValidationFailed,
			Message: "OPNsense rejected the request",
			Details: details,
		},
	}
}

// opnsenseDo is a package-level generic because Go does not permit type
// parameters on methods, and the response type varies per endpoint family.
func opnsenseDo[T any](
	o *OpnsenseApiClient,
	ctx context.Context,
	method, command string,
	payload any,
) OpnsenseApiResponse[T] {
	apiCtx, cancel := context.WithTimeout(ctx, o.cfg.GetApiTimeout())
	defer cancel()

	fullUrl, err := o.getFullPath(command)
	if err != nil {
		return errToOpnsenseApiResponse[T](OpnsenseApiErrorCodeRequestPreparationFailed,
			"failed to build request URL", err)
	}

	req, err := o.prepareRequest(apiCtx, method, fullUrl, payload)
	if err != nil {
		return errToOpnsenseApiResponse[T](OpnsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	o.debugLog("executing API request", "method", method, "url", fullUrl)
	response := parseOpnsenseHttpResponse[T](o.client.Do(req))
	o.debugLog("retrieved API result",
		"method", method, "url", fullUrl, "duration", time.Since(start).String())
	return response
}

// Search calls a searchXxx endpoint and returns the matched rows.
func (o *OpnsenseApiClient) Search(ctx context.Context, command string) OpnsenseApiResponse[OpnsenseSearchResult] {
	return opnsenseDo[OpnsenseSearchResult](o, ctx, http.MethodGet, command, nil)
}

// Get calls a getXxx endpoint. The returned object is in read form; run it
// through FlattenForWrite before sending it back.
func (o *OpnsenseApiClient) Get(ctx context.Context, command string) OpnsenseApiResponse[GenericJsonObject] {
	return opnsenseDo[GenericJsonObject](o, ctx, http.MethodGet, command, nil)
}

// Post calls a mutating endpoint (addXxx, setXxx, delXxx, reconfigure).
func (o *OpnsenseApiClient) Post(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) OpnsenseApiResponse[GenericJsonObject] {
	if payload == nil {
		payload = GenericJsonObject{}
	}
	return checkMutationResult(opnsenseDo[GenericJsonObject](o, ctx, http.MethodPost, command, payload))
}

// endregion API-client
