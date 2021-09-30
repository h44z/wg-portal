package wireguard

import (
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/lowlevel"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type KeyGenerator interface {
	GetFreshKeypair() (KeyPair, error)
	GetPreSharedKey() (PreSharedKey, error)
}

// DeviceManager provides methods to create/update/delete physical WireGuard devices.
type DeviceManager interface {
	GetDevices() ([]InterfaceConfig, error)
	CreateDevice(device DeviceIdentifier) error
	DeleteDevice(device DeviceIdentifier) error
	UpdateDevice(device DeviceIdentifier, cfg InterfaceConfig) error
}

type PeerManager interface {
	GetPeers(device DeviceIdentifier) ([]PeerConfig, error)
	SavePeers(device DeviceIdentifier, peers ...PeerConfig) error
	RemovePeer(device DeviceIdentifier, peer PeerIdentifier) error
}

type Opt func(svc *ManagementUtil)

type Manager interface {
	KeyGenerator
	DeviceManager
	PeerManager
}

// ManagementUtil is a persistent management util for WireGuard configurations.
type ManagementUtil struct {
	mux sync.RWMutex // mutex to synchronize access to maps

	wg lowlevel.WireGuardClient // WireGuard interface handler
	nl lowlevel.NetlinkClient   // Network interface handler
	cs ConfigStore              // Persistent backend

	// internal holder of interface configurations
	interfaces map[DeviceIdentifier]InterfaceConfig

	// internal holder of peer configurations
	peers map[DeviceIdentifier]map[PeerIdentifier]PeerConfig
}

func NewManagementUtil(wg lowlevel.WireGuardClient, nl lowlevel.NetlinkClient, cs ConfigStore, opts ...Opt) (*ManagementUtil, error) {
	m := &ManagementUtil{
		mux: sync.RWMutex{},
		wg:  wg,
		nl:  nl,
		cs:  cs,
	}

	for _, opt := range opts {
		opt(m)
	}

	// initialize
	err := m.initialize()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize WireGuard manager")
	}

	return m, nil
}

func IgnoredInterfaces(ignored ...DeviceIdentifier) Opt {
	return func(m *ManagementUtil) {
		m.unmanagedInterfaces = ignored
	}
}

func (m *ManagementUtil) GetFreshKeypair() (KeyPair, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return KeyPair{}, errors.Wrap(err, "failed to generate private Key")
	}

	return KeyPair{
		PrivateKey: privateKey.String(),
		PublicKey:  privateKey.PublicKey().String(),
	}, nil
}

func (m *ManagementUtil) GetPreSharedKey() (PreSharedKey, error) {
	preSharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate pre-shared Key")
	}

	return PreSharedKey(preSharedKey.String()), nil
}

func (m *ManagementUtil) GetDevices() ([]InterfaceConfig, error) {
	interfaces := make([]InterfaceConfig, 0, len(m.interfaces))
	for _, iface := range interfaces {
		interfaces = append(interfaces, iface)
	}
	// Order the interfaces by device name
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].DeviceName < interfaces[j].DeviceName
	})

	return interfaces, nil
}

func (m *ManagementUtil) CreateDevice(identifier DeviceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.deviceExists(identifier) {
		return errors.Errorf("device %s already exists", identifier)
	}

	err := m.createWgDevice(identifier)
	if err != nil {
		return errors.Wrapf(err, "failed to create WireGuard interface %s", identifier)
	}

	newInterface := InterfaceConfig{DeviceName: identifier}
	m.interfaces[identifier] = newInterface

	err = m.persistInterface(identifier, false)
	if err != nil {
		return errors.Wrapf(err, "failed to persist created interface %s", identifier)
	}

	return nil
}

func (m *ManagementUtil) createWgDevice(identifier DeviceIdentifier) error {
	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(identifier),
		},
		LinkType: "wireguard",
	}
	err := m.nl.LinkAdd(link)
	if err != nil {
		return errors.Wrapf(err, "failed to create WireGuard interface %s", identifier)
	}

	if err := m.nl.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "failed to enable WireGuard interface %s", identifier)
	}

	return nil
}

func (m *ManagementUtil) DeleteDevice(identifier DeviceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if !m.deviceExists(identifier) {
		return errors.Errorf("device %s does not exist", identifier)
	}
	err := m.nl.LinkDel(&netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(identifier),
		},
		LinkType: "wireguard",
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete WireGuard interface")
	}

	err = m.persistInterface(identifier, true)
	if err != nil {
		return errors.Wrapf(err, "failed to persist deleted interface %s", identifier)
	}

	delete(m.interfaces, identifier)

	return nil
}

func (m *ManagementUtil) UpdateDevice(identifier DeviceIdentifier, cfg InterfaceConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if !m.deviceExists(identifier) {
		return errors.Errorf("device %s does not exist", identifier)
	}
	cfg.DeviceName = identifier // ensure that the same device name is set

	// Update net-link attributes
	link, err := m.nl.LinkByName(string(identifier))
	if err != nil {
		return errors.Wrapf(err, "failed to open WireGuard interface")
	}
	if err := m.nl.LinkSetMTU(link, cfg.Mtu); err != nil {
		return errors.Wrapf(err, "failed to set MTU")
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
			return errors.Wrapf(err, "failed to set ip address %v", addresses[i])
		}
	}

	// Update WireGuard attributes
	pKey, _ := wgtypes.NewKey(cfg.KeyPair.GetPrivateKeyBytes())
	var fwMark *int
	if cfg.FirewallMark != 0 {
		*fwMark = int(cfg.FirewallMark)
	}
	err = m.wg.ConfigureDevice(string(identifier), wgtypes.Config{
		PrivateKey:   &pKey,
		ListenPort:   &cfg.ListenPort,
		FirewallMark: fwMark,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to update WireGuard settings")
	}

	// Update link state
	if cfg.Enabled {
		if err := m.nl.LinkSetUp(link); err != nil {
			return errors.Wrapf(err, "failed to enable WireGuard interface")
		}
	} else {
		if err := m.nl.LinkSetDown(link); err != nil {
			return errors.Wrapf(err, "failed to disable WireGuard interface")
		}
	}

	m.interfaces[identifier] = cfg

	err = m.persistInterface(identifier, false)
	if err != nil {
		return errors.Wrapf(err, "failed to persist updated interface %s", identifier)
	}

	return nil
}

func (m *ManagementUtil) GetPeers(device DeviceIdentifier) ([]PeerConfig, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if !m.deviceExists(device) {
		return nil, errors.Errorf("device %s does not exist", device)
	}

	peers := make([]PeerConfig, 0, len(m.peers[device]))
	for _, config := range m.peers[device] {
		peers = append(peers, config)
	}

	return peers, nil
}

func (m *ManagementUtil) SavePeers(device DeviceIdentifier, peers ...PeerConfig) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if !m.deviceExists(device) {
		return errors.Errorf("device %s does not exist", device)
	}

	deviceConfig := m.interfaces[device]

	for _, peer := range peers {
		wgPeer, err := getWireGuardPeerConfig(deviceConfig.Type, peer)
		if err != nil {
			return errors.Wrapf(err, "could not generate WireGuard peer configuration for %s", peer.Uid)
		}

		err = m.wg.ConfigureDevice(string(device), wgtypes.Config{Peers: []wgtypes.PeerConfig{wgPeer}})
		if err != nil {
			return errors.Wrapf(err, "could not save peer %s to WireGuard device %s", peer.Uid, device)
		}

		m.peers[device][peer.Uid] = peer

		err = m.persistPeer(peer.Uid, false)
		if err != nil {
			return errors.Wrapf(err, "failed to persist updated peer %s", peer.Uid)
		}
	}

	return nil
}

func (m *ManagementUtil) RemovePeer(device DeviceIdentifier, peer PeerIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if !m.deviceExists(device) {
		return errors.Errorf("device %s does not exist", device)
	}
	if !m.peerExists(peer) {
		return errors.Errorf("peer %s does not exist", peer)
	}

	peerConfig := m.peers[device][peer]

	publicKey, err := wgtypes.ParseKey(peerConfig.KeyPair.PublicKey)
	if err != nil {
		return errors.Wrapf(err, "invalid public key for peer %s", peer)
	}

	wgPeer := wgtypes.PeerConfig{
		PublicKey: publicKey,
		Remove:    true,
	}

	err = m.wg.ConfigureDevice(string(device), wgtypes.Config{Peers: []wgtypes.PeerConfig{wgPeer}})
	if err != nil {
		return errors.Wrapf(err, "could not remove peer %s from WireGuard device %s", peer, device)
	}

	err = m.persistPeer(peer, true)
	if err != nil {
		return errors.Wrapf(err, "failed to persist deleted peer %s", peer)
	}

	delete(m.peers[device], peer)

	return nil
}

// TODO: implement/think about
func (m *ManagementUtil) loadFromBackend() error {
	// Load all interfaces from the database
	backendInterfaces, err := m.cs.GetAvailableInterfaces()
	if err != nil {
		return errors.Wrap(err, "failed to load backend interfaces")
	}

	/*// Get a list of available WireGuard interfaces
	wgInterfaces, err := m.wg.Devices()
	if err != nil {
		return errors.Wrap(err, "failed to load WireGuard interfaces")
	}

	// Create missing WireGuard interfaces
	for _, backendInterface := range backendInterfaces {
		exists := false
		for _, wgInterface := range wgInterfaces {
			if string(backendInterface) == wgInterface.Name {
				exists = true
				break
			}
		}
		if !exists {
			err := m.createWgDevice(backendInterface)
			if err != nil {
				return errors.Wrapf(err, "failed to create WireGuard interface %s found in backend", backendInterface)
			}
		}
	}*/

	// Load config options from database backend, populate internal state maps
	err = m.loadBackendInterfaces(DatabaseBackendName, backendInterfaces...)
	if err != nil {
		return errors.Wrap(err, "failed to load interface configurations from backend")
	}

	// Load missing config options from current interfaces, populate internal state maps
	err = m.loadWireGuardInterfaces()
	if err != nil {
		return errors.Wrap(err, "failed to load interface configurations from WireGuard")
	}

	// Persists currently loaded configurations
	// TODO

	// Apply configuration options from internal state maps to current interfaces
	// TODO

	return nil
}

func (m *ManagementUtil) loadBackendInterfaces(backend string, identifiers ...DeviceIdentifier) error {
	for _, cl := range m.cl {
		if cl.Name() != backend {
			continue
		}
		ifaceAndPeers, err := cl.LoadAll(identifiers...)
		if err != nil {
			return errors.Wrapf(err, "failed to load interfaces from backend %s", cl.Name())
		}

		for iface, peers := range ifaceAndPeers {
			m.interfaces[iface.DeviceName] = iface
			for _, peer := range peers {
				m.peers[iface.DeviceName][peer.Uid] = peer
			}
		}
	}
	return nil
}

func (m *ManagementUtil) loadWireGuardInterfaces() error {
	// Get a list of available WireGuard interfaces
	wgInterfaces, err := m.wg.Devices()
	if err != nil {
		return errors.Wrap(err, "failed to load WireGuard interfaces")
	}

	for _, iface := range wgInterfaces {
		if m.interfaceIsIgnored(DeviceIdentifier(iface.Name)) {
			continue
		}

		devId := DeviceIdentifier(iface.Name)
		if _, existing := m.interfaces[devId]; !existing {
			m.interfaces[devId] = m.convertWireGuardInterface(*iface)
		}

		for _, peer := range iface.Peers {
			peerPublicKey := peer.PublicKey.String()

			// check if peer exists, compare public keys
			existing := false
			for _, existingPeer := range m.peers[devId] {
				if existingPeer.KeyPair.PublicKey == peerPublicKey {
					existing = true
					break
				}
			}

			if !existing {
				// Use the peers public key as UID
				m.peers[devId][PeerIdentifier(peerPublicKey)] = m.convertWireGuardPeer(peer)
			}

		}
	}
	return nil
}

func (m *ManagementUtil) restoreBackendInterfaces() error {
	return nil
}

func (m *ManagementUtil) interfaceIsIgnored(name DeviceIdentifier) bool {
	for _, iface := range m.unmanagedInterfaces {
		if iface == name {
			return true
		}
	}
	return false
}

//
// ---- Helpers
//

func getWireGuardPeerConfig(deviceType InterfaceType, peer PeerConfig) (wgtypes.PeerConfig, error) {
	publicKey, err := wgtypes.ParseKey(peer.KeyPair.PublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, errors.Wrapf(err, "invalid public key for peer %s", peer.Uid)
	}

	var presharedKey *wgtypes.Key
	if tmpPresharedKey, err := wgtypes.ParseKey(peer.PresharedKey); err == nil {
		presharedKey = &tmpPresharedKey
	}

	var endpoint *net.UDPAddr
	if peer.Endpoint.Value != "" && deviceType == InterfaceTypeClient {
		addr, err := net.ResolveUDPAddr("udp", peer.Endpoint.Value.(string))
		if err == nil {
			endpoint = addr
		}
	}

	var keepAlive *time.Duration
	if peer.PersistentKeepalive.Value != 0 {
		keepAliveDuration := time.Duration(peer.PersistentKeepalive.Value.(int)) * time.Second
		keepAlive = &keepAliveDuration
	}

	allowedIPs := make([]net.IPNet, 0)
	var peerAllowedIPs []*netlink.Addr
	switch deviceType {
	case InterfaceTypeClient:
		peerAllowedIPs, err = parseIpAddressString(peer.AllowedIPsStr.GetValue())
		if err != nil {
			return wgtypes.PeerConfig{}, errors.Wrapf(err, "failed to parse allowed IP's for peer %s", peer.Uid)
		}
	case InterfaceTypeServer:
		peerAllowedIPs, err = parseIpAddressString(peer.AllowedIPsStr.GetValue())
		if err != nil {
			return wgtypes.PeerConfig{}, errors.Wrapf(err, "failed to parse allowed IP's for peer %s", peer.Uid)
		}
		peerExtraAllowedIPs, err := parseIpAddressString(peer.ExtraAllowedIPsStr)
		if err != nil {
			return wgtypes.PeerConfig{}, errors.Wrapf(err, "failed to parse extra allowed IP's for peer %s", peer.Uid)
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

func (m *ManagementUtil) deviceExists(identifier DeviceIdentifier) bool {
	if _, ok := m.interfaces[identifier]; ok {
		return true
	}
	return false
}

func (m *ManagementUtil) peerExists(identifier PeerIdentifier) bool {
	for _, peers := range m.peers {
		if _, ok := peers[identifier]; ok {
			return true
		}
	}

	return false
}

func (m *ManagementUtil) persistInterface(identifier DeviceIdentifier, delete bool) error {
	var err error

	device := m.interfaces[identifier]
	peers := make([]PeerConfig, 0, len(m.peers[identifier]))
	for _, config := range m.peers[identifier] {
		peers = append(peers, config)
	}

	if !delete {
		err = m.cs.SaveInterface(device, peers)
	} else {
		err = m.cs.DeleteInterface(device.DeviceName)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to persist interface %s", identifier)
	}

	return nil
}

func (m *ManagementUtil) persistPeer(identifier PeerIdentifier, delete bool) error {
	var err error

	var device InterfaceConfig
	var peer PeerConfig
	for dev, peers := range m.peers {
		if p, ok := peers[identifier]; ok {
			device = m.interfaces[dev]
			peer = p
			break
		}
	}

	if !delete {
		err = m.cs.SavePeer(peer, device.DeviceName)
	} else {
		err = m.cs.DeletePeer(peer.Uid, device.DeviceName)
	}

	if err != nil {
		return errors.Wrapf(err, "failed to persist peer %s", identifier)
	}

	return nil
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

func (m *ManagementUtil) convertWireGuardInterface(device wgtypes.Device) InterfaceConfig {
	cfg := InterfaceConfig{
		DeviceName:   DeviceIdentifier(device.Name),
		KeyPair:      KeyPair{PublicKey: device.PublicKey.String(), PrivateKey: device.PrivateKey.String()},
		ListenPort:   device.ListenPort,
		FirewallMark: int32(device.FirewallMark),
		DriverType:   device.Type.String(),
	}

	link, err := m.nl.LinkByName(device.Name)
	if err != nil || link.Attrs() == nil {
		return cfg
	}
	cfg.Mtu = link.Attrs().MTU

	addresses, err := m.nl.AddrList(link)
	if err != nil {
		return cfg
	}
	addressesStr := make([]string, len(addresses))
	for i := range addresses {
		addressesStr[i] = addresses[i].String()
	}
	cfg.AddressStr = strings.Join(addressesStr, ",")

	return cfg
}

func (m *ManagementUtil) convertWireGuardPeer(peer wgtypes.Peer) PeerConfig {
	cfg := PeerConfig{
		KeyPair: KeyPair{PublicKey: peer.PublicKey.String()},
	}

	if peer.Endpoint != nil {
		cfg.Endpoint.Value = peer.Endpoint.String()
	}

	if peer.PresharedKey != (wgtypes.Key{}) {
		cfg.PresharedKey = peer.PresharedKey.String()
	}

	ipAddresses := make([]string, len(peer.AllowedIPs)) // use allowed IP's as the peer IP's
	for i, ip := range peer.AllowedIPs {
		ipAddresses[i] = ip.String()
	}
	cfg.AddressStr.Value = strings.Join(ipAddresses, ",")

	return cfg
}
