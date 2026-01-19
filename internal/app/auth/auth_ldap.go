package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// LdapAuthenticator is an authenticator that uses LDAP for authentication.
type LdapAuthenticator struct {
	cfg *config.LdapProvider
}

func newLdapAuthenticator(_ context.Context, cfg *config.LdapProvider) (*LdapAuthenticator, error) {
	return &LdapAuthenticator{cfg: cfg}, nil
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

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", ldap.EscapeFilter(string(userId)), -1)
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

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", ldap.EscapeFilter(string(userId)), -1)
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
		slog.Debug("LDAP user not found", "source", l.GetName(), "userId", userId, "filter", loginFilter)
		return nil, domain.ErrNotFound
	}

	if len(sr.Entries) > 1 {
		slog.Debug("LDAP user not unique",
			"source", l.GetName(), "userId", userId, "filter", loginFilter, "entries", len(sr.Entries))
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
	isAdmin := false
	adminInfoAvailable := false
	if l.cfg.FieldMap.GroupMembership != "" {
		adminInfoAvailable = true
		var err error
		isAdmin, err = internal.LdapIsMemberOf(raw[l.cfg.FieldMap.GroupMembership].([][]byte), l.cfg.ParsedAdminGroupDN)
		if err != nil {
			return nil, fmt.Errorf("failed to check admin group: %w", err)
		}
	}

	userInfo := &domain.AuthenticatorUserInfo{
		Identifier:         domain.UserIdentifier(internal.MapDefaultString(raw, l.cfg.FieldMap.UserIdentifier, "")),
		Email:              internal.MapDefaultString(raw, l.cfg.FieldMap.Email, ""),
		Firstname:          internal.MapDefaultString(raw, l.cfg.FieldMap.Firstname, ""),
		Lastname:           internal.MapDefaultString(raw, l.cfg.FieldMap.Lastname, ""),
		Phone:              internal.MapDefaultString(raw, l.cfg.FieldMap.Phone, ""),
		Department:         internal.MapDefaultString(raw, l.cfg.FieldMap.Department, ""),
		IsAdmin:            isAdmin,
		AdminInfoAvailable: adminInfoAvailable,
	}

	return userInfo, nil
}
