package ldap

import (
	"crypto/tls"
	"io/ioutil"
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
	loginFilter := strings.Replace(provider.config.LoginFilter, "{{login_identifier}}", username, -1)
	searchRequest := ldap.NewSearchRequest(
		provider.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		loginFilter,
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

	// Bind as the user to verify their password
	userDN := sr.Entries[0].DN
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
	loginFilter := strings.Replace(provider.config.LoginFilter, "{{login_identifier}}", username, -1)
	searchRequest := ldap.NewSearchRequest(
		provider.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		loginFilter,
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
	var tlsConfig *tls.Config

	if provider.config.LdapCertConn {

		cert_plain, err := ioutil.ReadFile(provider.config.LdapTlsCert)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to load the certificate")

		}

		key, err := ioutil.ReadFile(provider.config.LdapTlsKey)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to load the key")
		}

		cert_x509, err := tls.X509KeyPair(cert_plain, key)
		if err != nil {
			return nil, errors.WithMessage(err, "failed X509")

		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert_x509}}

	} else {

		tlsConfig = &tls.Config{InsecureSkipVerify: !provider.config.CertValidation}
	}

	conn, err := ldap.DialURL(provider.config.URL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to connect to LDAP")
	}

	if provider.config.StartTLS {
		// Reconnect with TLS
		err = conn.StartTLS(tlsConfig)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to start TLS session")
		}
	}

	err = conn.Bind(provider.config.BindUser, provider.config.BindPass)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to bind user")
	}

	return conn, nil
}

func (provider Provider) close(conn *ldap.Conn) {
	if conn != nil {
		conn.Close()
	}
}
