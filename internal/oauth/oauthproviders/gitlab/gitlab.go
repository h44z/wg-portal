package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/userprofile"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/gitlab"
)

const ProviderGitlab oauthproviders.ProviderType = "gitlab"

const (
	gitlabApiUserProfile = "https://gitlab.com/api/v4/user"
)

type userInfo struct {
	ID              int       `json:"id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	State           string    `json:"state"`
	AvatarURL       string    `json:"avatar_url"`
	WebURL          string    `json:"web_url"`
	CreatedAt       time.Time `json:"created_at"`
	Bio             string    `json:"bio"`
	PublicEmail     string    `json:"public_email"`
	Skype           string    `json:"skype"`
	Linkedin        string    `json:"linkedin"`
	Twitter         string    `json:"twitter"`
	WebsiteUrRL     string    `json:"website_url"`
	Organization    string    `json:"organization"`
	LastSignInAt    time.Time `json:"last_sign_in_at"`
	ConfirmedAt     time.Time `json:"confirmed_at"`
	ThemeID         int       `json:"theme_id"`
	LastActivityOn  string    `json:"last_activity_on"`
	ColorSchemeId   int       `json:"color_scheme_id"`
	ProjectsLimit   int       `json:"projects_limit"`
	CurrentSignInAt time.Time `json:"current_sign_in_at"`
	Identities      []struct {
		Provider  string `json:"provider"`
		ExternUid string `json:"extern_uid"`
	} `json:"identities"`
	CanCreateGroup   bool `json:"can_create_group"`
	CanCreateProject bool `json:"can_create_project"`
	TwoFactorEnabled bool `json:"two_factor_enabled"`
	External         bool `json:"external"`
	PrivateProfile   bool `json:"private_profile"`
}

type provider struct {
	id string
	oauth2.Config
	createUsers bool
}

func New(pc oauthproviders.ProviderConfig) oauthproviders.Provider {
	config := oauth2.Config{
		ClientID:     pc.ClientID,
		ClientSecret: pc.ClientSecret,
		Endpoint:     gitlab.Endpoint,
		Scopes:       []string{"read_user"},
		RedirectURL:  pc.RedirectURL,
	}

	return &provider{
		Config:      config,
		id:          string(ProviderGitlab),
		createUsers: pc.CreateUsers,
	}
}

func (g provider) ID() string {
	return g.id
}

func (g provider) UserInfo(ctx context.Context, ts oauth2.TokenSource) (userprofile.Profile, error) {
	resp, err := oauthproviders.DoRequest(ctx, ts, gitlabApiUserProfile)
	if err != nil {
		return userprofile.Profile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return userprofile.Profile{}, errors.New(fmt.Sprintf("gitlab: returned status code %s", resp.Status))
	}

	var p userInfo
	if err = json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return userprofile.Profile{}, errors.WithMessage(err, "gitlab: invalid response from the authentication sever")
	}

	email := p.PublicEmail
	if email == "" {
		email = p.Email
	}

	if p.Name == "" {
		p.Name = email
	}

	return userprofile.Profile{
		FirstName: p.Name,
		Email:     email,
	}, nil
}

func (g provider) CanCreateUsers() bool {
	return g.createUsers
}
