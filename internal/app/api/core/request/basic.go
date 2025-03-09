// Package request provides functions to extract parameters from the request.
package request

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"slices"
	"strings"
)

const CheckPrivateProxy = "PRIVATE"

// PathRaw returns the value of the named path parameter.
func PathRaw(r *http.Request, name string) string {
	return r.PathValue(name)
}

// Path returns the value of the named path parameter.
// The return value is trimmed of leading and trailing whitespace.
func Path(r *http.Request, name string) string {
	return strings.TrimSpace(PathRaw(r, name))
}

// PathDefault returns the value of the named path parameter.
// If the parameter is empty, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func PathDefault(r *http.Request, name string, defaultValue string) string {
	value := r.PathValue(name)
	if value == "" {
		return defaultValue
	}

	return Path(r, name)
}

// QueryRaw returns the value of the named query parameter.
func QueryRaw(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

// Query returns the value of the named query parameter.
// The return value is trimmed of leading and trailing whitespace.
func Query(r *http.Request, name string) string {
	return strings.TrimSpace(QueryRaw(r, name))
}

// QueryDefault returns the value of the named query parameter.
// If the parameter is empty, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func QueryDefault(r *http.Request, name string, defaultValue string) string {
	if !r.URL.Query().Has(name) {
		return defaultValue
	}

	return Query(r, name)
}

// QuerySlice returns the value of the named query parameter.
// All slice values are trimmed of leading and trailing whitespace.
func QuerySlice(r *http.Request, name string) []string {
	values, ok := r.URL.Query()[name]
	if !ok {
		return nil
	}

	result := make([]string, len(values))
	for i, value := range values {
		result[i] = strings.TrimSpace(value)
	}
	return result
}

// QuerySliceDefault returns the value of the named query parameter.
// If the parameter is empty, it returns the default value.
// All slice values are trimmed of leading and trailing whitespace.
func QuerySliceDefault(r *http.Request, name string, defaultValue []string) []string {
	if !r.URL.Query().Has(name) {
		return defaultValue
	}

	return QuerySlice(r, name)
}

// FragmentRaw returns the value of the named fragment parameter.
func FragmentRaw(r *http.Request) string {
	return r.URL.Fragment
}

// Fragment returns the value of the named fragment parameter.
// The return value is trimmed of leading and trailing whitespace.
func Fragment(r *http.Request) string {
	return strings.TrimSpace(FragmentRaw(r))
}

// FragmentDefault returns the value of the named fragment parameter.
// If the parameter is empty, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func FragmentDefault(r *http.Request, defaultValue string) string {
	if r.URL.Fragment == "" {
		return defaultValue
	}

	return Fragment(r)
}

// FormRaw returns the value of the named form parameter.
func FormRaw(r *http.Request, name string) string {
	return r.FormValue(name)
}

// Form returns the value of the named form parameter.
// The return value is trimmed of leading and trailing whitespace.
func Form(r *http.Request, name string) string {
	return strings.TrimSpace(FormRaw(r, name))
}

// DefaultForm returns the value of the named form parameter.
// If the parameter is not set, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func DefaultForm(r *http.Request, name, defaultValue string) string {
	err := r.ParseForm()
	if err != nil {
		return defaultValue
	}

	if !r.Form.Has(name) {
		return defaultValue
	}

	return Form(r, name)
}

// HeaderRaw returns the value of the named header.
func HeaderRaw(r *http.Request, name string) string {
	return r.Header.Get(name)
}

// Header returns the value of the named header.
// The return value is trimmed of leading and trailing whitespace.
func Header(r *http.Request, name string) string {
	return strings.TrimSpace(HeaderRaw(r, name))
}

// HeaderDefault returns the value of the named header.
// If the header is not set, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func HeaderDefault(r *http.Request, name, defaultValue string) string {
	if _, ok := textproto.MIMEHeader(r.Header)[name]; !ok {
		return defaultValue
	}

	return Header(r, name)
}

// Cookie returns the value of the named cookie.
// The return value is trimmed of leading and trailing whitespace.
func Cookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}

// CookieDefault returns the value of the named cookie.
// If the cookie is not set, it returns the default value.
// The return value is trimmed of leading and trailing whitespace.
func CookieDefault(r *http.Request, name, defaultValue string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return defaultValue
	}

	return strings.TrimSpace(cookie.Value)
}

// ClientIp returns the client IP address.
//
// As the request may come from a proxy, the function checks the
// X-Real-Ip and X-Forwarded-For headers to get the real client IP
// if the request IP matches one of the allowed proxy IPs.
// If the special proxy value CheckPrivateProxy ("PRIVATE") is passed, the function will
// also check the header if the request IP is a private IP address.
func ClientIp(r *http.Request, allowedProxyIp ...string) string {
	ipStr, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	switch {
	case err != nil && strings.Contains(err.Error(), "missing port in address"):
		ipStr = strings.TrimSpace(r.RemoteAddr)
	case err != nil:
		ipStr = ""
	}
	IP := net.ParseIP(ipStr)
	if IP == nil {
		return ""
	}

	isProxiedRequest := false
	if len(allowedProxyIp) > 0 {
		if slices.Contains(allowedProxyIp, IP.String()) {
			isProxiedRequest = true
		}
		if IP.IsPrivate() && slices.Contains(allowedProxyIp, CheckPrivateProxy) {
			isProxiedRequest = true
		}
	}

	if isProxiedRequest {
		realClientIP := r.Header.Get("X-Real-Ip")
		if realClientIP == "" {
			realClientIP = r.Header.Get("X-Forwarded-For")
		}
		if realClientIP != "" {
			realIpStr, _, err := net.SplitHostPort(strings.TrimSpace(realClientIP))
			switch {
			case err != nil && strings.Contains(err.Error(), "missing port in address"):
				realIpStr = realClientIP
			case err != nil:
				realIpStr = ipStr
			}
			realIP := net.ParseIP(realIpStr)
			if realIP == nil {
				return IP.String()
			}
			return realIP.String()
		}
	}

	return IP.String()
}

// BodyJson decodes the JSON value from the request body into the target.
// The target must be a pointer to a struct or slice.
// The function returns an error if the JSON value could not be decoded.
// The body reader is closed after reading.
func BodyJson(r *http.Request, target any) error {
	defer func() {
		_ = r.Body.Close()
	}()
	return json.NewDecoder(r.Body).Decode(target)
}

// BodyString returns the request body as a string.
// The content is read and returned as is, without any processing.
// The body is assumed to be UTF-8 encoded.
func BodyString(r *http.Request) (string, error) {
	defer func() {
		_ = r.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}
