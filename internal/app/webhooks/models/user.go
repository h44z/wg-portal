package models

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// User represents a user model for webhooks. For details about the fields, see the domain.User struct.
type User struct {
	CreatedBy string    `json:"CreatedBy"`
	UpdatedBy string    `json:"UpdatedBy"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`

	Identifier   string `json:"Identifier"`
	Email        string `json:"Email"`
	Source       string `json:"Source"`
	ProviderName string `json:"ProviderName"`
	IsAdmin      bool   `json:"IsAdmin"`

	Firstname  string `json:"Firstname,omitempty"`
	Lastname   string `json:"Lastname,omitempty"`
	Phone      string `json:"Phone,omitempty"`
	Department string `json:"Department,omitempty"`
	Notes      string `json:"Notes,omitempty"`

	Disabled       *time.Time `json:"Disabled,omitempty"`
	DisabledReason string     `json:"DisabledReason,omitempty"`
	Locked         *time.Time `json:"Locked,omitempty"`
	LockedReason   string     `json:"LockedReason,omitempty"`
}

// NewUser creates a new User model from a domain.User
func NewUser(src domain.User) User {
	return User{
		CreatedBy:      src.CreatedBy,
		UpdatedBy:      src.UpdatedBy,
		CreatedAt:      src.CreatedAt,
		UpdatedAt:      src.UpdatedAt,
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
		Disabled:       src.Disabled,
		DisabledReason: src.DisabledReason,
		Locked:         src.Locked,
		LockedReason:   src.LockedReason,
	}
}
