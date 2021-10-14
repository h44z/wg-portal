package portal

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
	Host                 string         `yaml:"host" envconfig:"EMAIL_HOST"`
	Port                 int            `yaml:"port" envconfig:"EMAIL_PORT"`
	Encryption           MailEncryption `yaml:"encryption" envconfig:"EMAIL_ENCRYPTION"`
	CertValidation       bool           `yaml:"certCheck" envconfig:"EMAIL_CERT_VALIDATION"`
	Username             string         `yaml:"user" envconfig:"EMAIL_USERNAME"`
	Password             string         `yaml:"pass" envconfig:"EMAIL_PASSWORD"`
	AuthType             MailAuthType   `yaml:"auth" envconfig:"EMAIL_AUTHTYPE"`
	MailFrom             string         `yaml:"mailFrom" envconfig:"MAIL_FROM"`
	IncludeSensitiveData bool           `yaml:"withSensitiveData" envconfig:"EMAIL_INCLUDE_SENSITIVE_DATA"`
}
