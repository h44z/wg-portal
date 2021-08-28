package wireguard

import (
	"bytes"
	"embed"
	"io"
	"text/template"

	"github.com/pkg/errors"
)

//go:embed tpl_files/*
var TemplateFiles embed.FS

type ConfigFileGenerator interface {
	GetInterfaceConfig(cfg InterfaceConfig, peers []PeerConfig) (io.Reader, error)
	GetPeerConfig(peer PeerConfig, iface InterfaceConfig) (io.Reader, error)
}

type ConfigFileParser interface {
	ParseConfig(fileContents io.Reader) (InterfaceConfig, []PeerConfig, error)
}

type TemplateHandler struct {
	templates *template.Template
}

func NewTemplateHandler() (*TemplateHandler, error) {
	templateCache, err := template.New("WireGuard").ParseFS(TemplateFiles, "tpl_files/*.tpl")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse template files")
	}

	handler := &TemplateHandler{
		templates: templateCache,
	}

	return handler, nil
}

func (c TemplateHandler) GetInterfaceConfig(cfg InterfaceConfig, peers []PeerConfig) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "interface.tpl", map[string]interface{}{
		"Interface": cfg,
		"Peers":     peers,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute interface template for %s", cfg.DeviceName)
	}

	return &tplBuff, nil
}

func (c TemplateHandler) GetPeerConfig(peer PeerConfig, iface InterfaceConfig) (io.Reader, error) {
	var tplBuff bytes.Buffer

	err := c.templates.ExecuteTemplate(&tplBuff, "peer.tpl", map[string]interface{}{
		"Peer":      peer,
		"Interface": iface,
		"Portal": map[string]interface{}{
			"Version": "unknown",
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute peer template for %s", peer.Uid)
	}

	return &tplBuff, nil
}
