package configfile

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"text/template"

	"github.com/h44z/wg-portal/internal/domain"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

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

func (c TemplateHandler) GetInterfaceConfig(cfg *domain.Interface, peers []domain.Peer) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "wg_interface.tpl", map[string]interface{}{
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

func (c TemplateHandler) GetPeerConfig(peer *domain.Peer) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "wg_peer.tpl", map[string]interface{}{
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
