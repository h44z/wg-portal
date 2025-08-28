package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/fedor-git/wg-portal-2/internal"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// LdapAuthenticator is an authenticator that uses LDAP for authentication.
type LdapAuthenticator struct {
	cfg *config.LdapProvider
}

func newLdapAuthenticator(_ context.Context, cfg *config.LdapProvider) (*LdapAuthenticator, error) {
	var provider = &LdapAuthenticator{}

	provider.cfg = cfg

	dn, err := ldap.ParseDN(cfg.AdminGroupDN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse admin group DN: %w", err)
	}
	provider.cfg.FieldMap = provider.getLdapFieldMapping(cfg.FieldMap)
	provider.cfg.ParsedAdminGroupDN = dn

	return provider, nil
}

// GetName returns the name of the LDAP authenticator.
func (l LdapAuthenticator) GetName() string {
	return l.cfg.ProviderName
}

// RegistrationEnabled returns whether registration is enabled for the LDAP authenticator.
func (l LdapAuthenticator) RegistrationEnabled() bool {
	return l.cfg.RegistrationEnabled
}

// PlaintextAuthentication performs a plaintext authentication against the LDAP server.
func (l LdapAuthenticator) PlaintextAuthentication(userId domain.UserIdentifier, plainPassword string) error {
	conn, err := internal.LdapConnect(l.cfg)
	if err != nil {
		return fmt.Errorf("failed to setup connection: %w", err)
	}
	defer internal.LdapDisconnect(conn)

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

// GetUserInfo retrieves user information from the LDAP server.
// If the user is not found, domain.ErrNotFound is returned.
// If multiple users are found, domain.ErrNotUnique is returned.
func (l LdapAuthenticator) GetUserInfo(_ context.Context, userId domain.UserIdentifier) (
	map[string]any,
	error,
) {
	conn, err := internal.LdapConnect(l.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup connection: %w", err)
	}
	defer internal.LdapDisconnect(conn)

	attrs := internal.LdapSearchAttributes(&l.cfg.FieldMap)

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

	users := internal.LdapConvertEntries(sr, &l.cfg.FieldMap)

	if l.cfg.LogUserInfo {
		contents, _ := json.Marshal(users[0])
		slog.Debug("LDAP user info",
			"source", l.GetName(),
			"userId", userId,
			"info", string(contents))
	}

	return users[0], nil
}

// ParseUserInfo parses the user information from the LDAP server into a domain.AuthenticatorUserInfo struct.
func (l LdapAuthenticator) ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error) {
	isAdmin, err := internal.LdapIsMemberOf(raw[l.cfg.FieldMap.GroupMembership].([][]byte), l.cfg.ParsedAdminGroupDN)
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

func (l LdapAuthenticator) getLdapFieldMapping(f config.LdapFields) config.LdapFields {
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
