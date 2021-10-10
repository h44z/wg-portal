package wireguard

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type wgCtrlManager struct {
	mux sync.RWMutex // mutex to synchronize access to maps and external api clients

	// external api clients
	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient

	// optional persistent backend
	store store

	// internal holder of interface configurations
	interfaces map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig
	// internal holder of peer configurations
	peers map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig
}

func newWgCtrlManager(wg lowlevel.WireGuardClient, nl lowlevel.NetlinkClient, store store) (*wgCtrlManager, error) {
	m := &wgCtrlManager{
		mux:        sync.RWMutex{},
		wg:         wg,
		nl:         nl,
		store:      store,
		interfaces: make(map[persistence.InterfaceIdentifier]*persistence.InterfaceConfig),
		peers:      make(map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]*persistence.PeerConfig),
	}

	if err := m.initializeFromStore(); err != nil {
		return nil, errors.WithMessage(err, "failed to initialize manager from store")
	}

	return m, nil
}

func (m *wgCtrlManager) GetInterfaces() ([]*persistence.InterfaceConfig, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	interfaces := make([]*persistence.InterfaceConfig, 0, len(m.interfaces))
	for _, iface := range m.interfaces {
		interfaces = append(interfaces, iface)
	}
	// Order the interfaces by device name
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Identifier < interfaces[j].Identifier
	})

	return interfaces, nil
}

func (m *wgCtrlManager) CreateInterface(id persistence.InterfaceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.deviceExists(id) {
		return errors.New("device already exists")
	}

	err := m.createLowLevelInterface(id)
	if err != nil {
		return errors.WithMessage(err, "failed to create low level interface")
	}

	newInterface := &persistence.InterfaceConfig{Identifier: id}
	m.interfaces[id] = newInterface
	m.peers[id] = make(map[persistence.PeerIdentifier]*persistence.PeerConfig)

	err = m.persistInterface(id, false)
	if err != nil {
		return errors.WithMessage(err, "failed to persist created interface")
	}

	return nil
}

func (m *wgCtrlManager) DeleteInterface(id persistence.InterfaceIdentifier) error {
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

	for peerId := range m.peers[id] {
		err = m.persistPeer(peerId, true)
		if err != nil {
			return errors.WithMessagef(err, "failed to persist deleted peer %s", peerId)
		}
	}

	delete(m.interfaces, id)
	delete(m.peers, id)

	return nil
}

func (m *wgCtrlManager) UpdateInterface(id persistence.InterfaceIdentifier, cfg *persistence.InterfaceConfig) error {
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
	if err != nil {
		return errors.WithMessage(err, "failed to parse ip address")
	}
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

func (m *wgCtrlManager) ApplyDefaultConfigs(id persistence.InterfaceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if !m.deviceExists(id) {
		return errors.New("device does not exist")
	}

	cfg := m.interfaces[id]

	for p := range m.peers[id] {
		m.peers[id][p].Endpoint.TrySetValue(cfg.PeerDefEndpoint)
		m.peers[id][p].AllowedIPsStr.TrySetValue(cfg.PeerDefAllowedIPsStr)

		m.peers[id][p].Interface.Identifier = cfg.Identifier
		m.peers[id][p].Interface.Type = cfg.Type
		m.peers[id][p].Interface.PublicKey = cfg.KeyPair.PublicKey

		m.peers[id][p].Interface.DnsStr.TrySetValue(cfg.PeerDefDnsStr)
		m.peers[id][p].Interface.Mtu.TrySetValue(cfg.PeerDefMtu)
		m.peers[id][p].Interface.FirewallMark.TrySetValue(cfg.PeerDefFirewallMark)
		m.peers[id][p].Interface.RoutingTable.TrySetValue(cfg.PeerDefRoutingTable)

		m.peers[id][p].Interface.PreUp.TrySetValue(cfg.PeerDefPreUp)
		m.peers[id][p].Interface.PostUp.TrySetValue(cfg.PeerDefPostUp)
		m.peers[id][p].Interface.PreDown.TrySetValue(cfg.PeerDefPreDown)
		m.peers[id][p].Interface.PostDown.TrySetValue(cfg.PeerDefPostDown)

		err := m.persistPeer(m.peers[id][p].Identifier, false)
		if err != nil {
			return errors.Wrapf(err, "failed to persist peer defaults to %s", m.peers[id][p].Identifier)
		}
	}

	return nil
}

func (m *wgCtrlManager) GetPeers(interfaceId persistence.InterfaceIdentifier) ([]*persistence.PeerConfig, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if !m.deviceExists(interfaceId) {
		return nil, errors.New("device does not exist")
	}

	peers := make([]*persistence.PeerConfig, 0, len(m.peers[interfaceId]))
	for _, config := range m.peers[interfaceId] {
		peers = append(peers, config)
	}

	return peers, nil
}

func (m *wgCtrlManager) SavePeers(peers ...*persistence.PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, peer := range peers {
		deviceId := peer.Interface.Identifier
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

func (m *wgCtrlManager) RemovePeer(id persistence.PeerIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	if !m.peerExists(id) {
		return errors.Errorf("peer does not exist")
	}

	peer, _ := m.getPeer(id)
	deviceId := peer.Interface.Identifier

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

func (m *wgCtrlManager) GetImportableInterfaces() (map[*ImportableInterface][]*persistence.PeerConfig, error) {
	devices, err := m.wg.Devices()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get WireGuard device list")
	}

	m.mux.RLock()
	defer m.mux.RUnlock()

	interfaces := make(map[*ImportableInterface][]*persistence.PeerConfig, len(devices))
	for d, device := range devices {
		if _, exists := m.interfaces[persistence.InterfaceIdentifier(device.Name)]; exists {
			continue // interface already managed, skip
		}

		cfg, err := m.convertWireGuardInterface(devices[d])
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to convert WireGuard interface %s", device.Name)
		}

		interfaces[cfg] = make([]*persistence.PeerConfig, len(device.Peers))

		for p, peer := range device.Peers {
			peerCfg, err := m.convertWireGuardPeer(&device.Peers[p], cfg)
			if err != nil {
				return nil, errors.WithMessagef(err, "failed to convert WireGuard peer %s from %s",
					peer.PublicKey.String(), device.Name)
			}

			interfaces[cfg][p] = peerCfg
		}
	}

	return interfaces, nil
}

func (m *wgCtrlManager) ImportInterface(cfg *ImportableInterface, peers []*persistence.PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	// TODO: implement

	return nil
}

//
// -- Helpers
//

func (m *wgCtrlManager) initializeFromStore() error {
	if m.store == nil {
		return nil // no store, nothing to do
	}

	interfaceIds, err := m.store.GetAvailableInterfaces()
	if err != nil {
		return errors.WithMessage(err, "failed to get available interfaces")
	}

	interfaces, err := m.store.GetAllInterfaces(interfaceIds...)
	if err != nil {
		return errors.WithMessage(err, "failed to get all interfaces")
	}

	for tmpCfg, tmpPeers := range interfaces {
		cfg := tmpCfg
		peers := tmpPeers
		m.interfaces[cfg.Identifier] = &cfg
		if _, ok := m.peers[cfg.Identifier]; !ok {
			m.peers[cfg.Identifier] = make(map[persistence.PeerIdentifier]*persistence.PeerConfig)
		}
		for _, peer := range peers {
			m.peers[cfg.Identifier][peer.Identifier] = &peer
		}
	}

	return nil
}

func (m *wgCtrlManager) createLowLevelInterface(id persistence.InterfaceIdentifier) error {
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

func (m *wgCtrlManager) deviceExists(id persistence.InterfaceIdentifier) bool {
	if _, ok := m.interfaces[id]; ok {
		return true
	}
	return false
}

func (m *wgCtrlManager) persistInterface(id persistence.InterfaceIdentifier, delete bool) error {
	if m.store == nil {
		return nil // nothing to do
	}

	var err error
	if delete {
		err = m.store.DeleteInterface(id)
	} else {
		err = m.store.SaveInterface(*m.interfaces[id])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to persist interface")
	}

	return nil
}

func (m *wgCtrlManager) peerExists(id persistence.PeerIdentifier) bool {
	for _, peers := range m.peers {
		if _, ok := peers[id]; ok {
			return true
		}
	}

	return false
}

func (m *wgCtrlManager) persistPeer(id persistence.PeerIdentifier, delete bool) error {
	if m.store == nil {
		return nil // nothing to do
	}

	var peer *persistence.PeerConfig
	for _, peers := range m.peers {
		if p, ok := peers[id]; ok {
			peer = p
			break
		}
	}

	var err error
	if delete {
		err = m.store.DeletePeer(id, peer.Interface.Identifier)
	} else {
		err = m.store.SavePeer(*peer, peer.Interface.Identifier)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to persist peer %s", id)
	}

	return nil
}

func (m *wgCtrlManager) getPeer(id persistence.PeerIdentifier) (*persistence.PeerConfig, error) {
	for _, peers := range m.peers {
		if _, ok := peers[id]; ok {
			return peers[id], nil
		}
	}

	return nil, errors.New("peer not found")
}

func (m *wgCtrlManager) convertWireGuardInterface(device *wgtypes.Device) (*ImportableInterface, error) {
	cfg := &ImportableInterface{}

	cfg.Identifier = persistence.InterfaceIdentifier(device.Name)
	cfg.FirewallMark = int32(device.FirewallMark)
	cfg.KeyPair = persistence.KeyPair{
		PrivateKey: device.PrivateKey.String(),
		PublicKey:  device.PublicKey.String(),
	}
	cfg.ListenPort = device.ListenPort
	cfg.DriverType = device.Type.String()

	lowLevelInterface, err := m.nl.LinkByName(device.Name)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get low level interface for %s", device.Name)
	}
	cfg.Mtu = lowLevelInterface.Attrs().MTU
	ipAddresses, err := m.nl.AddrList(lowLevelInterface)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get low level addresses for %s", device.Name)
	}
	cfg.AddressStr = ipAddressesToString(ipAddresses)

	return cfg, nil
}

func (m *wgCtrlManager) convertWireGuardPeer(peer *wgtypes.Peer, dev *ImportableInterface) (*persistence.PeerConfig, error) {
	peerCfg := &persistence.PeerConfig{}
	peerCfg.Identifier = persistence.PeerIdentifier(peer.PublicKey.String())
	peerCfg.KeyPair = persistence.KeyPair{
		PublicKey: peer.PublicKey.String(),
	}
	peerCfg.DisplayName = "Autodetected Peer (" + peer.PublicKey.String()[0:8] + ")"
	if peer.Endpoint != nil {
		peerCfg.Endpoint = persistence.NewStringConfigOption(peer.Endpoint.String(), true)
	}
	if peer.PresharedKey != (wgtypes.Key{}) {
		peerCfg.PresharedKey = peer.PresharedKey.String()
	}
	allowedIPs := make([]string, len(peer.AllowedIPs)) // use allowed IP's as the peer IP's
	for i, ip := range peer.AllowedIPs {
		allowedIPs[i] = ip.String()
	}
	peerCfg.AllowedIPsStr = persistence.NewStringConfigOption(strings.Join(allowedIPs, ","), true)
	peerCfg.PersistentKeepalive = persistence.NewIntConfigOption(int(peer.PersistentKeepaliveInterval.Seconds()), true)

	peerCfg.Interface = &persistence.PeerInterfaceConfig{
		Identifier: dev.Identifier,
		AddressStr: persistence.NewStringConfigOption(dev.AddressStr, true), // todo: correct?
		DnsStr:     persistence.NewStringConfigOption(dev.DnsStr, true),
		Mtu:        persistence.NewIntConfigOption(dev.Mtu, true),
	}

	return peerCfg, nil
}

func getWireGuardPeerConfig(devType persistence.InterfaceType, cfg *persistence.PeerConfig) (wgtypes.PeerConfig, error) {
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
	if cfg.PersistentKeepalive.GetValue() != 0 {
		keepAliveDuration := time.Duration(cfg.PersistentKeepalive.GetValue()) * time.Second
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
