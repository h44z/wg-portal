package auth

import (
	"testing"

	"golang.org/x/oauth2"
)

func TestPlainOauthAuthenticatorPKCES256Options(t *testing.T) {
	authenticator := PlainOauthAuthenticator{usePKCE: true, pkceMethod: "S256"}

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

func TestPlainOauthAuthenticatorPKCEPlainOptions(t *testing.T) {
	authenticator := PlainOauthAuthenticator{usePKCE: true, pkceMethod: "plain"}

	options, verifier := authenticator.PKCEAuthCodeOptions()
	values := authCodeValues(t, options)

	if values.Get("code_challenge") != verifier {
		t.Fatalf("expected plain challenge %q, got %q", verifier, values.Get("code_challenge"))
	}
	if values.Get("code_challenge_method") != "plain" {
		t.Fatalf("expected plain challenge method, got %q", values.Get("code_challenge_method"))
	}
}

func TestPlainOauthAuthenticatorPKCEDisabled(t *testing.T) {
	authenticator := PlainOauthAuthenticator{usePKCE: false, pkceMethod: "S256"}

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
