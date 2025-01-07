package models

import (
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

	Disabled       bool   `json:"Disabled"`       // If this field is set, the user is disabled.
	DisabledReason string `json:"DisabledReason"` // The reason why the user has been disabled.
	Locked         bool   `json:"Locked"`         // If this field is set, the user is locked and thus unable to log in to WireGuard Portal.
	LockedReason   string `json:"LockedReason"`   // The reason why the user has been locked.

	ApiEnabled bool `json:"ApiEnabled"` // If this field is set, the user is allowed to use the RESTful API.

	PeerCount int `json:"PeerCount"` // The number of peers linked to the user.
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
		Disabled:       src.IsDisabled(),
		DisabledReason: src.DisabledReason,
		Locked:         src.IsLocked(),
		LockedReason:   src.LockedReason,
		ApiEnabled:     src.IsApiEnabled(),

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
