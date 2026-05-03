package users

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	sanitizeutil "github.com/h44z/wg-portal/internal/sanitize"
)

func convertRawLdapUser(
	providerName string,
	rawUser map[string]any,
	fields *config.LdapFields,
	adminGroupDN *ldap.DN,
	sanitizeUserData bool,
) (*domain.User, error) {
	now := time.Now()

	isAdmin, err := internal.LdapIsMemberOf(rawUser[fields.GroupMembership].([][]byte), adminGroupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin group: %w", err)
	}

	uid := internal.MapDefaultString(rawUser, fields.UserIdentifier, "")
	email := strings.ToLower(internal.MapDefaultString(rawUser, fields.Email, ""))
	firstname := internal.MapDefaultString(rawUser, fields.Firstname, "")
	lastname := internal.MapDefaultString(rawUser, fields.Lastname, "")
	phone := internal.MapDefaultString(rawUser, fields.Phone, "")
	department := internal.MapDefaultString(rawUser, fields.Department, "")

	if sanitizeUserData {
		sanitizeutil.LogChange("ldap", providerName, "identifier", uid,
			func() string { return domain.SanitizeIdentifier(uid, 256) }, &uid)
		sanitizeutil.LogChange("ldap", providerName, "email", email,
			func() string { return domain.SanitizeEmail(email, 254) }, &email)
		sanitizeutil.LogChange("ldap", providerName, "firstname", firstname,
			func() string { return domain.SanitizeString(firstname, 128) }, &firstname)
		sanitizeutil.LogChange("ldap", providerName, "lastname", lastname,
			func() string { return domain.SanitizeString(lastname, 128) }, &lastname)
		sanitizeutil.LogChange("ldap", providerName, "phone", phone,
			func() string { return domain.SanitizePhone(phone, 50) }, &phone)
		sanitizeutil.LogChange("ldap", providerName, "department", department,
			func() string { return domain.SanitizeString(department, 128) }, &department)
	}

	if uid == "" {
		return nil, fmt.Errorf("empty user identifier: %w", domain.ErrInvalidData)
	}

	domainUid := domain.UserIdentifier(uid)

	return &domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: domain.CtxSystemLdapSyncer,
			UpdatedBy: domain.CtxSystemLdapSyncer,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier: domainUid,
		Email:      email,
		IsAdmin:    isAdmin,
		Authentications: []domain.UserAuthentication{
			{
				UserIdentifier: domainUid,
				Source:         domain.UserSourceLdap,
				ProviderName:   providerName,
			},
		},
		Firstname:  firstname,
		Lastname:   lastname,
		Phone:      phone,
		Department: department,
		Notes:      "",
		Password:   "",
		Disabled:   nil,
	}, nil
}

func userChangedInLdap(dbUser, ldapUser *domain.User) bool {
	if dbUser.Firstname != ldapUser.Firstname {
		return true
	}
	if dbUser.Lastname != ldapUser.Lastname {
		return true
	}
	if dbUser.Email != ldapUser.Email {
		return true
	}
	if dbUser.Phone != ldapUser.Phone {
		return true
	}
	if dbUser.Department != ldapUser.Department {
		return true
	}

	if dbUser.IsDisabled() != ldapUser.IsDisabled() {
		return true
	}

	if dbUser.IsAdmin != ldapUser.IsAdmin {
		return true
	}

	if !slices.ContainsFunc(dbUser.Authentications, func(authentication domain.UserAuthentication) bool {
		return authentication.Source == ldapUser.Authentications[0].Source
	}) {
		return true
	}

	return false
}
