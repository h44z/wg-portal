package adapters

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type MailRepo struct {
	cfg *config.MailConfig
}

// NewSmtpMailRepo creates a new MailRepo instance.
func NewSmtpMailRepo(cfg config.MailConfig) MailRepo {
	return MailRepo{cfg: &cfg}
}

// Send sends a mail using SMTP.
func (r MailRepo) Send(_ context.Context, subject, body string, to []string, options *domain.MailOptions) error {
	if options == nil {
		options = &domain.MailOptions{}
	}
	r.setDefaultOptions(r.cfg.From, options)

	if len(to) == 0 {
		return errors.New("missing email recipient")
	}

	uniqueTo := internal.UniqueStringSlice(to)
	email := mail.NewMSG()
	email.SetFrom(r.cfg.From).
		AddTo(uniqueTo...).
		SetReplyTo(options.ReplyTo).
		SetSubject(subject).
		SetBody(mail.TextPlain, body)

	if len(options.Cc) > 0 {
		// the underlying mail library does not allow the same address to appear in TO and CC... so filter entries that are already included
		// in the TO addresses
		cc := RemoveDuplicates(internal.UniqueStringSlice(options.Cc), uniqueTo)
		email.AddCc(cc...)
	}
	if len(options.Bcc) > 0 {
		// the underlying mail library does not allow the same address to appear in TO or CC and BCC... so filter entries that are already
		// included in the TO and CC addresses
		bcc := RemoveDuplicates(internal.UniqueStringSlice(options.Bcc), uniqueTo)
		bcc = RemoveDuplicates(bcc, options.Cc)

		email.AddCc(internal.UniqueStringSlice(options.Bcc)...)
	}
	if options.HtmlBody != "" {
		email.AddAlternative(mail.TextHTML, options.HtmlBody)
	}

	for _, attachment := range options.Attachments {
		attachmentData, err := io.ReadAll(attachment.Data)
		if err != nil {
			return fmt.Errorf("failed to read attachment data for %s: %w", attachment.Name, err)
		}

		if attachment.Embedded {
			email.AddInlineData(attachmentData, attachment.Name, attachment.ContentType)
		} else {
			email.AddAttachmentData(attachmentData, attachment.Name, attachment.ContentType)
		}
	}

	// Call Send and pass the client
	srv := r.getMailServer()
	client, err := srv.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	err = email.Send(client)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (r MailRepo) setDefaultOptions(sender string, options *domain.MailOptions) {
	if options.ReplyTo == "" {
		options.ReplyTo = sender
	}
}

func (r MailRepo) getMailServer() *mail.SMTPServer {
	srv := mail.NewSMTPClient()

	srv.ConnectTimeout = 30 * time.Second
	srv.SendTimeout = 30 * time.Second
	srv.Host = r.cfg.Host
	srv.Port = r.cfg.Port
	srv.Username = r.cfg.Username
	srv.Password = r.cfg.Password

	switch r.cfg.Encryption {
	case config.MailEncryptionTLS:
		srv.Encryption = mail.EncryptionSSLTLS
	case config.MailEncryptionStartTLS:
		srv.Encryption = mail.EncryptionSTARTTLS
	default: // MailEncryptionNone
		srv.Encryption = mail.EncryptionNone
	}
	srv.TLSConfig = &tls.Config{ServerName: srv.Host, InsecureSkipVerify: !r.cfg.CertValidation}
	switch r.cfg.AuthType {
	case config.MailAuthPlain:
		srv.Authentication = mail.AuthPlain
	case config.MailAuthLogin:
		srv.Authentication = mail.AuthLogin
	case config.MailAuthCramMD5:
		srv.Authentication = mail.AuthCRAMMD5
	}

	return srv
}

// RemoveDuplicates removes addresses from the given string slice which are contained in the remove slice.
func RemoveDuplicates(slice []string, remove []string) []string {
	uniqueSlice := make([]string, 0, len(slice))

	for _, i := range remove {
		for _, j := range slice {
			if i != j {
				uniqueSlice = append(uniqueSlice, j)
			}
		}
	}
	return uniqueSlice
}
