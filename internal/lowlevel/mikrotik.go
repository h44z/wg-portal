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
	"strconv"
	"strings"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
)

// region models

const (
	MikrotikApiStatusOk    = "success"
	MikrotikApiStatusError = "error"
)

const (
	MikrotikApiErrorCodeUnknown = iota + 600
	MikrotikApiErrorCodeRequestPreparationFailed
	MikrotikApiErrorCodeRequestFailed
	MikrotikApiErrorCodeResponseDecodeFailed
)

type MikrotikApiResponse[T any] struct {
	Status string
	Code   int
	Data   T                 `json:"data,omitempty"`
	Error  *MikrotikApiError `json:"error,omitempty"`
}

type MikrotikApiError struct {
	Code    int    `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"detail,omitempty"`
}

func (e *MikrotikApiError) String() string {
	if e == nil {
		return "no error"
	}
	return fmt.Sprintf("API error %d: %s - %s", e.Code, e.Message, e.Details)
}

type GenericJsonObject map[string]any
type EmptyResponse struct{}

func (JsonObject GenericJsonObject) GetString(key string) string {
	if value, ok := JsonObject[key]; ok {
		if strValue, ok := value.(string); ok {
			return strValue
		} else {
			return fmt.Sprintf("%v", value) // Convert to string if not already
		}
	}
	return ""
}

func (JsonObject GenericJsonObject) GetInt(key string) int {
	if value, ok := JsonObject[key]; ok {
		if intValue, ok := value.(int); ok {
			return intValue
		} else {
			if floatValue, ok := value.(float64); ok {
				return int(floatValue) // Convert float64 to int
			}
			if strValue, ok := value.(string); ok {
				if intValue, err := strconv.Atoi(strValue); err == nil {
					return intValue // Convert string to int if possible
				}
			}
		}
	}
	return 0
}

func (JsonObject GenericJsonObject) GetBool(key string) bool {
	if value, ok := JsonObject[key]; ok {
		if boolValue, ok := value.(bool); ok {
			return boolValue
		} else {
			if intValue, ok := value.(int); ok {
				return intValue == 1 // Convert int to bool (1 is true, 0 is false)
			}
			if floatValue, ok := value.(float64); ok {
				return int(floatValue) == 1 // Convert float64 to bool (1.0 is true, 0.0 is false)
			}
			if strValue, ok := value.(string); ok {
				boolValue, err := strconv.ParseBool(strValue)
				if err == nil {
					return boolValue
				}
			}
		}
	}
	return false
}

type MikrotikRequestOptions struct {
	Filters  map[string]string `json:"filters,omitempty"`
	PropList []string          `json:"proplist,omitempty"`
}

func (o *MikrotikRequestOptions) GetPath(base string) string {
	if o == nil {
		return base
	}

	path, err := url.Parse(base)
	if err != nil {
		return base
	}

	query := path.Query()
	for k, v := range o.Filters {
		query.Set(k, v)
	}
	if len(o.PropList) > 0 {
		query.Set(".proplist", strings.Join(o.PropList, ","))
	}
	path.RawQuery = query.Encode()
	return path.String()
}

// region models

// region API-client

type MikrotikApiClient struct {
	coreCfg *config.Config
	cfg     *config.BackendMikrotik

	client *http.Client
	log    *slog.Logger
}

func NewMikrotikApiClient(coreCfg *config.Config, cfg *config.BackendMikrotik) (*MikrotikApiClient, error) {
	c := &MikrotikApiClient{
		coreCfg: coreCfg,
		cfg:     cfg,
	}

	err := c.setup()
	if err != nil {
		return nil, err
	}

	c.debugLog("Mikrotik api client created", "api_url", cfg.ApiUrl)

	return c, nil
}

func (m *MikrotikApiClient) setup() error {
	m.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !m.cfg.ApiVerifyTls,
			},
		},
		Timeout: m.cfg.GetApiTimeout(),
	}

	if m.cfg.Debug {
		m.log = slog.New(internal.GetLoggingHandler("debug",
			m.coreCfg.Advanced.LogPretty,
			m.coreCfg.Advanced.LogJson).
			WithAttrs([]slog.Attr{
				{
					Key: "mikrotik-bid", Value: slog.StringValue(m.cfg.Id),
				},
			}))
	}

	return nil
}

func (m *MikrotikApiClient) debugLog(msg string, args ...any) {
	if m.log != nil {
		m.log.Debug("[MT-API] "+msg, args...)
	}
}

func (m *MikrotikApiClient) getFullPath(command string) string {
	path, err := url.JoinPath(m.cfg.ApiUrl, command)
	if err != nil {
		return ""
	}
	return path
}

func (m *MikrotikApiClient) prepareGetRequest(ctx context.Context, fullUrl string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if m.cfg.ApiUser != "" && m.cfg.ApiPassword != "" {
		req.SetBasicAuth(m.cfg.ApiUser, m.cfg.ApiPassword)
	}

	return req, nil
}

func (m *MikrotikApiClient) prepareDeleteRequest(ctx context.Context, fullUrl string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if m.cfg.ApiUser != "" && m.cfg.ApiPassword != "" {
		req.SetBasicAuth(m.cfg.ApiUser, m.cfg.ApiPassword)
	}

	return req, nil
}

func (m *MikrotikApiClient) preparePayloadRequest(
	ctx context.Context,
	method string,
	fullUrl string,
	payload GenericJsonObject,
) (*http.Request, error) {
	// marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullUrl, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if m.cfg.ApiUser != "" && m.cfg.ApiPassword != "" {
		req.SetBasicAuth(m.cfg.ApiUser, m.cfg.ApiPassword)
	}

	return req, nil
}

func errToApiResponse[T any](code int, message string, err error) MikrotikApiResponse[T] {
	return MikrotikApiResponse[T]{
		Status: MikrotikApiStatusError,
		Code:   code,
		Error: &MikrotikApiError{
			Code:    code,
			Message: message,
			Details: err.Error(),
		},
	}
}

func parseHttpResponse[T any](resp *http.Response, err error) MikrotikApiResponse[T] {
	if err != nil {
		return errToApiResponse[T](MikrotikApiErrorCodeRequestFailed, "failed to execute request", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data T

		// if the type of T is EmptyResponse, we can return an empty response with just the status
		if _, ok := any(data).(EmptyResponse); ok {
			return MikrotikApiResponse[T]{Status: MikrotikApiStatusOk, Code: resp.StatusCode}
		}

		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return errToApiResponse[T](MikrotikApiErrorCodeResponseDecodeFailed, "failed to decode response", err)
		}
		return MikrotikApiResponse[T]{Status: MikrotikApiStatusOk, Code: resp.StatusCode, Data: data}
	}

	var apiErr MikrotikApiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return errToApiResponse[T](resp.StatusCode, "unknown error, unparsable response", err)
	} else {
		return MikrotikApiResponse[T]{Status: MikrotikApiStatusError, Code: resp.StatusCode, Error: &apiErr}
	}
}

func (m *MikrotikApiClient) Query(
	ctx context.Context,
	command string,
	opts *MikrotikRequestOptions,
) MikrotikApiResponse[[]GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := opts.GetPath(m.getFullPath(command))

	req, err := m.prepareGetRequest(apiCtx, fullUrl)
	if err != nil {
		return errToApiResponse[[]GenericJsonObject](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API query", "url", fullUrl)
	response := parseHttpResponse[[]GenericJsonObject](m.client.Do(req))
	m.debugLog("retrieved API query result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (m *MikrotikApiClient) Get(
	ctx context.Context,
	command string,
	opts *MikrotikRequestOptions,
) MikrotikApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := opts.GetPath(m.getFullPath(command))

	req, err := m.prepareGetRequest(apiCtx, fullUrl)
	if err != nil {
		return errToApiResponse[GenericJsonObject](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API get", "url", fullUrl)
	response := parseHttpResponse[GenericJsonObject](m.client.Do(req))
	m.debugLog("retrieved API get result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (m *MikrotikApiClient) Create(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) MikrotikApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := m.getFullPath(command)

	req, err := m.preparePayloadRequest(apiCtx, http.MethodPut, fullUrl, payload)
	if err != nil {
		return errToApiResponse[GenericJsonObject](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API put", "url", fullUrl)
	response := parseHttpResponse[GenericJsonObject](m.client.Do(req))
	m.debugLog("retrieved API put result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (m *MikrotikApiClient) Update(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) MikrotikApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := m.getFullPath(command)

	req, err := m.preparePayloadRequest(apiCtx, http.MethodPatch, fullUrl, payload)
	if err != nil {
		return errToApiResponse[GenericJsonObject](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API patch", "url", fullUrl)
	response := parseHttpResponse[GenericJsonObject](m.client.Do(req))
	m.debugLog("retrieved API patch result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (m *MikrotikApiClient) Delete(
	ctx context.Context,
	command string,
) MikrotikApiResponse[EmptyResponse] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := m.getFullPath(command)

	req, err := m.prepareDeleteRequest(apiCtx, fullUrl)
	if err != nil {
		return errToApiResponse[EmptyResponse](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API delete", "url", fullUrl)
	response := parseHttpResponse[EmptyResponse](m.client.Do(req))
	m.debugLog("retrieved API delete result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (m *MikrotikApiClient) ExecList(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) MikrotikApiResponse[[]GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, m.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := m.getFullPath(command)

	req, err := m.preparePayloadRequest(apiCtx, http.MethodPost, fullUrl, payload)
	if err != nil {
		return errToApiResponse[[]GenericJsonObject](MikrotikApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	m.debugLog("executing API post", "url", fullUrl)
	response := parseHttpResponse[[]GenericJsonObject](m.client.Do(req))
	m.debugLog("retrieved API post result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

// endregion API-client
