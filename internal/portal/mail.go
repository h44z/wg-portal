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
	Host                 string         `yaml:"host"`
	Port                 int            `yaml:"port"`
	Encryption           MailEncryption `yaml:"encryption"`
	CertValidation       bool           `yaml:"cert_validation"`
	Username             string         `yaml:"user"`
	Password             string         `yaml:"pass"`
	AuthType             MailAuthType   `yaml:"auth"`
	MailFrom             string         `yaml:"mail_from"`
	IncludeSensitiveData bool           `yaml:"include_sensitive_data"`
}
