package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/userprofile"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/bitbucket"
)

const ProviderBitbucket oauthproviders.ProviderType = "bitbucket"

const (
	bitbucketApiUserProfile = "https://api.bitbucket.org/2.0/user"
	bitbucketApiEmails      = "https://api.bitbucket.org/2.0/user/emails"
)

type userInfo struct {
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	AccountStatus string `json:"account_status"`
	DisplayName   string `json:"display_name"`
	Website       string `json:"website"`
	CreatedOn     string `json:"created_on"`
	UUID          string `json:"uuid"`
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
		Endpoint:     bitbucket.Endpoint,
		Scopes:       []string{"account", "email"},
		RedirectURL:  pc.RedirectURL,
	}

	return &provider{
		Config:      config,
		id:          string(ProviderBitbucket),
		createUsers: pc.CreateUsers,
	}
}

func (p provider) ID() string {
	return p.id
}

func (p provider) UserInfo(ctx context.Context, ts oauth2.TokenSource) (userprofile.Profile, error) {
	respProfile, err := oauthproviders.DoRequest(ctx, ts, bitbucketApiUserProfile)
	if err != nil {
		return userprofile.Profile{}, errors.WithMessage(err, "bitbucket: cannot create request to profile endpoint")
	}
	defer respProfile.Body.Close()

	if respProfile.StatusCode != http.StatusOK {
		return userprofile.Profile{}, errors.New(fmt.Sprintf("bitbucket: profile endpoint returned status code %s", respProfile.Status))
	}

	var u userInfo
	if err = json.NewDecoder(respProfile.Body).Decode(&u); err != nil {
		return userprofile.Profile{}, errors.WithMessage(err, "bitbucket: invalid response from the authentication sever profile endpoint")
	}

	if u.AccountStatus != "active" {
		return userprofile.Profile{}, errors.WithMessage(err, "bitbucket: the account is not active")
	}

	email, err := p.userEmail(ctx, ts)
	if err != nil {
		return userprofile.Profile{}, errors.WithMessagef(err, "bitbucket: user %s", u.Username)
	}

	if u.DisplayName == "" {
		u.DisplayName = email
	}

	return userprofile.Profile{
		FirstName: u.DisplayName,
		Email:     email,
	}, nil
}

type userEmails struct {
	Values []struct {
		IsPrimary   bool   `json:"is_primary"`
		IsConfirmed bool   `json:"is_confirmed"`
		Type        string `json:"type"`
		Email       string `json:"email"`
	} `json:"values"`
}

func (e userEmails) getPrimary() (string, error) {
	for _, email := range e.Values {
		if email.IsPrimary && email.IsConfirmed && email.Type == "email" {
			return email.Email, nil
		}
	}

	return "", errors.New("no valid email was found for the account")
}

func (p provider) userEmail(ctx context.Context, ts oauth2.TokenSource) (string, error) {
	respEmails, err := oauthproviders.DoRequest(ctx, ts, bitbucketApiEmails)
	if err != nil {
		return "", errors.WithMessage(err, "bitbucket: cannot create request to emails endpoint")
	}
	defer respEmails.Body.Close()

	if respEmails.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("bitbucket: profile endpoint returned status code %s", respEmails.Status))
	}

	var e userEmails
	if err = json.NewDecoder(respEmails.Body).Decode(&e); err != nil {
		return "", errors.WithMessage(err, "bitbucket: invalid response from the authentication sever emails endpoint")
	}

	return e.getPrimary()
}

func (p provider) CanCreateUsers() bool {
	return p.createUsers
}
