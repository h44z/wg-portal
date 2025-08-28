package configfile

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"text/template"

	"github.com/fedor-git/wg-portal-2/internal/domain"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

// TemplateHandler is responsible for rendering the WireGuard configuration files
// based on the provided templates.
type TemplateHandler struct {
	templates *template.Template
}

func newTemplateHandler() (*TemplateHandler, error) {
	tplFuncs := template.FuncMap{
		"CidrsToString": domain.CidrsToString,
	}

	templateCache, err := template.New("WireGuard").Funcs(tplFuncs).ParseFS(TemplateFiles, "tpl_files/*.tpl")
	if err != nil {
		return nil, err
	}

	handler := &TemplateHandler{
		templates: templateCache,
	}

	return handler, nil
}

// GetInterfaceConfig returns the rendered configuration file for a WireGuard interface.
func (c TemplateHandler) GetInterfaceConfig(cfg *domain.Interface, peers []domain.Peer) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "wg_interface.tpl", map[string]any{
		"Interface": cfg,
		"Peers":     peers,
		"Portal": map[string]any{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute interface template for %s: %w", cfg.Identifier, err)
	}

	return &tplBuff, nil
}

// GetPeerConfig returns the rendered configuration file for a WireGuard peer.
func (c TemplateHandler) GetPeerConfig(peer *domain.Peer, style string) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "wg_peer.tpl", map[string]any{
		"Style": style,
		"Peer":  peer,
		"Portal": map[string]any{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute peer template for %s: %w", peer.Identifier, err)
	}

	return &tplBuff, nil
}
