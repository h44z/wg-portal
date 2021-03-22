package ldap

import (
	"crypto/tls"
	"fmt"
	"strconv"

	"github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"
)

type RawLdapData struct {
	DN            string
	Attributes    map[string]string
	RawAttributes map[string][][]byte
}

func Open(cfg *Config) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(cfg.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to LDAP")
	}

	if cfg.StartTLS {
		// Reconnect with TLS
		err = conn.StartTLS(&tls.Config{InsecureSkipVerify: !cfg.CertValidation})
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
	if cfg.DisabledAttribute != "" {
		attrs = append(attrs, cfg.DisabledAttribute)
	}
	searchRequest := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(objectClass=%s)", cfg.UserClass), attrs, nil,
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

func IsActiveDirectoryUserDisabled(userAccountControl string) bool {
	if userAccountControl == "" {
		return false
	}

	uacInt, err := strconv.ParseInt(userAccountControl, 10, 32)
	if err != nil {
		return true
	}
	if int32(uacInt)&0x2 != 0 {
		return true // bit 2 set means account is disabled
	}

	return false
}

func IsOpenLdapUserDisabled(pwdAccountLockedTime string) bool {
	if pwdAccountLockedTime != "" {
		return true
	}

	return false
}
