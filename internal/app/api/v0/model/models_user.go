package model

import (
	"time"

	"github.com/fedor-git/wg-portal-2/internal/domain"
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
	Locked         bool   `json:"Locked"`         // if this field is set, the user is locked
	LockedReason   string `json:"LockedReason"`   // the reason why the user has been locked

	ApiToken        string     `json:"ApiToken"`
	ApiTokenCreated *time.Time `json:"ApiTokenCreated,omitempty"`
	ApiEnabled      bool       `json:"ApiEnabled"`

	// Calculated

	PeerCount int `json:"PeerCount"`
}

func NewUser(src *domain.User, exposeCreds bool) *User {
	u := &User{
		Identifier:      string(src.Identifier),
		Email:           src.Email,
		Source:          string(src.Source),
		ProviderName:    src.ProviderName,
		IsAdmin:         src.IsAdmin,
		Firstname:       src.Firstname,
		Lastname:        src.Lastname,
		Phone:           src.Phone,
		Department:      src.Department,
		Notes:           src.Notes,
		Password:        "", // never fill password
		Disabled:        src.IsDisabled(),
		DisabledReason:  src.DisabledReason,
		Locked:          src.IsLocked(),
		LockedReason:    src.LockedReason,
		ApiToken:        "", // by default, do not expose API token
		ApiTokenCreated: src.ApiTokenCreated,
		ApiEnabled:      src.IsApiEnabled(),

		PeerCount: src.LinkedPeerCount,
	}

	if exposeCreds {
		u.ApiToken = src.ApiToken
	}

	return u
}

func NewUsers(src []domain.User) []User {
	results := make([]User, len(src))
	for i := range src {
		results[i] = *NewUser(&src[i], false)
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
		Locked:          nil, // set below
		LockedReason:    src.LockedReason,
		LinkedPeerCount: src.PeerCount,
	}

	if src.Disabled {
		res.Disabled = &now
		if src.DisabledReason == "" {
			res.DisabledReason = domain.DisabledReasonAdmin
		}
	}

	if src.Locked {
		res.Locked = &now
		if src.LockedReason == "" {
			res.LockedReason = domain.LockedReasonAdmin
		}
	}

	return res
}
