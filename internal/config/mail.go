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
	// Host is the hostname or IP of the SMTP server
	Host string `yaml:"host"`
	// Port is the port number for the SMTP server
	Port int `yaml:"port"`
	// Encryption is the SMTP encryption type
	Encryption MailEncryption `yaml:"encryption"`
	// CertValidation specifies whether the SMTP server certificate should be validated
	CertValidation bool `yaml:"cert_validation"`
	// Username is the optional SMTP username for authentication
	Username string `yaml:"username"`
	// Password is the optional SMTP password for authentication
	Password string `yaml:"password"`
	// AuthType is the SMTP authentication type
	AuthType MailAuthType `yaml:"auth_type"`

	// From is the default "From" address when sending emails
	From string `yaml:"from"`
	// LinkOnly specifies whether emails should only contain a link to WireGuard Portal or attach the full configuration
	LinkOnly bool `yaml:"link_only"`
}
