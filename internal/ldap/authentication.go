package ldap

import (
	"crypto/tls"
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

type Authentication struct {
	Cfg *Config
}

func NewAuthentication(config Config) Authentication {
	a := Authentication{
		Cfg: &config,
	}

	return a
}

func (a Authentication) open() (*ldap.Conn, error) {
	conn, err := ldap.DialURL(a.Cfg.URL)
	if err != nil {
		return nil, err
	}

	if a.Cfg.StartTLS {
		// Reconnect with TLS
		err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return nil, err
		}
	}

	err = conn.Bind(a.Cfg.BindUser, a.Cfg.BindPass)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (a Authentication) close(conn *ldap.Conn) {
	if conn != nil {
		conn.Close()
	}
}

func (a Authentication) CheckLogin(username, password string) bool {
	return a.CheckCustomLogin("sAMAccountName", username, password)
}

func (a Authentication) CheckCustomLogin(userIdentifier, username, password string) bool {
	client, err := a.open()
	if err != nil {
		return false
	}
	defer a.close(client)

	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		a.Cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(%s=%s))", userIdentifier, username),
		[]string{"dn"},
		nil,
	)

	sr, err := client.Search(searchRequest)
	if err != nil {
		return false
	}

	if len(sr.Entries) != 1 {
		return false
	}

	userDN := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = client.Bind(userDN, password)
	if err != nil {
		return false
	}

	return true
}
