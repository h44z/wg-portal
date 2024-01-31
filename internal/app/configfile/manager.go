package configfile

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
	"github.com/yeqown/go-qrcode/v2"
	"io"
	"os"
	"strings"
)

type Manager struct {
	cfg        *config.Config
	bus        evbus.MessageBus
	tplHandler *TemplateHandler

	fsRepo FileSystemRepo // can be nil if storing the configuration is disabled
	users  UserDatabaseRepo
	wg     WireguardDatabaseRepo
}

func NewConfigFileManager(cfg *config.Config, bus evbus.MessageBus, users UserDatabaseRepo, wg WireguardDatabaseRepo, fsRepo FileSystemRepo) (*Manager, error) {
	tplHandler, err := newTemplateHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template handler: %w", err)
	}

	m := &Manager{
		cfg:        cfg,
		bus:        bus,
		tplHandler: tplHandler,

		fsRepo: fsRepo,
		users:  users,
		wg:     wg,
	}

	if err := m.createStorageDirectory(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m Manager) createStorageDirectory() error {
	if m.cfg.Advanced.ConfigStoragePath == "" {
		return nil // no storage path configured, skip initialization step
	}

	err := os.MkdirAll(m.cfg.Advanced.ConfigStoragePath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create configuration storage path %s: %w",
			m.cfg.Advanced.ConfigStoragePath, err)
	}

	return nil
}

func (m Manager) connectToMessageBus() {
	if m.fsRepo == nil {
		return // skip subscription
	}

	_ = m.bus.Subscribe(app.TopicInterfaceUpdated, m.handleInterfaceUpdatedEvent)
	_ = m.bus.Subscribe(app.TopicPeerInterfaceUpdated, m.handleInterfaceUpdatedEvent)
}

func (m Manager) handleInterfaceUpdatedEvent(iface *domain.Interface) {
	logrus.Errorf("handling interface updated event for %s", iface.Identifier)

	if !iface.SaveConfig || m.fsRepo == nil {
		return
	}

	err := m.PersistInterfaceConfig(context.Background(), iface.Identifier)
	if err != nil {
		logrus.Errorf("failed to automatically persist interface config for %s: %v", iface.Identifier, err)
	}
}

func (m Manager) handlePeerInterfaceUpdatedEvent(id domain.InterfaceIdentifier) {
	logrus.Errorf("handling interface updated event for %s", id)

	if m.fsRepo == nil {
		return
	}

	peerInterface, err := m.wg.GetInterface(context.Background(), id)
	if err != nil {
		logrus.Errorf("failed to load interface %s: %v", id, err)
		return
	}

	if !peerInterface.SaveConfig {
		return
	}

	err = m.PersistInterfaceConfig(context.Background(), peerInterface.Identifier)
	if err != nil {
		logrus.Errorf("failed to automatically persist interface config for %s: %v", peerInterface.Identifier, err)
	}
}

func (m Manager) GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

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

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	return m.tplHandler.GetPeerConfig(peer)
}

func (m Manager) GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error) {
	peer, err := m.wg.GetPeer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peer %s: %w", id, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	cfgData, err := m.tplHandler.GetPeerConfig(peer)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer config for %s: %w", id, err)
	}

	// remove comments from qr-code config as it is not needed
	sb := strings.Builder{}
	scanner := bufio.NewScanner(cfgData)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read peer config for %s: %w", id, err)
	}

	code, err := qrcode.New(sb.String())
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

func (m Manager) PersistInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	if m.fsRepo == nil {
		return fmt.Errorf("peristing configuration is not supported")
	}

	iface, peers, err := m.wg.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to fetch interface %s: %w", id, err)
	}

	cfg, err := m.tplHandler.GetInterfaceConfig(iface, peers)
	if err != nil {
		return fmt.Errorf("failed to get interface config: %w", err)
	}

	if err := m.fsRepo.WriteFile(iface.GetConfigFileName(), cfg); err != nil {
		return fmt.Errorf("failed to write interface config: %w", err)
	}

	return nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }
