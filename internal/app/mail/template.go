package mail

import (
	"bytes"
	"embed"
	"fmt"
	htmlTemplate "html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/h44z/wg-portal/internal/domain"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

// TemplateHandler is a struct that holds the html and text templates.
type TemplateHandler struct {
	portalUrl     string
	portalName    string
	htmlTemplates *htmlTemplate.Template
	textTemplates *template.Template
}

func newTemplateHandler(portalUrl, portalName string, basePath string) (*TemplateHandler, error) {
	// Always parse embedded defaults first
	htmlTemplateCache, err := htmlTemplate.New("Html").ParseFS(TemplateFiles, "tpl_files/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded html template files: %w", err)
	}

	txtTemplateCache, err := template.New("Txt").ParseFS(TemplateFiles, "tpl_files/*.gotpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded text template files: %w", err)
	}

	// If a basePath is provided, ensure existence, populate if empty, then parse to override
	if basePath != "" {
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create templates base directory %s: %w", basePath, err)
		}

		hasTemplates, err := dirHasTemplates(basePath)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect templates directory: %w", err)
		}

		// If no templates present, copy embedded defaults to directory
		if !hasTemplates {
			if err := copyEmbeddedTemplates(basePath); err != nil {
				return nil, fmt.Errorf("failed to populate templates directory: %w", err)
			}
		}

		// Parse files from basePath to override embedded ones.
		// Only parse when matches exist to allow partial overrides without errors.
		if matches, _ := filepath.Glob(filepath.Join(basePath, "*.gohtml")); len(matches) > 0 {
			slog.Debug("parsing html email templates from base path", "base-path", basePath, "files", matches)
			if htmlTemplateCache, err = htmlTemplateCache.ParseFiles(matches...); err != nil {
				return nil, fmt.Errorf("failed to parse html templates from base path: %w", err)
			}
		}
		if matches, _ := filepath.Glob(filepath.Join(basePath, "*.gotpl")); len(matches) > 0 {
			slog.Debug("parsing text email templates from base path", "base-path", basePath, "files", matches)
			if txtTemplateCache, err = txtTemplateCache.ParseFiles(matches...); err != nil {
				return nil, fmt.Errorf("failed to parse text templates from base path: %w", err)
			}
		}
	}

	handler := &TemplateHandler{
		portalUrl:     portalUrl,
		portalName:    portalName,
		htmlTemplates: htmlTemplateCache,
		textTemplates: txtTemplateCache,
	}

	return handler, nil
}

// dirHasTemplates checks whether directory contains any .gohtml or .gotpl files.
func dirHasTemplates(basePath string) (bool, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".gohtml" || ext == ".gotpl" {
			return true, nil
		}
	}
	return false, nil
}

// copyEmbeddedTemplates writes embedded templates into basePath.
func copyEmbeddedTemplates(basePath string) error {
	list, err := fs.ReadDir(TemplateFiles, "tpl_files")
	if err != nil {
		return err
	}
	for _, entry := range list {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Only copy known template extensions
		if ext := filepath.Ext(name); ext != ".gohtml" && ext != ".gotpl" {
			continue
		}
		data, err := TemplateFiles.ReadFile(filepath.Join("tpl_files", name))
		if err != nil {
			return err
		}
		out := filepath.Join(basePath, name)
		if err := os.WriteFile(out, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// GetConfigMail returns the text and html template for the mail with a link.
func (c TemplateHandler) GetConfigMail(user *domain.User, link string) (io.Reader, io.Reader, error) {
	var tplBuff bytes.Buffer
	var htmlTplBuff bytes.Buffer

	err := c.textTemplates.ExecuteTemplate(&tplBuff, "mail_with_link.gotpl", map[string]any{
		"User":       user,
		"Link":       link,
		"PortalUrl":  c.portalUrl,
		"PortalName": c.portalName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_link.gotpl: %w", err)
	}

	err = c.htmlTemplates.ExecuteTemplate(&htmlTplBuff, "mail_with_link.gohtml", map[string]any{
		"User":       user,
		"Link":       link,
		"PortalUrl":  c.portalUrl,
		"PortalName": c.portalName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_link.gohtml: %w", err)
	}

	return &tplBuff, &htmlTplBuff, nil
}

// GetConfigMailWithAttachment returns the text and html template for the mail with an attachment.
func (c TemplateHandler) GetConfigMailWithAttachment(user *domain.User, cfgName, qrName string) (
	io.Reader,
	io.Reader,
	error,
) {
	var tplBuff bytes.Buffer
	var htmlTplBuff bytes.Buffer

	err := c.textTemplates.ExecuteTemplate(&tplBuff, "mail_with_attachment.gotpl", map[string]any{
		"User":           user,
		"ConfigFileName": cfgName,
		"QrcodePngName":  qrName,
		"PortalUrl":      c.portalUrl,
		"PortalName":     c.portalName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_attachment.gotpl: %w", err)
	}

	err = c.htmlTemplates.ExecuteTemplate(&htmlTplBuff, "mail_with_attachment.gohtml", map[string]any{
		"User":           user,
		"ConfigFileName": cfgName,
		"QrcodePngName":  qrName,
		"PortalUrl":      c.portalUrl,
		"PortalName":     c.portalName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template mail_with_attachment.gohtml: %w", err)
	}

	return &tplBuff, &htmlTplBuff, nil
}
