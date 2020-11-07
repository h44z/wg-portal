package wireguard

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Manager struct {
	Cfg *Config
	wg  *wgctrl.Client
	mux sync.RWMutex
}

func (m *Manager) Init() error {
	var err error
	m.wg, err = wgctrl.New()
	if err != nil {
		return fmt.Errorf("could not create WireGuard client: %w", err)
	}

	return nil
}

func (m *Manager) GetDeviceInfo() (*wgtypes.Device, error) {
	dev, err := m.wg.Device(m.Cfg.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("could not get WireGuard device: %w", err)
	}

	return dev, nil
}

func (m *Manager) GetPeerList() ([]wgtypes.Peer, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	dev, err := m.wg.Device(m.Cfg.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("could not get WireGuard device: %w", err)
	}

	return dev.Peers, nil
}

func (m *Manager) GetPeer(pubKey string) (*wgtypes.Peer, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	publicKey, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	peers, err := m.GetPeerList()
	if err != nil {
		return nil, fmt.Errorf("could not get WireGuard peers: %w", err)
	}

	for _, peer := range peers {
		if peer.PublicKey == publicKey {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("could not find WireGuard peer: %s", pubKey)
}

func (m *Manager) AddPeer(cfg wgtypes.PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	err := m.wg.ConfigureDevice(m.Cfg.DeviceName, wgtypes.Config{Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return fmt.Errorf("could not configure WireGuard device: %w", err)
	}

	return nil
}

func (m *Manager) UpdatePeer(cfg wgtypes.PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	cfg.UpdateOnly = true
	err := m.wg.ConfigureDevice(m.Cfg.DeviceName, wgtypes.Config{Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return fmt.Errorf("could not configure WireGuard device: %w", err)
	}

	return nil
}

func (m *Manager) RemovePeer(pubKey string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	publicKey, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	peer := wgtypes.PeerConfig{
		PublicKey: publicKey,
		Remove:    true,
	}

	err = m.wg.ConfigureDevice(m.Cfg.DeviceName, wgtypes.Config{Peers: []wgtypes.PeerConfig{peer}})
	if err != nil {
		return fmt.Errorf("could not configure WireGuard device: %w", err)
	}

	return nil
}

func (m *Manager) UpdateDevice(name string, cfg wgtypes.Config) error {
	return m.wg.ConfigureDevice(name, cfg)
}
