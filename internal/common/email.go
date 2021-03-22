package common

import (
	"crypto/tls"
	"io"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/jordan-wright/email"
)

type MailConfig struct {
	Host           string `yaml:"host" envconfig:"EMAIL_HOST"`
	Port           int    `yaml:"port" envconfig:"EMAIL_PORT"`
	TLS            bool   `yaml:"tls" envconfig:"EMAIL_TLS"`
	CertValidation bool   `yaml:"certcheck" envconfig:"EMAIL_CERT_VALIDATION"`
	Username       string `yaml:"user" envconfig:"EMAIL_USERNAME"`
	Password       string `yaml:"pass" envconfig:"EMAIL_PASSWORD"`
}

type MailAttachment struct {
	Name        string
	ContentType string
	Data        io.Reader
	Embedded    bool
}

// SendEmailWithAttachments sends a mail with optional attachments.
func SendEmailWithAttachments(cfg MailConfig, sender, replyTo, subject, body string, htmlBody string, receivers []string, attachments []MailAttachment) error {
	e := email.NewEmail()

	hostname := cfg.Host + ":" + strconv.Itoa(cfg.Port)
	subject = strings.Trim(subject, "\n\r\t")
	sender = strings.Trim(sender, "\n\r\t")
	replyTo = strings.Trim(replyTo, "\n\r\t")
	if replyTo == "" {
		replyTo = sender
	}

	var auth smtp.Auth
	if cfg.Username == "" {
		auth = nil
	} else {
		// Set up authentication information.
		auth = smtp.PlainAuth(
			"",
			cfg.Username,
			cfg.Password,
			cfg.Host,
		)
	}

	// Set email data.
	e.From = sender
	e.To = receivers
	e.ReplyTo = []string{replyTo}
	e.Subject = subject
	e.Text = []byte(body)
	if htmlBody != "" {
		e.HTML = []byte(htmlBody)
	}

	for _, attachment := range attachments {
		a, err := e.Attach(attachment.Data, attachment.Name, attachment.ContentType)
		if err != nil {
			return err
		}
		if attachment.Embedded {
			a.HTMLRelated = true
		}
	}

	if cfg.TLS {
		return e.SendWithStartTLS(hostname, auth, &tls.Config{InsecureSkipVerify: !cfg.CertValidation})
	} else {
		return e.Send(hostname, auth)
	}
}
