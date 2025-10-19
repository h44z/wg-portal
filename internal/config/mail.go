package config

// MailEncryption is the type of the SMTP encryption.
// Supported: none, tls, starttls
type MailEncryption string

const (
	MailEncryptionNone     MailEncryption = "none"
	MailEncryptionTLS      MailEncryption = "tls"
	MailEncryptionStartTLS MailEncryption = "starttls"
)

// MailAuthType is the type of the SMTP authentication.
// Supported: plain, login, crammd5
type MailAuthType string

const (
	MailAuthPlain   MailAuthType = "plain"
	MailAuthLogin   MailAuthType = "login"
	MailAuthCramMD5 MailAuthType = "crammd5"
)

// MailConfig contains the configuration for the mail server which is used to send emails.
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
	// AllowPeerEmail specifies whether emails should be sent to peers which have no valid user account linked, but an email address is set as "user".
	AllowPeerEmail bool `yaml:"allow_peer_email"`
}
