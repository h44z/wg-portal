package ldap

import (
	"crypto/tls"
	"io/ioutil"

	"github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"
)

type RawLdapData struct {
	DN            string
	Attributes    map[string]string
	RawAttributes map[string][][]byte
}

func Open(cfg *Config) (*ldap.Conn, error) {
	var tlsConfig *tls.Config

	if cfg.LdapCertConn {

		cert_plain, err := ioutil.ReadFile(cfg.LdapTlsCert)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to load the certificate")

		}

		key, err := ioutil.ReadFile(cfg.LdapTlsKey)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to load the key")
		}

		cert_x509, err := tls.X509KeyPair(cert_plain, key)
		if err != nil {
			return nil, errors.WithMessage(err, "failed X509")

		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert_x509}}

	} else {

		tlsConfig = &tls.Config{InsecureSkipVerify: !cfg.CertValidation}
	}

	conn, err := ldap.DialURL(cfg.URL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to LDAP")
	}

	if cfg.StartTLS {
		// Reconnect with TLS
		err = conn.StartTLS(tlsConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to star TLS on connection")
		}
	}

	err = conn.Bind(cfg.BindUser, cfg.BindPass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to bind to LDAP")
	}

	return conn, nil
}

func Close(conn *ldap.Conn) {
	if conn != nil {
		conn.Close()
	}
}

func FindAllUsers(cfg *Config) ([]RawLdapData, error) {
	client, err := Open(cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to open ldap connection")
	}
	defer Close(client)

	// Search all users
	attrs := []string{"dn", cfg.EmailAttribute, cfg.EmailAttribute, cfg.FirstNameAttribute, cfg.LastNameAttribute,
		cfg.PhoneAttribute, cfg.GroupMemberAttribute}
	searchRequest := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		cfg.SyncFilter, attrs, nil,
	)

	sr, err := client.Search(searchRequest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search in ldap")
	}

	tmpData := make([]RawLdapData, 0, len(sr.Entries))

	for _, entry := range sr.Entries {
		tmp := RawLdapData{
			DN:            entry.DN,
			Attributes:    make(map[string]string, len(attrs)),
			RawAttributes: make(map[string][][]byte, len(attrs)),
		}

		for _, field := range attrs {
			tmp.Attributes[field] = entry.GetAttributeValue(field)
			tmp.RawAttributes[field] = entry.GetRawAttributeValues(field)
		}

		tmpData = append(tmpData, tmp)
	}

	return tmpData, nil
}
