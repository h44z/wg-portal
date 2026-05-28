package auth

import (
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func authCodeValues(t *testing.T, options []oauth2.AuthCodeOption) url.Values {
	t.Helper()

	config := oauth2.Config{
		ClientID:    "client-id",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://example.com/auth"},
		RedirectURL: "https://wg.example.com/callback",
	}
	authCodeURL, err := url.Parse(config.AuthCodeURL("state", options...))
	if err != nil {
		t.Fatalf("failed to parse auth code URL: %v", err)
	}

	return authCodeURL.Query()
}

func TestOidcAuthenticatorPKCES256Options(t *testing.T) {
	authenticator := OidcAuthenticator{usePKCE: true, pkceMethod: "S256"}

	options, verifier := authenticator.PKCEAuthCodeOptions()
	if verifier == "" {
		t.Fatal("expected verifier")
	}

	values := authCodeValues(t, options)

	if values.Get("code_challenge") == "" {
		t.Fatal("expected code_challenge")
	}
	if values.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256 challenge method, got %q", values.Get("code_challenge_method"))
	}

	tokenOptions := authenticator.PKCETokenOptions(verifier)
	if len(tokenOptions) != 1 {
		t.Fatalf("expected one token option, got %d", len(tokenOptions))
	}

}

func TestOidcAuthenticatorPKCEPlainOptions(t *testing.T) {
	authenticator := OidcAuthenticator{usePKCE: true, pkceMethod: "plain"}

	options, verifier := authenticator.PKCEAuthCodeOptions()
	values := authCodeValues(t, options)

	if values.Get("code_challenge") != verifier {
		t.Fatalf("expected plain challenge %q, got %q", verifier, values.Get("code_challenge"))
	}
	if values.Get("code_challenge_method") != "plain" {
		t.Fatalf("expected plain challenge method, got %q", values.Get("code_challenge_method"))
	}
}

func TestOidcAuthenticatorPKCEDisabled(t *testing.T) {
	authenticator := OidcAuthenticator{usePKCE: false, pkceMethod: "S256"}

	options, verifier := authenticator.PKCEAuthCodeOptions()
	if len(options) != 0 {
		t.Fatalf("expected no auth code options, got %d", len(options))
	}
	if verifier != "" {
		t.Fatalf("expected empty verifier, got %q", verifier)
	}

	tokenOptions := authenticator.PKCETokenOptions(oauth2.GenerateVerifier())
	if len(tokenOptions) != 0 {
		t.Fatalf("expected no token options, got %d", len(tokenOptions))
	}
}
