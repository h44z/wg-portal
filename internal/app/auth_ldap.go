package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type ldapAuthenticator struct {
	cfg *config.LdapProvider
}

func newLdapAuthenticator(_ context.Context, cfg *config.LdapProvider) (*ldapAuthenticator, error) {
	var provider = &ldapAuthenticator{}

	provider.cfg = cfg

	dn, err := ldap.ParseDN(cfg.AdminGroupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse admin group DN: %w", err)
	}
	provider.cfg.FieldMap = provider.getLdapFieldMapping(cfg.FieldMap)
	provider.cfg.ParsedAdminGroupDN = dn

	return provider, nil
}

func (l ldapAuthenticator) GetName() string {
	return l.cfg.ProviderName
}

func (l ldapAuthenticator) RegistrationEnabled() bool {
	return l.cfg.RegistrationEnabled
}

func (l ldapAuthenticator) PlaintextAuthentication(userId domain.UserIdentifier, plainPassword string) error {
	conn, err := ldapConnect(l.cfg)
	if err != nil {
		return fmt.Errorf("failed to setup connection: %w", err)
	}
	defer ldapDisconnect(conn)

	attrs := []string{"dn"}

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", string(userId), -1)
	searchRequest := ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 20, false, // 20 second time limit
		loginFilter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to search in ldap: %w", err)
	}

	if len(sr.Entries) == 0 {
		return domain.ErrNotFound
	}

	if len(sr.Entries) > 1 {
		return domain.ErrNotUnique
	}

	// Bind as the user to verify their password
	userDN := sr.Entries[0].DN
	err = conn.Bind(userDN, plainPassword)
	if err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}
	_ = conn.Unbind()

	return nil
}

func (l ldapAuthenticator) GetUserInfo(_ context.Context, userId domain.UserIdentifier) (map[string]interface{}, error) {
	conn, err := ldapConnect(l.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup connection: %w", err)
	}
	defer ldapDisconnect(conn)

	attrs := ldapSearchAttributes(&l.cfg.FieldMap)

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", string(userId), -1)
	searchRequest := ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 20, false, // 20 second time limit
		loginFilter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search in ldap: %w", err)
	}

	if len(sr.Entries) == 0 {
		return nil, domain.ErrNotFound
	}

	if len(sr.Entries) > 1 {
		return nil, domain.ErrNotUnique
	}

	users := ldapConvertEntries(sr, &l.cfg.FieldMap)

	return users[0], nil
}

func (l ldapAuthenticator) ParseUserInfo(raw map[string]interface{}) (*domain.AuthenticatorUserInfo, error) {
	isAdmin, err := ldapIsMemberOf(raw[l.cfg.FieldMap.GroupMembership].([][]byte), l.cfg.ParsedAdminGroupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin group: %w", err)
	}
	userInfo := &domain.AuthenticatorUserInfo{
		Identifier: domain.UserIdentifier(internal.MapDefaultString(raw, l.cfg.FieldMap.UserIdentifier, "")),
		Email:      internal.MapDefaultString(raw, l.cfg.FieldMap.Email, ""),
		Firstname:  internal.MapDefaultString(raw, l.cfg.FieldMap.Firstname, ""),
		Lastname:   internal.MapDefaultString(raw, l.cfg.FieldMap.Lastname, ""),
		Phone:      internal.MapDefaultString(raw, l.cfg.FieldMap.Phone, ""),
		Department: internal.MapDefaultString(raw, l.cfg.FieldMap.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}

func (l ldapAuthenticator) getLdapFieldMapping(f config.LdapFields) config.LdapFields {
	defaultMap := config.LdapFields{
		BaseFields: config.BaseFields{
			UserIdentifier: "mail",
			Email:          "mail",
			Firstname:      "givenName",
			Lastname:       "sn",
			Phone:          "telephoneNumber",
			Department:     "department",
		},
		GroupMembership: "memberOf",
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
	if f.GroupMembership != "" {
		defaultMap.GroupMembership = f.GroupMembership
	}

	return defaultMap
}
