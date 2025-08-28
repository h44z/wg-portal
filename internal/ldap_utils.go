package internal

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-ldap/ldap/v3"

	"github.com/fedor-git/wg-portal-2/internal/config"
)

type RawLdapUser map[string]any

func LdapFindAllUsers(conn *ldap.Conn, baseDn, filter string, fields *config.LdapFields) ([]RawLdapUser, error) {
	// Search all users
	attrs := LdapSearchAttributes(fields)
	searchRequest := ldap.NewSearchRequest(
		baseDn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	results := LdapConvertEntries(sr, fields)

	return results, nil
}

func LdapConnect(cfg *config.LdapProvider) (*ldap.Conn, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: !cfg.CertValidation}
	if cfg.TlsCertificatePath != "" {
		certificate, err := os.ReadFile(cfg.TlsCertificatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)

		}

		key, err := os.ReadFile(cfg.TlsKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS key: %w", err)
		}

		keyPair, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, fmt.Errorf("failed to generate X509 keypair: %w", err)

		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{keyPair}}
	}

	conn, err := ldap.DialURL(cfg.URL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	if cfg.StartTLS { // Reconnect with TLS
		if err = conn.StartTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("failed to start TLS on connection: %w", err)
		}
	}

	if err = conn.Bind(cfg.BindUser, cfg.BindPass); err != nil {
		return nil, fmt.Errorf("failed to bind to LDAP: %w", err)
	}

	return conn, nil
}

func LdapDisconnect(conn *ldap.Conn) {
	if conn != nil {
		if err := conn.Close(); err != nil {
			slog.Error("failed to close ldap connection", "error", err)
		}
	}
}

func LdapConvertEntries(sr *ldap.SearchResult, fields *config.LdapFields) []RawLdapUser {
	users := make([]RawLdapUser, len(sr.Entries))

	for i, entry := range sr.Entries {
		userData := make(RawLdapUser)
		userData[fields.UserIdentifier] = entry.GetAttributeValue(fields.UserIdentifier)
		userData[fields.Email] = entry.GetAttributeValue(fields.Email)
		userData[fields.Firstname] = entry.GetAttributeValue(fields.Firstname)
		userData[fields.Lastname] = entry.GetAttributeValue(fields.Lastname)
		userData[fields.Phone] = entry.GetAttributeValue(fields.Phone)
		userData[fields.Department] = entry.GetAttributeValue(fields.Department)
		userData[fields.GroupMembership] = entry.GetRawAttributeValues(fields.GroupMembership)

		users[i] = userData
	}
	return users
}

func LdapSearchAttributes(fields *config.LdapFields) []string {
	attrs := []string{"dn", fields.UserIdentifier}

	if fields.Email != "" {
		attrs = append(attrs, fields.Email)
	}
	if fields.Firstname != "" {
		attrs = append(attrs, fields.Firstname)
	}
	if fields.Lastname != "" {
		attrs = append(attrs, fields.Lastname)
	}
	if fields.Phone != "" {
		attrs = append(attrs, fields.Phone)
	}
	if fields.Department != "" {
		attrs = append(attrs, fields.Department)
	}
	if fields.GroupMembership != "" {
		attrs = append(attrs, fields.GroupMembership)
	}

	return UniqueStringSlice(attrs)
}

// LdapIsMemberOf checks if the groupData array contains the group DN
func LdapIsMemberOf(groupData [][]byte, groupDN *ldap.DN) (bool, error) {
	for _, group := range groupData {
		dn, err := ldap.ParseDN(string(group))
		if err != nil {
			return false, fmt.Errorf("failed to parse group DN: %w", err)
		}
		if groupDN.Equal(dn) {
			return true, nil
		}
	}

	return false, nil
}
