package common

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/pkg/errors"

	"github.com/go-ldap/ldap/v3"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/h44z/wg-portal/internal/user"
)

type LdapAuthenticator interface {
	user.Authenticator
	GetAllUserInfos(ctx context.Context) ([]map[string]interface{}, error)
	GetUserInfo(ctx context.Context, username persistence.UserIdentifier) (map[string]interface{}, error)
	ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error)
	RegistrationEnabled() bool
	SynchronizationEnabled() bool
}

type ldapAuthenticator struct {
	cfg *LdapProvider
}

func NewLdapAuthenticator(_ context.Context, cfg *LdapProvider) (*ldapAuthenticator, error) {
	var authenticator = &ldapAuthenticator{}

	authenticator.cfg = cfg

	dn, err := ldap.ParseDN(cfg.AdminGroupDN)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse admin group DN")
	}
	authenticator.cfg.FieldMap = getLdapFieldMapping(cfg.FieldMap)
	authenticator.cfg.adminGroupDN = dn

	return authenticator, nil
}

func (l *ldapAuthenticator) RegistrationEnabled() bool {
	return l.cfg.RegistrationEnabled
}

func (l *ldapAuthenticator) SynchronizationEnabled() bool {
	return l.cfg.Synchronize
}

func (l *ldapAuthenticator) PlaintextAuthentication(userId persistence.UserIdentifier, plainPassword string) error {
	conn, err := l.connect()
	if err != nil {
		return errors.WithMessage(err, "failed to setup connection")
	}
	defer l.disconnect(conn)

	attrs := []string{"dn"}

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", string(userId), -1)
	searchRequest := ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 20, false, // 20 second time limit
		loginFilter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return errors.Wrapf(err, "failed to search in ldap")
	}

	if len(sr.Entries) == 0 {
		return errors.New("user not found")
	}

	if len(sr.Entries) > 1 {
		return errors.New("no unique user found")
	}

	// Bind as the user to verify their password
	userDN := sr.Entries[0].DN
	err = conn.Bind(userDN, plainPassword)
	if err != nil {
		return errors.Wrapf(err, "invalid credentials")
	}
	_ = conn.Unbind()

	return nil
}

func (l *ldapAuthenticator) HashedAuthentication(_ persistence.UserIdentifier, _ string) error {
	// TODO: is this possible?
	return errors.New("unimplemented")
}

func (l *ldapAuthenticator) GetUserInfo(_ context.Context, userId persistence.UserIdentifier) (map[string]interface{}, error) {
	conn, err := l.connect()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to setup connection")
	}
	defer l.disconnect(conn)

	attrs := l.getLdapSearchAttributes()

	loginFilter := strings.Replace(l.cfg.LoginFilter, "{{login_identifier}}", string(userId), -1)
	searchRequest := ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 20, false, // 20 second time limit
		loginFilter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search in ldap")
	}

	if len(sr.Entries) == 0 {
		return nil, errors.New("user not found")
	}

	if len(sr.Entries) > 1 {
		return nil, errors.New("no unique user found")
	}

	users := l.convertLdapEntries(sr)

	return users[0], nil
}

func (l *ldapAuthenticator) GetAllUserInfos(_ context.Context) ([]map[string]interface{}, error) {
	conn, err := l.connect()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to setup connection")
	}
	defer l.disconnect(conn)

	attrs := l.getLdapSearchAttributes()

	searchRequest := ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 20, false, // 20 second time limit
		l.cfg.SyncFilter, attrs, nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search in ldap")
	}

	users := l.convertLdapEntries(sr)

	return users, nil
}

func (l *ldapAuthenticator) convertLdapEntries(sr *ldap.SearchResult) []map[string]interface{} {
	users := make([]map[string]interface{}, len(sr.Entries))

	fieldMap := l.cfg.FieldMap
	for i, entry := range sr.Entries {
		userData := make(map[string]interface{})
		userData[fieldMap.UserIdentifier] = entry.DN
		userData[fieldMap.Email] = entry.GetAttributeValue(fieldMap.Email)
		userData[fieldMap.Firstname] = entry.GetAttributeValue(fieldMap.Firstname)
		userData[fieldMap.Lastname] = entry.GetAttributeValue(fieldMap.Lastname)
		userData[fieldMap.Phone] = entry.GetAttributeValue(fieldMap.Phone)
		userData[fieldMap.Department] = entry.GetAttributeValue(fieldMap.Department)
		userData[fieldMap.GroupMembership] = entry.GetRawAttributeValues(fieldMap.GroupMembership)

		users[i] = userData
	}
	return users
}

func (l *ldapAuthenticator) getLdapSearchAttributes() []string {
	fieldMap := l.cfg.FieldMap
	attrs := []string{"dn", fieldMap.UserIdentifier}
	if fieldMap.Email != "" {
		attrs = append(attrs, fieldMap.Email)
	}
	if fieldMap.Firstname != "" {
		attrs = append(attrs, fieldMap.Firstname)
	}
	if fieldMap.Lastname != "" {
		attrs = append(attrs, fieldMap.Lastname)
	}
	if fieldMap.Phone != "" {
		attrs = append(attrs, fieldMap.Phone)
	}
	if fieldMap.Department != "" {
		attrs = append(attrs, fieldMap.Department)
	}
	if fieldMap.GroupMembership != "" {
		attrs = append(attrs, fieldMap.GroupMembership)
	}

	return uniqueStringSlice(attrs)
}

func (l ldapAuthenticator) ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error) {
	isAdmin, err := userIsInAdminGroup(raw[l.cfg.FieldMap.GroupMembership].([][]byte), l.cfg.adminGroupDN)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to check admin group")
	}
	userInfo := &AuthenticatorUserInfo{
		Identifier: persistence.UserIdentifier(mapDefaultString(raw, l.cfg.FieldMap.UserIdentifier, "")),
		Email:      mapDefaultString(raw, l.cfg.FieldMap.Email, ""),
		Firstname:  mapDefaultString(raw, l.cfg.FieldMap.Firstname, ""),
		Lastname:   mapDefaultString(raw, l.cfg.FieldMap.Lastname, ""),
		Phone:      mapDefaultString(raw, l.cfg.FieldMap.Phone, ""),
		Department: mapDefaultString(raw, l.cfg.FieldMap.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}

func (l *ldapAuthenticator) connect() (*ldap.Conn, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: !l.cfg.CertValidation}
	conn, err := ldap.DialURL(l.cfg.URL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to LDAP")
	}

	if l.cfg.StartTLS { // Reconnect with TLS
		if err = conn.StartTLS(tlsConfig); err != nil {
			return nil, errors.Wrap(err, "failed to start TLS on connection")
		}
	}

	if err = conn.Bind(l.cfg.BindUser, l.cfg.BindPass); err != nil {
		return nil, errors.Wrap(err, "failed to bind to LDAP")
	}

	return conn, nil
}

func (l *ldapAuthenticator) disconnect(conn *ldap.Conn) {
	if conn != nil {
		conn.Close()
	}
}

func userIsInAdminGroup(groupData [][]byte, adminGroupDN *ldap.DN) (bool, error) {
	for _, group := range groupData {
		dn, err := ldap.ParseDN(string(group))
		if err != nil {
			return false, errors.WithMessage(err, "failed to parse group DN")
		}
		if adminGroupDN.Equal(dn) {
			return true, nil
		}
	}

	return false, nil
}

func getLdapFieldMapping(f LdapFields) LdapFields {
	defaultMap := LdapFields{
		BaseFields: BaseFields{
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

// uniqueStringSlice removes duplicates in the given string slice
func uniqueStringSlice(slice []string) []string {
	keys := make(map[string]struct{})
	uniqueSlice := make([]string, 0, len(slice))
	for _, entry := range slice {
		if _, exists := keys[entry]; !exists {
			keys[entry] = struct{}{}
			uniqueSlice = append(uniqueSlice, entry)
		}
	}
	return uniqueSlice
}
