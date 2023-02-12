package app

import (
	"bytes"
	"embed"
	"fmt"
	htmlTemplate "html/template"
	"io"
	"text/template"

	"github.com/h44z/wg-portal/internal/domain"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

type templateHandler struct {
	wireGuardTemplates *template.Template

	mailHtmlTemplates *htmlTemplate.Template
	mailTextTemplates *template.Template
}

func newTemplateHandler() (*templateHandler, error) {
	templateCache, err := template.New("WireGuard").ParseFS(TemplateFiles, "tpl_files/*.tpl")
	if err != nil {
		return nil, err
	}

	mailHtmlTemplateCache, err := htmlTemplate.New("WireGuard").ParseFS(TemplateFiles, "tpl_files/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse html template files: %w", err)
	}

	mailTxtTemplateCache, err := template.New("WireGuard").ParseFS(TemplateFiles, "tpl_files/*.gotpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse text template files: %w", err)
	}

	handler := &templateHandler{
		wireGuardTemplates: templateCache,
		mailHtmlTemplates:  mailHtmlTemplateCache,
		mailTextTemplates:  mailTxtTemplateCache,
	}

	return handler, nil
}

func (c templateHandler) GetInterfaceConfig(cfg *domain.Interface, peers []*domain.Peer) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.wireGuardTemplates.ExecuteTemplate(&tplBuff, "wg_interface.tpl", map[string]interface{}{
		"Interface": cfg,
		"Peers":     peers,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute interface template for %s: %w", cfg.Identifier, err)
	}

	return &tplBuff, nil
}

func (c templateHandler) GetPeerConfig(peer *domain.Peer) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.wireGuardTemplates.ExecuteTemplate(&tplBuff, "wg_peer.tpl", map[string]interface{}{
		"Peer": peer,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute peer template for %s: %w", peer.Identifier, err)
	}

	return &tplBuff, nil
}

func (c templateHandler) GetConfigMail(user *domain.User, peer *domain.Peer, link string) (io.Reader, io.Reader, error) {
	var tplBuff bytes.Buffer
	var htmlTplBuff bytes.Buffer

	err := c.mailTextTemplates.ExecuteTemplate(&tplBuff, "mail_with_link.gotpl", map[string]interface{}{
		"User": user,
		"Peer": peer,
		"Link": link,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_link.gotpl")
	}

	err = c.mailHtmlTemplates.ExecuteTemplate(&tplBuff, "mail_with_link.gohtml", map[string]interface{}{
		"User": user,
		"Peer": peer,
		"Link": link,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_link.gohtml")
	}

	return &tplBuff, &htmlTplBuff, nil
}

func (c templateHandler) GetConfigMailWithAttachment(user *domain.User, peer *domain.Peer) (io.Reader, io.Reader, error) {
	var tplBuff bytes.Buffer
	var htmlTplBuff bytes.Buffer

	err := c.mailTextTemplates.ExecuteTemplate(&tplBuff, "mail_with_attachment.gotpl", map[string]interface{}{
		"User": user,
		"Peer": peer,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_attachment.gotpl")
	}

	err = c.mailHtmlTemplates.ExecuteTemplate(&tplBuff, "mail_with_attachment.gohtml", map[string]interface{}{
		"User": user,
		"Peer": peer,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_attachment.gohtml")
	}

	return &tplBuff, &htmlTplBuff, nil
}
