package wireguard

import (
	"bytes"
	"embed"
	"io"
	"text/template"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

type templateHandler struct {
	templates *template.Template
}

func newTemplateHandler() (*templateHandler, error) {
	templateCache, err := template.New("WireGuard").ParseFS(TemplateFiles, "tpl_files/*.tpl")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse template files")
	}

	handler := &templateHandler{
		templates: templateCache,
	}

	return handler, nil
}

func (c templateHandler) GetInterfaceConfig(cfg *persistence.InterfaceConfig, peers []*persistence.PeerConfig) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "interface.tpl", map[string]interface{}{
		"Interface": cfg,
		"Peers":     peers,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute interface template for %s", cfg.Identifier)
	}

	return &tplBuff, nil
}

func (c templateHandler) GetPeerConfig(peer *persistence.PeerConfig) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "peer.tpl", map[string]interface{}{
		"Peer": peer,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute peer template for %s", peer.Identifier)
	}

	return &tplBuff, nil
}
