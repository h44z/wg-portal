package google

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/userprofile"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	googleApiUserProfile = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
)

type userInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture,omitempty"`
	Locale        string `json:"locale,omitempty"`
}

type provider struct {
	oauth2.Config
	createUsers bool
}

func New(pc oauthproviders.ProviderConfig) oauthproviders.Provider {
	config := oauth2.Config{
		ClientID:     pc.ClientID,
		ClientSecret: pc.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.profile", "https://www.googleapis.com/auth/userinfo.email"},
		RedirectURL:  pc.RedirectURL,
	}

	return &provider{
		Config:      config,
		createUsers: pc.CreateUsers,
	}
}

func (g provider) UserInfo(ctx context.Context, ts oauth2.TokenSource) (userprofile.Profile, error) {
	resp, err := oauthproviders.DoRequest(ctx, ts, googleApiUserProfile)
	if err != nil {
		return userprofile.Profile{}, err
	}
	defer resp.Body.Close()

	var p userInfo
	if err = json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return userprofile.Profile{}, fmt.Errorf("google: invalid response from the authentication sever: %v", err)
	}

	if p.Email == "" || !p.VerifiedEmail {
		return userprofile.Profile{}, fmt.Errorf("google: no valid email found for the user '%s' (%s)", p.Name, p.ID)
	}

	if p.GivenName == "" && p.FamilyName == "" {
		p.GivenName = p.Email
	}

	return userprofile.Profile{
		FirstName: p.GivenName,
		LastName:  p.FamilyName,
		Email:     p.Email,
	}, nil
}

func (g provider) CanCreateUsers() bool {
	return g.createUsers
}
