package models

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// User represents a user in the system.
type User struct {
	Identifier   string `json:"Identifier"`   // The unique identifier of the user.
	Email        string `json:"Email"`        // The email address of the user. This field is optional.
	Source       string `json:"Source"`       // The source of the user. This field is optional.
	ProviderName string `json:"ProviderName"` // The name of the authentication provider. This field is optional.
	IsAdmin      bool   `json:"IsAdmin"`      // If this field is set, the user is an admin.

	Firstname  string `json:"Firstname"`  // The first name of the user. This field is optional.
	Lastname   string `json:"Lastname"`   // The last name of the user. This field is optional.
	Phone      string `json:"Phone"`      // The phone number of the user. This field is optional.
	Department string `json:"Department"` // The department of the user. This field is optional.
	Notes      string `json:"Notes"`      // Additional notes about the user. This field is optional.

	Password       string `json:"Password,omitempty"` // The password of the user. This field is never populated on read operations.
	Disabled       bool   `json:"Disabled"`           // If this field is set, the user is disabled.
	DisabledReason string `json:"DisabledReason"`     // The reason why the user has been disabled.
	Locked         bool   `json:"Locked"`             // If this field is set, the user is locked and thus unable to log in to WireGuard Portal.
	LockedReason   string `json:"LockedReason"`       // The reason why the user has been locked.

	ApiToken   string `json:"ApiToken"`   // The API token of the user. This field is never populated on bulk read operations.
	ApiEnabled bool   `json:"ApiEnabled"` // If this field is set, the user is allowed to use the RESTful API. This field is read-only.

	PeerCount int `json:"PeerCount"` // The number of peers linked to the user. This field is read-only.
}

func NewUser(src *domain.User, exposeCredentials bool) *User {
	u := &User{
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
		Locked:         src.IsLocked(),
		LockedReason:   src.LockedReason,
		ApiToken:       "", // by default, do not expose API token
		ApiEnabled:     src.IsApiEnabled(),
		PeerCount:      src.LinkedPeerCount,
	}

	if exposeCredentials {
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
		Identifier:     domain.UserIdentifier(src.Identifier),
		Email:          src.Email,
		Source:         domain.UserSource(src.Source),
		ProviderName:   src.ProviderName,
		IsAdmin:        src.IsAdmin,
		Firstname:      src.Firstname,
		Lastname:       src.Lastname,
		Phone:          src.Phone,
		Department:     src.Department,
		Notes:          src.Notes,
		Password:       domain.PrivateString(src.Password),
		Disabled:       nil, // set below
		DisabledReason: src.DisabledReason,
		Locked:         nil, // set below
		LockedReason:   src.LockedReason,
	}

	if src.ApiToken != "" {
		res.ApiToken = src.ApiToken
		res.ApiTokenCreated = &now
	}

	if src.Disabled {
		res.Disabled = &now
	}

	if src.Locked {
		res.Locked = &now
	}

	return res
}
