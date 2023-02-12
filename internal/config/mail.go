package config

type MailEncryption string

const (
	MailEncryptionNone     MailEncryption = "none"
	MailEncryptionTLS      MailEncryption = "tls"
	MailEncryptionStartTLS MailEncryption = "starttls"
)

type MailAuthType string

const (
	MailAuthPlain   MailAuthType = "plain"
	MailAuthLogin   MailAuthType = "login"
	MailAuthCramMD5 MailAuthType = "crammd5"
)

type MailConfig struct {
	Host           string         `envconfig:"EMAIL_HOST"`
	Port           int            `envconfig:"EMAIL_PORT"`
	Encryption     MailEncryption `envconfig:"EMAIL_ENCRYPTION"`
	CertValidation bool           `envconfig:"EMAIL_CERT_VALIDATION"`
	Username       string         `envconfig:"EMAIL_USERNAME"`
	Password       string         `envconfig:"EMAIL_PASSWORD"`
	AuthType       MailAuthType   `envconfig:"EMAIL_AUTHTYPE"`

	From string `envconfig:"EMAIL_FROM"`
}
