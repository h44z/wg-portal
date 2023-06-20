package filetemplate

import (
	"context"
	"fmt"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
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
