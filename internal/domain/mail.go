package domain

import "io"

type MailOptions struct {
	ReplyTo     string // defaults to the sender
	HtmlBody    string // if html body is empty, a text-only email will be sent
	Cc          []string
	Bcc         []string
	Attachments []MailAttachment
}

type MailAttachment struct {
	Name        string
	ContentType string
	Data        io.Reader
	Embedded    bool
}
