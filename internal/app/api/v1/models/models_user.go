package models

import (
	"time"

	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// User represents a user in the system.
type User struct {
	// The unique identifier of the user.
	Identifier string `json:"Identifier" binding:"required,max=64" example:"uid-1234567"`
	// The email address of the user. This field is optional.
	Email string `json:"Email" binding:"omitempty,email" example:"test@test.com"`
	// The source of the user. This field is optional.
	Source string `json:"Source" binding:"oneof=db" example:"db"`
	// The name of the authentication provider. This field is read-only.
	ProviderName string `json:"ProviderName,omitempty" readonly:"true" example:""`
	// If this field is set, the user is an admin.
	IsAdmin bool `json:"IsAdmin" example:"false"`

	// The first name of the user. This field is optional.
	Firstname string `json:"Firstname" example:"Max"`
	// The last name of the user. This field is optional.
	Lastname string `json:"Lastname" example:"Muster"`
	// The phone number of the user. This field is optional.
	Phone string `json:"Phone" example:"+1234546789"`
	// The department of the user. This field is optional.
	Department string `json:"Department" example:"Software Development"`
	// Additional notes about the user. This field is optional.
	Notes string `json:"Notes" example:"some sample notes"`

	// The password of the user. This field is never populated on read operations.
	Password string `json:"Password,omitempty" binding:"omitempty,min=16,max=64" example:""`
	// If this field is set, the user is disabled.
	Disabled bool `json:"Disabled" example:"false"`
	// The reason why the user has been disabled.
	DisabledReason string `json:"DisabledReason" binding:"required_if=Disabled true" example:""`
	// If this field is set, the user is locked and thus unable to log in to WireGuard Portal.
	Locked bool `json:"Locked" example:"false"`
	// The reason why the user has been locked.
	LockedReason string `json:"LockedReason" binding:"required_if=Locked true" example:""`

	// The API token of the user. This field is never populated on bulk read operations.
	ApiToken string `json:"ApiToken,omitempty" binding:"omitempty,min=32,max=64" example:""`
	// If this field is set, the user is allowed to use the RESTful API. This field is read-only.
	ApiEnabled bool `json:"ApiEnabled" readonly:"true" example:"false"`

	// The number of peers linked to the user. This field is read-only.
	PeerCount int `json:"PeerCount" readonly:"true" example:"2"`
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
		if src.DisabledReason == "" {
			res.DisabledReason = domain.DisabledReasonApi
		}
	}

	if src.Locked {
		res.Locked = &now
		if src.LockedReason == "" {
			res.LockedReason = domain.LockedReasonApi
		}
	}

	return res
}
