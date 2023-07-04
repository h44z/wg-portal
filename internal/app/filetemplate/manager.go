package filetemplate

import (
	"bytes"
	"context"
	"fmt"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/yeqown/go-qrcode/v2"
	"io"
)

type Manager struct {
	cfg        *config.Config
	tplHandler *TemplateHandler

	users UserDatabaseRepo
	wg    WireguardDatabaseRepo
}

func NewTemplateManager(cfg *config.Config, users UserDatabaseRepo, wg WireguardDatabaseRepo) (*Manager, error) {
	tplHandler, err := newTemplateHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template handler: %w", err)
	}

	m := &Manager{
		cfg:        cfg,
		tplHandler: tplHandler,

		users: users,
		wg:    wg,
	}

	return m, nil
}

func (m Manager) GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error) {
	iface, peers, err := m.wg.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch interface %s: %w", id, err)
	}

	return m.tplHandler.GetInterfaceConfig(iface, peers)
}

func (m Manager) GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error) {
	peer, err := m.wg.GetPeer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peer %s: %w", id, err)
	}

	return m.tplHandler.GetPeerConfig(peer)
}

func (m Manager) GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error) {
	peer, err := m.wg.GetPeer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peer %s: %w", id, err)
	}

	cfgData, err := m.tplHandler.GetPeerConfig(peer)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer config for %s: %w", id, err)
	}

	configBytes, err := io.ReadAll(cfgData)
	if err != nil {
		return nil, fmt.Errorf("failed to read peer config for %s: %w", id, err)
	}

	code, err := qrcode.New(string(configBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to initializeqr code for %s: %w", id, err)
	}

	buf := bytes.NewBuffer(nil)
	wr := nopCloser{Writer: buf}
	option := Option{
		Padding:   8, // padding pixels around the qr code.
		BlockSize: 4, // block pixels which represents a bit data.
	}
	qrWriter := NewCompressedWriter(wr, &option)
	err = code.Save(qrWriter)
	if err != nil {
		return nil, fmt.Errorf("failed to write code for %s: %w", id, err)
	}

	return buf, nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }
