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
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
)

// PfsenseApiClient provides HTTP client functionality for interacting with the pfSense REST API.
// Documentation: https://pfrest.org/
// Swagger UI: https://pfrest.org/api-docs/

// region models

const (
	PfsenseApiStatusOk    = "ok"    // pfSense REST API uses "ok" in response
	PfsenseApiStatusError = "error"
)

const (
	PfsenseApiErrorCodeUnknown = iota + 700
	PfsenseApiErrorCodeRequestPreparationFailed
	PfsenseApiErrorCodeRequestFailed
	PfsenseApiErrorCodeResponseDecodeFailed
)

type PfsenseApiResponse[T any] struct {
	Status string
	Code   int
	Data   T                 `json:"data,omitempty"`
	Error  *PfsenseApiError  `json:"error,omitempty"`
}

type PfsenseApiError struct {
	Code    int    `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"detail,omitempty"`
}

func (e *PfsenseApiError) String() string {
	if e == nil {
		return "no error"
	}
	return fmt.Sprintf("API error %d: %s - %s", e.Code, e.Message, e.Details)
}

type PfsenseRequestOptions struct {
	Filters  map[string]string `json:"filters,omitempty"`
	PropList []string          `json:"proplist,omitempty"`
}

func (o *PfsenseRequestOptions) GetPath(base string) string {
	if o == nil {
		return base
	}

	path, err := url.Parse(base)
	if err != nil {
		return base
	}

	query := path.Query()
	// pfSense REST API uses standard query parameters for filtering
	for k, v := range o.Filters {
		query.Set(k, v)
	}
	// Note: PropList may not be supported by pfSense REST API in the same way as Mikrotik
	// pfSense typically returns all fields by default, but we keep this for potential future use
	// Verify the correct parameter name in Swagger docs if field selection is needed
	if len(o.PropList) > 0 {
		// pfSense might use different parameter name - verify in Swagger docs
		// For now, we'll skip it as pfSense may return all fields by default
		// query.Set("fields", strings.Join(o.PropList, ","))
	}
	path.RawQuery = query.Encode()
	return path.String()
}

// endregion models

// region API-client

type PfsenseApiClient struct {
	coreCfg *config.Config
	cfg     *config.BackendPfsense

	client *http.Client
	log    *slog.Logger
}

func NewPfsenseApiClient(coreCfg *config.Config, cfg *config.BackendPfsense) (*PfsenseApiClient, error) {
	c := &PfsenseApiClient{
		coreCfg: coreCfg,
		cfg:     cfg,
	}

	err := c.setup()
	if err != nil {
		return nil, err
	}

	c.debugLog("pfSense api client created", "api_url", cfg.ApiUrl)

	return c, nil
}

func (p *PfsenseApiClient) setup() error {
	p.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !p.cfg.ApiVerifyTls,
			},
		},
		Timeout: p.cfg.GetApiTimeout(),
	}

	if p.cfg.Debug {
		p.log = slog.New(internal.GetLoggingHandler("debug",
			p.coreCfg.Advanced.LogPretty,
			p.coreCfg.Advanced.LogJson).
			WithAttrs([]slog.Attr{
				{
					Key: "pfsense-bid", Value: slog.StringValue(p.cfg.Id),
				},
			}))
	}

	return nil
}

func (p *PfsenseApiClient) debugLog(msg string, args ...any) {
	if p.log != nil {
		p.log.Debug("[PFS-API] "+msg, args...)
	}
}

func (p *PfsenseApiClient) getFullPath(command string) string {
	path, err := url.JoinPath(p.cfg.ApiUrl, command)
	if err != nil {
		return ""
	}
	return path
}

func (p *PfsenseApiClient) prepareGetRequest(ctx context.Context, fullUrl string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if p.cfg.ApiKey != "" {
		// pfSense REST API API Key authentication (https://pfrest.org/AUTHENTICATION_AND_AUTHORIZATION/)
		// Uses X-API-Key header for API key authentication
		req.Header.Set("X-API-Key", p.cfg.ApiKey)
	}

	return req, nil
}

func (p *PfsenseApiClient) prepareDeleteRequest(ctx context.Context, fullUrl string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if p.cfg.ApiKey != "" {
		// pfSense REST API API Key authentication (https://pfrest.org/AUTHENTICATION_AND_AUTHORIZATION/)
		// Uses X-API-Key header for API key authentication
		req.Header.Set("X-API-Key", p.cfg.ApiKey)
	}

	return req, nil
}

func (p *PfsenseApiClient) preparePayloadRequest(
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
	if p.cfg.ApiKey != "" {
		// pfSense REST API API Key authentication (https://pfrest.org/AUTHENTICATION_AND_AUTHORIZATION/)
		// Uses X-API-Key header for API key authentication
		req.Header.Set("X-API-Key", p.cfg.ApiKey)
	}

	return req, nil
}

func errToPfsenseApiResponse[T any](code int, message string, err error) PfsenseApiResponse[T] {
	return PfsenseApiResponse[T]{
		Status: PfsenseApiStatusError,
		Code:   code,
		Error: &PfsenseApiError{
			Code:    code,
			Message: message,
			Details: err.Error(),
		},
	}
}

func parsePfsenseHttpResponse[T any](resp *http.Response, err error) PfsenseApiResponse[T] {
	if err != nil {
		return errToPfsenseApiResponse[T](PfsenseApiErrorCodeRequestFailed, "failed to execute request", err)
	}

	// pfSense REST API wraps responses in {code, status, data} or {code, status, error} structure
	var wrapper struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
		Data   T      `json:"data,omitempty"`
		Error  *struct {
			Code    int    `json:"code,omitempty"`
			Message string `json:"message,omitempty"`
			Detail  string `json:"detail,omitempty"`
		} `json:"error,omitempty"`
	}

	// Read the entire body first
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errToPfsenseApiResponse[T](PfsenseApiErrorCodeResponseDecodeFailed, "failed to read response body", err)
	}
	
	// Close the body after reading
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if len(bodyBytes) == 0 {
		// Empty response for DELETE operations
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return PfsenseApiResponse[T]{Status: PfsenseApiStatusOk, Code: resp.StatusCode}
		}
		return errToPfsenseApiResponse[T](resp.StatusCode, "empty error response", fmt.Errorf("HTTP %d", resp.StatusCode))
	}

	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		// Log the actual response for debugging when JSON parsing fails
		contentType := resp.Header.Get("Content-Type")
		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "..."
		}
		slog.Error("failed to decode pfSense API response",
			"status_code", resp.StatusCode,
			"content_type", contentType,
			"url", resp.Request.URL.String(),
			"method", resp.Request.Method,
			"body_preview", bodyPreview,
			"error", err)
		return errToPfsenseApiResponse[T](PfsenseApiErrorCodeResponseDecodeFailed, 
			fmt.Sprintf("failed to decode response (status %d, content-type: %s): %v", resp.StatusCode, contentType, err), err)
	}

	// Check if response indicates success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Map pfSense status to our status
		status := PfsenseApiStatusOk
		if wrapper.Status != "ok" && wrapper.Status != "success" {
			status = PfsenseApiStatusError
		}

		// Handle EmptyResponse type
		if _, ok := any(wrapper.Data).(EmptyResponse); ok {
			return PfsenseApiResponse[T]{Status: status, Code: wrapper.Code}
		}

		return PfsenseApiResponse[T]{Status: status, Code: wrapper.Code, Data: wrapper.Data}
	}

	// Handle error response
	if wrapper.Error != nil {
		return PfsenseApiResponse[T]{
			Status: PfsenseApiStatusError,
			Code:   wrapper.Code,
			Error: &PfsenseApiError{
				Code:    wrapper.Error.Code,
				Message: wrapper.Error.Message,
				Details: wrapper.Error.Detail,
			},
		}
	}

	// Fallback error response
	return errToPfsenseApiResponse[T](wrapper.Code, "unknown error", fmt.Errorf("HTTP %d: %s", wrapper.Code, wrapper.Status))
}

func (p *PfsenseApiClient) Query(
	ctx context.Context,
	command string,
	opts *PfsenseRequestOptions,
) PfsenseApiResponse[[]GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, p.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := opts.GetPath(p.getFullPath(command))

	req, err := p.prepareGetRequest(apiCtx, fullUrl)
	if err != nil {
		return errToPfsenseApiResponse[[]GenericJsonObject](PfsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	p.debugLog("executing API query", "url", fullUrl)
	response := parsePfsenseHttpResponse[[]GenericJsonObject](p.client.Do(req))
	p.debugLog("retrieved API query result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (p *PfsenseApiClient) Get(
	ctx context.Context,
	command string,
	opts *PfsenseRequestOptions,
) PfsenseApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, p.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := opts.GetPath(p.getFullPath(command))

	req, err := p.prepareGetRequest(apiCtx, fullUrl)
	if err != nil {
		return errToPfsenseApiResponse[GenericJsonObject](PfsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	p.debugLog("executing API get", "url", fullUrl)
	response := parsePfsenseHttpResponse[GenericJsonObject](p.client.Do(req))
	p.debugLog("retrieved API get result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (p *PfsenseApiClient) Create(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) PfsenseApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, p.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := p.getFullPath(command)

	req, err := p.preparePayloadRequest(apiCtx, http.MethodPost, fullUrl, payload)
	if err != nil {
		return errToPfsenseApiResponse[GenericJsonObject](PfsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	p.debugLog("executing API post", "url", fullUrl)
	response := parsePfsenseHttpResponse[GenericJsonObject](p.client.Do(req))
	p.debugLog("retrieved API post result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (p *PfsenseApiClient) Update(
	ctx context.Context,
	command string,
	payload GenericJsonObject,
) PfsenseApiResponse[GenericJsonObject] {
	apiCtx, cancel := context.WithTimeout(ctx, p.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := p.getFullPath(command)

	req, err := p.preparePayloadRequest(apiCtx, http.MethodPatch, fullUrl, payload)
	if err != nil {
		return errToPfsenseApiResponse[GenericJsonObject](PfsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	p.debugLog("executing API patch", "url", fullUrl)
	response := parsePfsenseHttpResponse[GenericJsonObject](p.client.Do(req))
	p.debugLog("retrieved API patch result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

func (p *PfsenseApiClient) Delete(
	ctx context.Context,
	command string,
) PfsenseApiResponse[EmptyResponse] {
	apiCtx, cancel := context.WithTimeout(ctx, p.cfg.GetApiTimeout())
	defer cancel()

	fullUrl := p.getFullPath(command)

	req, err := p.prepareDeleteRequest(apiCtx, fullUrl)
	if err != nil {
		return errToPfsenseApiResponse[EmptyResponse](PfsenseApiErrorCodeRequestPreparationFailed,
			"failed to create request", err)
	}

	start := time.Now()
	p.debugLog("executing API delete", "url", fullUrl)
	response := parsePfsenseHttpResponse[EmptyResponse](p.client.Do(req))
	p.debugLog("retrieved API delete result", "url", fullUrl, "duration", time.Since(start).String())
	return response
}

// endregion API-client

