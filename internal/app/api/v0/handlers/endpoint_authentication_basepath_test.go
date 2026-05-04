package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/h44z/wg-portal/internal/config"
)

type testSession struct {
	data SessionData
}

func (s *testSession) SetData(_ context.Context, val SessionData) {
	s.data = val
}

func (s *testSession) GetData(_ context.Context) SessionData {
	return s.data
}

func (s *testSession) DestroyData(_ context.Context) {
	s.data = SessionData{}
}

func newBasePathAuthEndpoint(session Session) AuthEndpoint {
	return AuthEndpoint{
		cfg: &config.Config{
			Web: config.WebConfig{
				ExternalUrl: "https://wg.example.com",
				BasePath:    "/subpath",
			},
		},
		session: session,
	}
}

func TestAuthEndpointIsValidReturnUrlRequiresBasePathApp(t *testing.T) {
	ep := newBasePathAuthEndpoint(&testSession{})

	valid := []string{
		"https://wg.example.com/subpath/app/#/login",
		"https://wg.example.com/subpath/app/#/login?all=true",
		"https://wg.example.com/subpath/app/?beforeHash=true#/login",
	}
	for _, returnURL := range valid {
		if !ep.isValidReturnUrl(returnURL) {
			t.Fatalf("expected return URL to be valid: %s", returnURL)
		}
	}

	invalid := []string{
		"https://wg.example.com/#/login",
		"https://wg.example.com/subpath/#/login",
		"https://other.example.com/subpath/app/#/login",
	}
	for _, returnURL := range invalid {
		if ep.isValidReturnUrl(returnURL) {
			t.Fatalf("expected return URL to be invalid: %s", returnURL)
		}
	}
}

func TestAuthEndpointOauthCallbackRedirectsToBasePathHashRoute(t *testing.T) {
	session := &testSession{data: SessionData{
		LoggedIn:      true,
		OauthReturnTo: "https://wg.example.com/subpath/app/#/login",
	}}
	ep := newBasePathAuthEndpoint(session)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/login/google/callback", nil)
	req.SetPathValue("provider", "google")
	res := httptest.NewRecorder()

	ep.handleOauthCallbackGet().ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, res.Code)
	}
	if got, want := res.Header().Get("Location"), "https://wg.example.com/subpath/app/#/login?wgLoginState=success"; got != want {
		t.Fatalf("expected redirect %q, got %q", want, got)
	}
}

func TestAuthEndpointReturnUrlWithLoginStatePreservesHashQuery(t *testing.T) {
	session := &testSession{data: SessionData{
		LoggedIn:      true,
		OauthReturnTo: "https://wg.example.com/subpath/app/#/login?all=true",
	}}
	ep := newBasePathAuthEndpoint(session)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/login/google/callback", nil)
	req.SetPathValue("provider", "google")
	res := httptest.NewRecorder()

	ep.handleOauthCallbackGet().ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, res.Code)
	}
	if got, want := res.Header().Get("Location"), "https://wg.example.com/subpath/app/#/login?all=true&wgLoginState=success"; got != want {
		t.Fatalf("expected redirect %q, got %q", want, got)
	}
}

func TestAuthEndpointFrontendUrlUsesBasePathAppMount(t *testing.T) {
	ep := newBasePathAuthEndpoint(&testSession{})

	if got, want := ep.frontendUrl("/login"), "https://wg.example.com/subpath/app/#/login"; got != want {
		t.Fatalf("expected frontend URL %q, got %q", want, got)
	}
}
