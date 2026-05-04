package domain

import (
	"fmt"
	"strings"
)

type LoginProvider string

type LoginProviderInfo struct {
	Identifier  string
	Name        string
	ProviderUrl string
	CallbackUrl string
}

type AuthenticatorUserInfo struct {
	Identifier         UserIdentifier
	Email              string
	UserGroups         []string
	Firstname          string
	Lastname           string
	Phone              string
	Department         string
	IsAdmin            bool
	AdminInfoAvailable bool // true if the IsAdmin flag is valid
}

// Sanitize sanitizes all external identity provider fields in place.
// Returns ErrInvalidData if the identifier becomes empty after sanitization.
func (u *AuthenticatorUserInfo) Sanitize(providerType, providerName string) error {
	identifier := string(u.Identifier)
	LogSanitizeChange(providerType, providerName, "identifier", identifier,
		func() string { return SanitizeIdentifier(identifier, 256) }, &identifier)
	u.Identifier = UserIdentifier(identifier)

	email := u.Email
	LogSanitizeChange(providerType, providerName, "email", email,
		func() string { return SanitizeEmail(email, 254) }, &u.Email)
	LogSanitizeChange(providerType, providerName, "firstname", u.Firstname,
		func() string { return SanitizeString(u.Firstname, 128) }, &u.Firstname)
	LogSanitizeChange(providerType, providerName, "lastname", u.Lastname,
		func() string { return SanitizeString(u.Lastname, 128) }, &u.Lastname)
	LogSanitizeChange(providerType, providerName, "phone", u.Phone,
		func() string { return SanitizePhone(u.Phone, 50) }, &u.Phone)
	LogSanitizeChange(providerType, providerName, "department", u.Department,
		func() string { return SanitizeString(u.Department, 128) }, &u.Department)

	u.UserGroups = sanitizeGroups(providerType, providerName, u.UserGroups)

	if u.Identifier == "" {
		return fmt.Errorf("empty user identifier: %w", ErrInvalidData)
	}

	return nil
}

// sanitizeGroups sanitizes group names, dropping any that were modified by sanitization.
func sanitizeGroups(providerType, providerName string, rawGroups []string) []string {
	if len(rawGroups) == 0 {
		return rawGroups
	}

	groups := make([]string, 0, len(rawGroups))
	for _, rawGroup := range rawGroups {
		sanitized := rawGroup
		LogSanitizeChange(providerType, providerName, "user_group", rawGroup,
			func() string { return SanitizeString(rawGroup, 256) }, &sanitized)
		if sanitized == "" || sanitized != strings.TrimSpace(rawGroup) {
			continue
		}
		groups = append(groups, sanitized)
	}

	return groups
}
