package users

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func convertRawLdapUser(
	providerName string,
	rawUser map[string]any,
	fields *config.LdapFields,
	adminGroupDN *ldap.DN,
) (*domain.User, error) {
	now := time.Now()

	isAdmin, err := internal.LdapIsMemberOf(rawUser[fields.GroupMembership].([][]byte), adminGroupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin group: %w", err)
	}

	return &domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: domain.CtxSystemLdapSyncer,
			UpdatedBy: domain.CtxSystemLdapSyncer,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier:   domain.UserIdentifier(internal.MapDefaultString(rawUser, fields.UserIdentifier, "")),
		Email:        strings.ToLower(internal.MapDefaultString(rawUser, fields.Email, "")),
		Source:       domain.UserSourceLdap,
		ProviderName: providerName,
		IsAdmin:      isAdmin,
		Firstname:    internal.MapDefaultString(rawUser, fields.Firstname, ""),
		Lastname:     internal.MapDefaultString(rawUser, fields.Lastname, ""),
		Phone:        internal.MapDefaultString(rawUser, fields.Phone, ""),
		Department:   internal.MapDefaultString(rawUser, fields.Department, ""),
		Notes:        "",
		Password:     "",
		Disabled:     nil,
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

	if dbUser.ProviderName != ldapUser.ProviderName {
		return true
	}

	return false
}
