package common

import (
	"crypto/tls"
	"io"
	"time"

	"github.com/pkg/errors"
	mail "github.com/xhit/go-simple-mail/v2"
)

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
	Host           string         `yaml:"host" envconfig:"EMAIL_HOST"`
	Port           int            `yaml:"port" envconfig:"EMAIL_PORT"`
	TLS            bool           `yaml:"tls" envconfig:"EMAIL_TLS"` // Deprecated, use MailConfig.Encryption instead.
	Encryption     MailEncryption `yaml:"encryption" envconfig:"EMAIL_ENCRYPTION"`
	CertValidation bool           `yaml:"certcheck" envconfig:"EMAIL_CERT_VALIDATION"`
	Username       string         `yaml:"user" envconfig:"EMAIL_USERNAME"`
	Password       string         `yaml:"pass" envconfig:"EMAIL_PASSWORD"`
	AuthType       MailAuthType   `yaml:"auth" envconfig:"EMAIL_AUTHTYPE"`
}

type MailAttachment struct {
	Name        string
	ContentType string
	Data        io.Reader
	Embedded    bool
}

// SendEmailWithAttachments sends a mail with optional attachments.
func SendEmailWithAttachments(cfg MailConfig, sender, replyTo, subject, body, htmlBody string, receivers []string, attachments []MailAttachment) error {
	srv := mail.NewSMTPClient()

	srv.ConnectTimeout = 30 * time.Second
	srv.SendTimeout = 30 * time.Second
	srv.Host = cfg.Host
	srv.Port = cfg.Port
	srv.Username = cfg.Username
	srv.Password = cfg.Password

	// TODO: remove this once the deprecated MailConfig.TLS config option has been removed
	if cfg.TLS {
		cfg.Encryption = MailEncryptionStartTLS
	}
	switch cfg.Encryption {
	case MailEncryptionTLS:
		srv.Encryption = mail.EncryptionSSLTLS
	case MailEncryptionStartTLS:
		srv.Encryption = mail.EncryptionSTARTTLS
	default: // MailEncryptionNone
		srv.Encryption = mail.EncryptionNone
	}
	srv.TLSConfig = &tls.Config{ServerName: srv.Host, InsecureSkipVerify: !cfg.CertValidation}
	switch cfg.AuthType {
	case MailAuthPlain:
		srv.Authentication = mail.AuthPlain
	case MailAuthLogin:
		srv.Authentication = mail.AuthLogin
	case MailAuthCramMD5:
		srv.Authentication = mail.AuthCRAMMD5
	}

	client, err := srv.Connect()
	if err != nil {
		return errors.Wrap(err, "failed to connect via SMTP")
	}

	if replyTo == "" {
		replyTo = sender
	}

	email := mail.NewMSG()
	email.SetFrom(sender).
		AddTo(receivers...).
		SetReplyTo(replyTo).
		SetSubject(subject)

	email.SetBody(mail.TextHTML, htmlBody)
	email.AddAlternative(mail.TextPlain, body)

	for _, attachment := range attachments {
		attachmentData, err := io.ReadAll(attachment.Data)
		if err != nil {
			return errors.Wrapf(err, "failed to read attachment data for %s", attachment.Name)
		}

		if attachment.Embedded {
			email.AddInlineData(attachmentData, attachment.Name, attachment.ContentType)
		} else {
			email.AddAttachmentData(attachmentData, attachment.Name, attachment.ContentType)
		}
	}

	// Call Send and pass the client
	err = email.Send(client)
	if err != nil {
		return errors.Wrapf(err, "failed to send email")
	}
	return nil
}
