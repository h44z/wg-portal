package auth

import (
	"strings"

	"github.com/fedor-git/wg-portal-2/internal"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// parseOauthUserInfo parses the raw user info from the oauth provider and maps it to the internal user info struct
func parseOauthUserInfo(
	mapping config.OauthFields,
	adminMapping *config.OauthAdminMapping,
	raw map[string]any,
) (*domain.AuthenticatorUserInfo, error) {
	var isAdmin bool

	// first try to match the is_admin field against the given regex
	if mapping.IsAdmin != "" {
		re := adminMapping.GetAdminValueRegex()
		if re.MatchString(strings.TrimSpace(internal.MapDefaultString(raw, mapping.IsAdmin, ""))) {
			isAdmin = true
		}
	}

	// next try to parse the user's groups
	if !isAdmin && mapping.UserGroups != "" && adminMapping.AdminGroupRegex != "" {
		userGroups := internal.MapDefaultStringSlice(raw, mapping.UserGroups, nil)
		re := adminMapping.GetAdminGroupRegex()
		for _, group := range userGroups {
			if re.MatchString(strings.TrimSpace(group)) {
				isAdmin = true
				break
			}
		}
	}

	userInfo := &domain.AuthenticatorUserInfo{
		Identifier: domain.UserIdentifier(internal.MapDefaultString(raw, mapping.UserIdentifier, "")),
		Email:      internal.MapDefaultString(raw, mapping.Email, ""),
		Firstname:  internal.MapDefaultString(raw, mapping.Firstname, ""),
		Lastname:   internal.MapDefaultString(raw, mapping.Lastname, ""),
		Phone:      internal.MapDefaultString(raw, mapping.Phone, ""),
		Department: internal.MapDefaultString(raw, mapping.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
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
