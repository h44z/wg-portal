package auth

import (
	"fmt"
	"strings"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/sanitize"
)

// parseOauthUserInfo parses the raw user info from the oauth provider and maps it to the internal user info struct
func parseOauthUserInfo(
	mapping config.OauthFields,
	adminMapping *config.OauthAdminMapping,
	raw map[string]any,
	sanitizeUserData bool,
	providerType string,
	providerName string,
) (*domain.AuthenticatorUserInfo, error) {
	var isAdmin bool
	var adminInfoAvailable bool
	userGroups := internal.MapDefaultStringSlice(raw, mapping.UserGroups, nil)
	if sanitizeUserData {
		userGroups = sanitizeOauthGroups(providerType, providerName, userGroups)
	}

	// first try to match the is_admin field against the given regex
	if mapping.IsAdmin != "" {
		adminInfoAvailable = true
		re := adminMapping.GetAdminValueRegex()
		if re.MatchString(strings.TrimSpace(internal.MapDefaultString(raw, mapping.IsAdmin, ""))) {
			isAdmin = true
		}
	}

	// next try to parse the user's groups
	if !isAdmin && mapping.UserGroups != "" && adminMapping.AdminGroupRegex != "" {
		adminInfoAvailable = true
		re := adminMapping.GetAdminGroupRegex()
		for _, group := range userGroups {
			if re.MatchString(strings.TrimSpace(group)) {
				isAdmin = true
				break
			}
		}
	}

	identifier := internal.MapDefaultString(raw, mapping.UserIdentifier, "")
	email := internal.MapDefaultString(raw, mapping.Email, "")
	firstname := internal.MapDefaultString(raw, mapping.Firstname, "")
	lastname := internal.MapDefaultString(raw, mapping.Lastname, "")
	phone := internal.MapDefaultString(raw, mapping.Phone, "")
	department := internal.MapDefaultString(raw, mapping.Department, "")

	if sanitizeUserData {
		sanitize.LogChange(providerType, providerName, "identifier", identifier,
			func() string { return domain.SanitizeIdentifier(identifier, 256) }, &identifier)
		sanitize.LogChange(providerType, providerName, "email", email,
			func() string { return domain.SanitizeEmail(email, 254) }, &email)
		sanitize.LogChange(providerType, providerName, "firstname", firstname,
			func() string { return domain.SanitizeString(firstname, 128) }, &firstname)
		sanitize.LogChange(providerType, providerName, "lastname", lastname,
			func() string { return domain.SanitizeString(lastname, 128) }, &lastname)
		sanitize.LogChange(providerType, providerName, "phone", phone,
			func() string { return domain.SanitizePhone(phone, 50) }, &phone)
		sanitize.LogChange(providerType, providerName, "department", department,
			func() string { return domain.SanitizeString(department, 128) }, &department)
	}

	if identifier == "" {
		return nil, fmt.Errorf("empty user identifier: %w", domain.ErrInvalidData)
	}

	userInfo := &domain.AuthenticatorUserInfo{
		Identifier:         domain.UserIdentifier(identifier),
		Email:              email,
		UserGroups:         userGroups,
		Firstname:          firstname,
		Lastname:           lastname,
		Phone:              phone,
		Department:         department,
		IsAdmin:            isAdmin,
		AdminInfoAvailable: adminInfoAvailable,
	}

	return userInfo, nil
}

func sanitizeOauthGroups(providerType, providerName string, rawGroups []string) []string {
	if len(rawGroups) == 0 {
		return rawGroups
	}

	groups := make([]string, 0, len(rawGroups))
	for _, rawGroup := range rawGroups {
		sanitized := rawGroup
		sanitize.LogChange(providerType, providerName, "user_group", rawGroup,
			func() string { return domain.SanitizeString(rawGroup, 256) }, &sanitized)
		if sanitized == "" {
			continue
		}
		if sanitized != strings.TrimSpace(rawGroup) {
			continue
		}
		groups = append(groups, sanitized)
	}

	return groups
}

// getOauthFieldMapping returns the default field mapping for the oauth provider
func getOauthFieldMapping(f config.OauthFields) config.OauthFields {
	defaultMap := config.OauthFields{
		BaseFields: config.BaseFields{
			UserIdentifier: "sub",
			Email:          "email",
			Firstname:      "given_name",
			Lastname:       "family_name",
			Phone:          "phone",
			Department:     "department",
		},
		IsAdmin:    "admin_flag",
		UserGroups: "", // by default, do not use user groups
	}
	if f.UserIdentifier != "" {
		defaultMap.UserIdentifier = f.UserIdentifier
	}
	if f.Email != "" {
		defaultMap.Email = f.Email
	}
	if f.Firstname != "" {
		defaultMap.Firstname = f.Firstname
	}
	if f.Lastname != "" {
		defaultMap.Lastname = f.Lastname
	}
	if f.Phone != "" {
		defaultMap.Phone = f.Phone
	}
	if f.Department != "" {
		defaultMap.Department = f.Department
	}
	if f.IsAdmin != "" {
		defaultMap.IsAdmin = f.IsAdmin
	}
	if f.UserGroups != "" {
		defaultMap.UserGroups = f.UserGroups
	}

	return defaultMap
}
