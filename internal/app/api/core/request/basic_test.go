package request

import (
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	r := &http.Request{URL: &url.URL{Path: "/test/sample"}}
	r.SetPathValue("first", "test")
	if got := Path(r, "first"); got != "test" {
		t.Errorf("Path() = %v, want %v", got, "test")
	}
}

func TestDefaultPath(t *testing.T) {
	r := &http.Request{URL: &url.URL{Path: "/"}}
	if got := PathDefault(r, "test", "default"); got != "default" {
		t.Errorf("PathDefault() = %v, want %v", got, "default")
	}
}

func TestDefaultPath_noDefault(t *testing.T) {
	r := &http.Request{URL: &url.URL{Path: "/"}}
	r.SetPathValue("first", "test")
	if got := PathDefault(r, "first", "test"); got != "test" {
		t.Errorf("PathDefault() = %v, want %v", got, "test")
	}
}

func TestQuery(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "name=value"}}
	if got := Query(r, "name"); got != "value" {
		t.Errorf("Query() = %v, want %v", got, "value")
	}
}

func TestDefaultQuery(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: ""}}
	if got := QueryDefault(r, "name", "default"); got != "default" {
		t.Errorf("QueryDefault() = %v, want %v", got, "default")
	}
}

func TestQuerySlice(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "name=value1  &name=value2"}}
	expected := []string{"value1", "value2"}
	if got := QuerySlice(r, "name"); !slices.Equal(got, expected) {
		t.Errorf("QuerySlice() = %v, want %v", got, expected)
	}
}

func TestQuerySlice_empty(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "name=value1&name=value2"}}
	if got := QuerySlice(r, "nix"); !slices.Equal(got, nil) {
		t.Errorf("QuerySlice() = %v, want %v", got, nil)
	}
}

func TestDefaultQuerySlice(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: ""}}
	defaultValue := []string{"default1", "default2"}
	if got := QuerySliceDefault(r, "name", defaultValue); !slices.Equal(got, defaultValue) {
		t.Errorf("QuerySliceDefault() = %v, want %v", got, defaultValue)
	}
}

func TestFragment(t *testing.T) {
	r := &http.Request{URL: &url.URL{Fragment: "section"}}
	if got := Fragment(r); got != "section" {
		t.Errorf("Fragment() = %v, want %v", got, "section")
	}
}

func TestDefaultFragment(t *testing.T) {
	r := &http.Request{URL: &url.URL{Fragment: ""}}
	if got := FragmentDefault(r, "default"); got != "default" {
		t.Errorf("FragmentDefault() = %v, want %v", got, "default")
	}
}

func TestForm(t *testing.T) {
	r := &http.Request{Form: url.Values{"name": {"value"}}}
	if got := Form(r, "name"); got != "value" {
		t.Errorf("Form() = %v, want %v", got, "value")
	}
}

func TestDefaultForm(t *testing.T) {
	r := &http.Request{Form: url.Values{}}
	if got := DefaultForm(r, "name", "default"); got != "default" {
		t.Errorf("DefaultForm() = %v, want %v", got, "default")
	}
}

func TestHeader(t *testing.T) {
	r := &http.Request{Header: http.Header{"X-Test-Header": {"value"}}}
	if got := Header(r, "X-Test-Header"); got != "value" {
		t.Errorf("Header() = %v, want %v", got, "value")
	}
}

func TestDefaultHeader(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if got := HeaderDefault(r, "X-Test-Header", "default"); got != "default" {
		t.Errorf("HeaderDefault() = %v, want %v", got, "default")
	}
}

func TestCookie(t *testing.T) {
	r := &http.Request{Header: http.Header{"Cookie": {"name=value"}}}
	if got := Cookie(r, "name"); got != "value" {
		t.Errorf("Cookie() = %v, want %v", got, "value")
	}
}

func TestDefaultCookie(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if got := CookieDefault(r, "name", "default"); got != "default" {
		t.Errorf("CookieDefault() = %v, want %v", got, "default")
	}
}

func TestClientIp(t *testing.T) {
	r := &http.Request{RemoteAddr: "192.168.1.1:12345"}
	if got := ClientIp(r); got != "192.168.1.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "192.168.1.1")
	}
}

func TestClientIp_invalid(t *testing.T) {
	r := &http.Request{RemoteAddr: "was_isn_des"}
	if got := ClientIp(r); got != "" {
		t.Errorf("ClientIp() = %v, want %v", got, "")
	}
}

func TestClientIp_ignore_header(t *testing.T) {
	r := &http.Request{RemoteAddr: "192.168.1.1:12345", Header: http.Header{"X-Forwarded-For": {"123.45.67.1"}}}
	if got := ClientIp(r); got != "192.168.1.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "192.168.1.1")
	}
}

func TestClientIp_header1(t *testing.T) {
	r := &http.Request{RemoteAddr: "192.168.1.1:12345", Header: http.Header{"X-Forwarded-For": {"123.45.67.1"}}}
	if got := ClientIp(r, CheckPrivateProxy); got != "123.45.67.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "123.45.67.1")
	}
}

func TestClientIp_header2(t *testing.T) {
	r := &http.Request{RemoteAddr: "192.168.1.1:12345", Header: http.Header{"X-Real-Ip": {"123.45.67.1"}}}
	if got := ClientIp(r, CheckPrivateProxy); got != "123.45.67.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "123.45.67.1")
	}
}

func TestClientIp_header3(t *testing.T) {
	r := &http.Request{RemoteAddr: "1.1.1.1:12345", Header: http.Header{"X-Real-Ip": {"123.45.67.1"}}}
	if got := ClientIp(r, "1.1.1.1"); got != "123.45.67.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "123.45.67.1")
	}
}

func TestClientIp_header4(t *testing.T) {
	r := &http.Request{RemoteAddr: "8.8.8.8:12345", Header: http.Header{"X-Real-Ip": {"123.45.67.1"}}}
	if got := ClientIp(r, "1.1.1.1"); got != "8.8.8.8" {
		t.Errorf("ClientIp() = %v, want %v", got, "8.8.8.8")
	}
}

func TestClientIp_header_invalid(t *testing.T) {
	r := &http.Request{RemoteAddr: "1.1.1.1:12345", Header: http.Header{"X-Real-Ip": {"so-sicher-nit"}}}
	if got := ClientIp(r, "1.1.1.1"); got != "1.1.1.1" {
		t.Errorf("ClientIp() = %v, want %v", got, "1.1.1.1")
	}
}

func TestBodyJson(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonStr := `{"name": "test", "value": 123}`
	r := &http.Request{
		Body: io.NopCloser(strings.NewReader(jsonStr)),
	}

	var result TestStruct
	err := BodyJson(r, &result)
	if err != nil {
		t.Fatalf("BodyJson() error = %v", err)
	}

	expected := TestStruct{Name: "test", Value: 123}
	if result != expected {
		t.Errorf("BodyJson() = %v, want %v", result, expected)
	}
}

func TestBodyString(t *testing.T) {
	bodyStr := "test body content"
	r := &http.Request{
		Body: io.NopCloser(strings.NewReader(bodyStr)),
	}

	result, err := BodyString(r)
	if err != nil {
		t.Fatalf("BodyString() error = %v", err)
	}

	if result != bodyStr {
		t.Errorf("BodyString() = %v, want %v", result, bodyStr)
	}
}
