package model

import (
	"slices"
	"strings"

	"github.com/h44z/wg-portal/internal/domain"
)

type LoginProviderInfo struct {
	Identifier  string `json:"Identifier" example:"google"`
	Name        string `json:"Name" example:"Login with Google"`
	ProviderUrl string `json:"ProviderUrl" example:"/auth/google/login"`
	CallbackUrl string `json:"CallbackUrl" example:"/auth/google/callback"`
}

func NewLoginProviderInfo(src *domain.LoginProviderInfo) *LoginProviderInfo {
	return &LoginProviderInfo{
		Identifier:  src.Identifier,
		Name:        src.Name,
		ProviderUrl: src.ProviderUrl,
		CallbackUrl: src.CallbackUrl,
	}
}

func NewLoginProviderInfos(src []domain.LoginProviderInfo) []LoginProviderInfo {
	accessories := make([]LoginProviderInfo, len(src))
	for i := range src {
		accessories[i] = *NewLoginProviderInfo(&src[i])
	}
	return accessories
}

type SessionInfo struct {
	LoggedIn       bool    `json:"LoggedIn"`
	IsAdmin        bool    `json:"IsAdmin,omitempty"`
	UserIdentifier *string `json:"UserIdentifier,omitempty"`
	UserFirstname  *string `json:"UserFirstname,omitempty"`
	UserLastname   *string `json:"UserLastname,omitempty"`
	UserEmail      *string `json:"UserEmail,omitempty"`
}

type OauthInitiationResponse struct {
	RedirectUrl string
	State       string
}

type WebAuthnCredentialRequest struct {
	Name string `json:"Name"`
}
type WebAuthnCredentialResponse struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	CreatedAt string `json:"CreatedAt"`
}

func NewWebAuthnCredentialResponse(src domain.UserWebauthnCredential) WebAuthnCredentialResponse {
	return WebAuthnCredentialResponse{
		ID:        src.CredentialIdentifier,
		Name:      src.DisplayName,
		CreatedAt: src.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func NewWebAuthnCredentialResponses(src []domain.UserWebauthnCredential) []WebAuthnCredentialResponse {
	credentials := make([]WebAuthnCredentialResponse, len(src))
	for i := range src {
		credentials[i] = NewWebAuthnCredentialResponse(src[i])
	}
	// Sort by CreatedAt, newest first
	slices.SortFunc(credentials, func(i, j WebAuthnCredentialResponse) int {
		return strings.Compare(i.CreatedAt, j.CreatedAt)
	})
	return credentials
}
