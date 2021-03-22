package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-ldap/ldap/v3"
	"github.com/h44z/wg-portal/internal/authentication"
	ldapconfig "github.com/h44z/wg-portal/internal/ldap"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
)

// Provider implements a password login method for an LDAP backend.
type Provider struct {
	config *ldapconfig.Config
}

func New(cfg *ldapconfig.Config) (*Provider, error) {
	p := &Provider{
		config: cfg,
	}

	// test ldap connectivity
	client, err := p.open()
	if err != nil {
		return nil, errors.Wrap(err, "unable to open ldap connection")
	}
	defer p.close(client)

	return p, nil
}

// GetName return provider name
func (Provider) GetName() string {
	return string(users.UserSourceLdap)
}

// GetType return provider type
func (Provider) GetType() authentication.AuthProviderType {
	return authentication.AuthProviderTypePassword
}

// GetPriority return provider priority
func (Provider) GetPriority() int {
	return 1 // LDAP password provider
}

func (provider Provider) SetupRoutes(routes *gin.RouterGroup) {
	// nothing todo here
}

func (provider Provider) Login(ctx *authentication.AuthContext) (string, error) {
	username := strings.ToLower(ctx.Username)
	password := ctx.Password

	// Validate input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		return "", errors.New("empty username or password")
	}

	client, err := provider.open()
	if err != nil {
		return "", errors.Wrap(err, "unable to open ldap connection")
	}
	defer provider.close(client)

	// Search for the given username
	attrs := []string{"dn", provider.config.EmailAttribute}
	if provider.config.DisabledAttribute != "" {
		attrs = append(attrs, provider.config.DisabledAttribute)
	}
	searchRequest := ldap.NewSearchRequest(
		provider.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=%s)(%s=%s))", provider.config.UserClass, provider.config.EmailAttribute, username),
		attrs,
		nil,
	)

	sr, err := client.Search(searchRequest)
	if err != nil {
		return "", errors.Wrap(err, "unable to find user in ldap")
	}

	if len(sr.Entries) != 1 {
		return "", errors.Errorf("invalid amount of ldap entries (%d)", len(sr.Entries))
	}

	userDN := sr.Entries[0].DN

	// Check if user is disabled, if so deny login
	if provider.config.DisabledAttribute != "" {
		uac := sr.Entries[0].GetAttributeValue(provider.config.DisabledAttribute)
		switch provider.config.Type {
		case ldapconfig.TypeActiveDirectory:
			if ldapconfig.IsActiveDirectoryUserDisabled(uac) {
				return "", errors.New("user is disabled")
			}
		case ldapconfig.TypeOpenLDAP:
			if ldapconfig.IsOpenLdapUserDisabled(uac) {
				return "", errors.New("user is disabled")
			}
		}
	}

	// Bind as the user to verify their password
	err = client.Bind(userDN, password)
	if err != nil {
		return "", errors.Wrapf(err, "invalid credentials")
	}

	return sr.Entries[0].GetAttributeValue(provider.config.EmailAttribute), nil
}

func (provider Provider) Logout(context *authentication.AuthContext) error {
	return nil // nothing todo here
}

func (provider Provider) GetUserModel(ctx *authentication.AuthContext) (*authentication.User, error) {
	username := strings.ToLower(ctx.Username)

	// Validate input
	if strings.Trim(username, " ") == "" {
		return nil, errors.New("empty username")
	}

	client, err := provider.open()
	if err != nil {
		return nil, errors.Wrap(err, "unable to open ldap connection")
	}
	defer provider.close(client)

	// Search for the given username
	attrs := []string{"dn", provider.config.EmailAttribute, provider.config.FirstNameAttribute, provider.config.LastNameAttribute,
		provider.config.PhoneAttribute, provider.config.GroupMemberAttribute}
	if provider.config.DisabledAttribute != "" {
		attrs = append(attrs, provider.config.DisabledAttribute)
	}
	searchRequest := ldap.NewSearchRequest(
		provider.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=%s)(%s=%s))", provider.config.UserClass, provider.config.EmailAttribute, username),
		attrs,
		nil,
	)

	sr, err := client.Search(searchRequest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to find user in ldap")
	}

	if len(sr.Entries) != 1 {
		return nil, errors.Wrapf(err, "invalid amount of ldap entries (%d)", len(sr.Entries))
	}

	user := &authentication.User{
		Firstname: sr.Entries[0].GetAttributeValue(provider.config.FirstNameAttribute),
		Lastname:  sr.Entries[0].GetAttributeValue(provider.config.LastNameAttribute),
		Email:     sr.Entries[0].GetAttributeValue(provider.config.EmailAttribute),
		Phone:     sr.Entries[0].GetAttributeValue(provider.config.PhoneAttribute),
		IsAdmin:   false,
	}

	for _, group := range sr.Entries[0].GetAttributeValues(provider.config.GroupMemberAttribute) {
		if group == provider.config.AdminLdapGroup {
			user.IsAdmin = true
			break
		}
	}

	return user, nil
}

func (provider Provider) open() (*ldap.Conn, error) {
	conn, err := ldap.DialURL(provider.config.URL)
	if err != nil {
		return nil, err
	}

	if provider.config.StartTLS {
		// Reconnect with TLS
		err = conn.StartTLS(&tls.Config{InsecureSkipVerify: !provider.config.CertValidation})
		if err != nil {
			return nil, err
		}
	}

	err = conn.Bind(provider.config.BindUser, provider.config.BindPass)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (provider Provider) close(conn *ldap.Conn) {
	if conn != nil {
		conn.Close()
	}
}
