package model

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

type User struct {
	Identifier   string `json:"Identifier"`
	Email        string `json:"Email"`
	Source       string `json:"Source"`
	ProviderName string `json:"ProviderName"`
	IsAdmin      bool   `json:"IsAdmin"`

	Firstname  string `json:"Firstname"`
	Lastname   string `json:"Lastname"`
	Phone      string `json:"Phone"`
	Department string `json:"Department"`
	Notes      string `json:"Notes"`

	Password       string `json:"Password,omitempty"`
	Disabled       bool   `json:"Disabled"`       // if this field is set, the user is disabled
	DisabledReason string `json:"DisabledReason"` // the reason why the user has been disabled

	// Calculated

	PeerCount int `json:"PeerCount"`
}

func NewUser(src *domain.User) *User {
	return &User{
		Identifier:     string(src.Identifier),
		Email:          src.Email,
		Source:         string(src.Source),
		ProviderName:   src.ProviderName,
		IsAdmin:        src.IsAdmin,
		Firstname:      src.Firstname,
		Lastname:       src.Lastname,
		Phone:          src.Phone,
		Department:     src.Department,
		Notes:          src.Notes,
		Password:       "", // never fill password
		Disabled:       src.IsDisabled(),
		DisabledReason: src.DisabledReason,

		PeerCount: src.LinkedPeerCount,
	}
}

func NewUsers(src []domain.User) []User {
	results := make([]User, len(src))
	for i := range src {
		results[i] = *NewUser(&src[i])
	}

	return results
}

func NewDomainUser(src *User) *domain.User {
	now := time.Now()
	res := &domain.User{
		Identifier:      domain.UserIdentifier(src.Identifier),
		Email:           src.Email,
		Source:          domain.UserSource(src.Source),
		ProviderName:    src.ProviderName,
		IsAdmin:         src.IsAdmin,
		Firstname:       src.Firstname,
		Lastname:        src.Lastname,
		Phone:           src.Phone,
		Department:      src.Department,
		Notes:           src.Notes,
		Password:        domain.PrivateString(src.Password),
		Disabled:        nil, // set below
		DisabledReason:  src.DisabledReason,
		LinkedPeerCount: src.PeerCount,
	}

	if src.Disabled {
		res.Disabled = &now
	}

	return res
}
