package oauthproviders

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal/oauth/userprofile"
)

type ProviderType string

type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CreateUsers  bool
}

type Provider interface {
	AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	Exchange(context.Context, string, ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	TokenSource(context.Context, *oauth2.Token) oauth2.TokenSource

	ID() string
	CanCreateUsers() bool
	UserInfo(ctx context.Context, ts oauth2.TokenSource) (userprofile.Profile, error)
}

func DoRequest(ctx context.Context, ts oauth2.TokenSource, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("oauth: cannot create GET request: %v", err)
	}

	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("oauth: cannot get access token: %v", err)
	}

	token.SetAuthHeader(req)

	client := &http.Client{Timeout: 5 * time.Second}

	return client.Do(req)
}
