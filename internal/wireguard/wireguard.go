package wireguard

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/vishvananda/netlink"

	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
)

type WgCtrlManager struct {
	mux sync.RWMutex // mutex to synchronize access to maps and external api clients

	// external api clients
	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient

	// optional persistent backend
	store store

	// internal holder of interface configurations
	interfaces map[persistence.InterfaceIdentifier]persistence.InterfaceConfig
	// internal holder of peer configurations
	peers map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]persistence.PeerConfig
}

func (m *WgCtrlManager) GetInterfaces() ([]persistence.InterfaceConfig, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	interfaces := make([]persistence.InterfaceConfig, 0, len(m.interfaces))
	for _, iface := range interfaces {
		interfaces = append(interfaces, iface)
	}
	// Order the interfaces by device name
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Identifier < interfaces[j].Identifier
	})

	return interfaces, nil
}

func (m *WgCtrlManager) CreateInterface(id persistence.InterfaceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.deviceExists(id) {
		return errors.New("device already exists")
	}

	err := m.createLowLevelInterface(id)
	if err != nil {
		return errors.WithMessage(err, "failed to create low level interface")
	}

	newInterface := persistence.InterfaceConfig{Identifier: id}
	m.interfaces[id] = newInterface

	err = m.persistInterface(id, false)
	if err != nil {
		return errors.WithMessage(err, "failed to persist created interface")
	}

	return nil
}

func (m *WgCtrlManager) DeleteInterface(id persistence.InterfaceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if !m.deviceExists(id) {
		return errors.New("interface does not exist")
	}

	err := m.nl.LinkDel(&netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(id),
		},
		LinkType: "wireguard",
	})
	if err != nil {
		return errors.WithMessage(err, "failed to delete low level interface")
	}

	err = m.persistInterface(id, true)
	if err != nil {
		return errors.WithMessage(err, "failed to persist deleted interface")
	}

	delete(m.interfaces, id)

	return nil
}

func (m *WgCtrlManager) UpdateInterface(id persistence.InterfaceIdentifier, cfg persistence.InterfaceConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if !m.deviceExists(id) {
		return errors.New("interface does not exist")
	}
	cfg.Identifier = id // ensure that the same device name is set

	// Update net-link attributes
	link, err := m.nl.LinkByName(string(id))
	if err != nil {
		return errors.WithMessage(err, "failed to open low level interface")
	}
	if err := m.nl.LinkSetMTU(link, cfg.Mtu); err != nil {
		return errors.WithMessage(err, "failed to set MTU")
	}
	addresses, err := parseIpAddressString(cfg.AddressStr)
	for i := 0; i < len(addresses); i++ {
		var err error
		if i == 0 {
			err = m.nl.AddrReplace(link, addresses[i])
		} else {
			err = m.nl.AddrAdd(link, addresses[i])
		}
		if err != nil {
			return errors.WithMessage(err, "failed to set ip address")
		}
	}

	// Update WireGuard attributes
	pKey, err := wgtypes.NewKey(GetPrivateKeyBytes(cfg.KeyPair))
	if err != nil {
		return errors.WithMessage(err, "failed to parse private key bytes")
	}

	var fwMark *int
	if cfg.FirewallMark != 0 {
		*fwMark = int(cfg.FirewallMark)
	}
	err = m.wg.ConfigureDevice(string(id), wgtypes.Config{
		PrivateKey:   &pKey,
		ListenPort:   &cfg.ListenPort,
		FirewallMark: fwMark,
	})
	if err != nil {
		return errors.WithMessage(err, "failed to update WireGuard settings")
	}

	// Update link state
	if cfg.Enabled {
		if err := m.nl.LinkSetUp(link); err != nil {
			return errors.WithMessage(err, "failed to enable low level interface")
		}
	} else {
		if err := m.nl.LinkSetDown(link); err != nil {
			return errors.WithMessage(err, "failed to disable low level interface")
		}
	}

	// update internal map
	m.interfaces[id] = cfg

	err = m.persistInterface(id, false)
	if err != nil {
		return errors.WithMessage(err, "failed to persist updated interface")
	}

	return nil
}

func (m *WgCtrlManager) GetPeers(interfaceId persistence.InterfaceIdentifier) ([]persistence.PeerConfig, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if !m.deviceExists(interfaceId) {
		return nil, errors.New("device does not exist")
	}

	peers := make([]persistence.PeerConfig, 0, len(m.peers[interfaceId]))
	for _, config := range m.peers[interfaceId] {
		peers = append(peers, config)
	}

	return peers, nil
}

func (m *WgCtrlManager) SavePeers(peers ...persistence.PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, peer := range peers {
		deviceId := peer.PeerInterfaceConfig.Identifier
		if !m.deviceExists(deviceId) {
			return errors.Errorf("device does not exist")
		}
		deviceConfig := m.interfaces[deviceId]

		wgPeer, err := getWireGuardPeerConfig(deviceConfig.Type, peer)
		if err != nil {
			return errors.WithMessagef(err, "could not generate WireGuard peer configuration for %s", peer.Identifier)
		}

		err = m.wg.ConfigureDevice(string(deviceId), wgtypes.Config{Peers: []wgtypes.PeerConfig{wgPeer}})
		if err != nil {
			return errors.Wrapf(err, "could not save peer %s to WireGuard device %s", peer.Identifier, deviceId)
		}

		m.peers[deviceId][peer.Identifier] = peer

		err = m.persistPeer(peer.Identifier, false)
		if err != nil {
			return errors.Wrapf(err, "failed to persist updated peer %s", peer.Identifier)
		}
	}

	return nil
}

func (m *WgCtrlManager) RemovePeer(id persistence.PeerIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if !m.peerExists(id) {
		return errors.Errorf("peer does not exist")
	}

	peer, _ := m.getPeer(id)
	deviceId := peer.PeerInterfaceConfig.Identifier

	publicKey, err := wgtypes.ParseKey(peer.KeyPair.PublicKey)
	if err != nil {
		return errors.WithMessage(err, "invalid public key")
	}

	wgPeer := wgtypes.PeerConfig{
		PublicKey: publicKey,
		Remove:    true,
	}

	err = m.wg.ConfigureDevice(string(deviceId), wgtypes.Config{Peers: []wgtypes.PeerConfig{wgPeer}})
	if err != nil {
		return errors.WithMessage(err, "could not remove peer from WireGuard interface")
	}

	err = m.persistPeer(id, true)
	if err != nil {
		return errors.WithMessage(err, "failed to persist deleted peer")
	}

	delete(m.peers[deviceId], id)

	return nil
}

//
// -- Helpers
//

func (m *WgCtrlManager) createLowLevelInterface(id persistence.InterfaceIdentifier) error {
	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(id),
		},
		LinkType: "wireguard",
	}
	err := m.nl.LinkAdd(link)
	if err != nil {
		return errors.Wrapf(err, "failed to create netlink interface")
	}

	if err := m.nl.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "failed to enable netlink interface")
	}

	return nil
}

func (m *WgCtrlManager) deviceExists(id persistence.InterfaceIdentifier) bool {
	if _, ok := m.interfaces[id]; ok {
		return true
	}
	return false
}

func (m *WgCtrlManager) persistInterface(id persistence.InterfaceIdentifier, delete bool) error {
	if m.store == nil {
		return nil // nothing to do
	}

	device := m.interfaces[id]
	peers := make([]persistence.PeerConfig, 0, len(m.peers[id]))
	for _, config := range m.peers[id] {
		peers = append(peers, config)
	}

	var err error
	if delete {
		err = m.store.DeleteInterface(id)
	} else {
		err = m.store.SaveInterface(device, peers)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to persist interface")
	}

	return nil
}

func (m *WgCtrlManager) peerExists(id persistence.PeerIdentifier) bool {
	for _, peers := range m.peers {
		if _, ok := peers[id]; ok {
			return true
		}
	}

	return false
}

func (m *WgCtrlManager) persistPeer(id persistence.PeerIdentifier, delete bool) error {
	if m.store == nil {
		return nil // nothing to do
	}

	var peer persistence.PeerConfig
	for _, peers := range m.peers {
		if p, ok := peers[id]; ok {
			peer = p
			break
		}
	}

	var err error
	if delete {
		err = m.store.DeletePeer(id, peer.PeerInterfaceConfig.Identifier)
	} else {
		err = m.store.SavePeer(peer, peer.PeerInterfaceConfig.Identifier)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to persist peer %s", id)
	}

	return nil
}

func (m *WgCtrlManager) getPeer(id persistence.PeerIdentifier) (persistence.PeerConfig, error) {
	for _, peers := range m.peers {
		if _, ok := peers[id]; ok {
			return peers[id], nil
		}
	}

	return persistence.PeerConfig{}, errors.New("peer not found")
}

func parseIpAddressString(addrStr string) ([]*netlink.Addr, error) {
	rawAddresses := strings.Split(addrStr, ",")
	addresses := make([]*netlink.Addr, 0, len(rawAddresses))
	for i := range rawAddresses {
		rawAddress := strings.TrimSpace(rawAddresses[i])
		if rawAddress == "" {
			continue // skip empty string
		}
		address, err := netlink.ParseAddr(rawAddress)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse IP address %s", rawAddress)
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

func getWireGuardPeerConfig(devType persistence.InterfaceType, cfg persistence.PeerConfig) (wgtypes.PeerConfig, error) {
	publicKey, err := wgtypes.ParseKey(cfg.KeyPair.PublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, errors.WithMessage(err, "invalid public key for peer")
	}

	var presharedKey *wgtypes.Key
	if tmpPresharedKey, err := wgtypes.ParseKey(cfg.PresharedKey); err == nil {
		presharedKey = &tmpPresharedKey
	}

	var endpoint *net.UDPAddr
	if cfg.Endpoint.Value != "" && devType == persistence.InterfaceTypeClient {
		addr, err := net.ResolveUDPAddr("udp", cfg.Endpoint.Value.(string))
		if err == nil {
			endpoint = addr
		}
	}

	var keepAlive *time.Duration
	if cfg.PersistentKeepalive.Value != 0 {
		keepAliveDuration := time.Duration(cfg.PersistentKeepalive.Value.(int)) * time.Second
		keepAlive = &keepAliveDuration
	}

	allowedIPs := make([]net.IPNet, 0)
	var peerAllowedIPs []*netlink.Addr
	switch devType {
	case persistence.InterfaceTypeClient:
		peerAllowedIPs, err = parseIpAddressString(cfg.AllowedIPsStr.GetValue())
		if err != nil {
			return wgtypes.PeerConfig{}, errors.WithMessage(err, "failed to parse allowed IP's")
		}
	case persistence.InterfaceTypeServer:
		peerAllowedIPs, err = parseIpAddressString(cfg.AllowedIPsStr.GetValue())
		if err != nil {
			return wgtypes.PeerConfig{}, errors.WithMessage(err, "failed to parse allowed IP's")
		}
		peerExtraAllowedIPs, err := parseIpAddressString(cfg.ExtraAllowedIPsStr)
		if err != nil {
			return wgtypes.PeerConfig{}, errors.WithMessage(err, "failed to parse extra allowed IP's")
		}

		peerAllowedIPs = append(peerAllowedIPs, peerExtraAllowedIPs...)
	}
	for _, ip := range peerAllowedIPs {
		allowedIPs = append(allowedIPs, *ip.IPNet)
	}

	wgPeer := wgtypes.PeerConfig{
		PublicKey:                   publicKey,
		Remove:                      false,
		UpdateOnly:                  true,
		PresharedKey:                presharedKey,
		Endpoint:                    endpoint,
		PersistentKeepaliveInterval: keepAlive,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  allowedIPs,
	}

	return wgPeer, nil
}
