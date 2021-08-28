package wireguard

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
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
	CreateDevice(device DeviceIdentifier) error
	DeleteDevice(device DeviceIdentifier) error
	UpdateDevice(device DeviceIdentifier, cfg InterfaceConfig) error
}

type PeerManager interface {
	GetPeers(device DeviceIdentifier) ([]PeerConfig, error)
	SavePeers(device DeviceIdentifier, peers ...PeerConfig) error
	RemovePeer(device DeviceIdentifier, peer PeerIdentifier) error
}

type Manager interface {
	KeyGenerator
	DeviceManager
	PeerManager
}

type ManagementUtil struct {
	mux sync.RWMutex // mutex to synchronize access to maps

	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient

	// config writers and loaders are used to populate the internal config maps
	cw []ConfigWriter
	cl []ConfigLoader

	// internal holder of interface configurations
	interfaces map[DeviceIdentifier]InterfaceConfig
	// internal holder of peer configurations
	peers map[DeviceIdentifier]map[PeerIdentifier]PeerConfig
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

func (m *ManagementUtil) CreateDevice(identifier DeviceIdentifier) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.deviceExists(identifier) {
		return errors.Errorf("device %s already exists", identifier)
	}
	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(identifier),
		},
		LinkType: "wireguard",
	}
	err := m.nl.LinkAdd(link)
	if err != nil {
		return errors.Wrapf(err, "failed to create WireGuard interface")
	}

	if err := m.nl.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "failed to enable WireGuard interface")
	}

	newInterface := InterfaceConfig{DeviceName: identifier}
	m.interfaces[identifier] = newInterface

	err = m.persistInterface(identifier, false)
	if err != nil {
		return errors.Wrapf(err, "failed to persist created interface %s", identifier)
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

	for _, writer := range m.cw {
		if delete {
			err = writer.DeleteInterface(device, peers)
		} else {
			err = writer.SaveInterface(device, peers)
		}
		if err != nil {
			return errors.Wrapf(err, "failed to persist interface %s", identifier)
		}
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

	for _, writer := range m.cw {
		if delete {
			err = writer.DeletePeer(peer, device)
		} else {
			err = writer.SavePeer(peer, device)
		}
		if err != nil {
			return errors.Wrapf(err, "failed to persist peer %s", identifier)
		}
	}

	return nil
}

// TODO: fix/implement
func (m *ManagementUtil) loadExistingInterfaces() ([]InterfaceConfig, error) {
	devices, err := m.wg.Devices()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get WireGuard device list")
	}

	interfaces := make([]InterfaceConfig, len(devices))
	for i, device := range devices {
		interfaces[i].DeviceName = DeviceIdentifier(device.Name)
		interfaces[i].FirewallMark = int32(device.FirewallMark)
		interfaces[i].KeyPair = KeyPair{
			PrivateKey: device.PrivateKey.String(),
			PublicKey:  device.PublicKey.String(),
		}
		interfaces[i].ListenPort = device.ListenPort
		interfaces[i].DriverType = device.Type.String()

		parsedInterface, _, err := m.parseConfigFile(device.Name)
		if err != nil {
			continue
		}
		interfaces[i].DnsStr = parsedInterface.DnsStr
		interfaces[i].DisplayName = parsedInterface.DisplayName
		interfaces[i].PostDown = parsedInterface.PostDown
		interfaces[i].PreDown = parsedInterface.PreDown
		interfaces[i].PostUp = parsedInterface.PostUp
		interfaces[i].PreUp = parsedInterface.PreUp
		interfaces[i].AddressStr = parsedInterface.AddressStr
		interfaces[i].RoutingTable = parsedInterface.RoutingTable
		interfaces[i].Mtu = parsedInterface.Mtu

		fmt.Println(interfaces[i])
	}

	return interfaces, nil
}

// parseConfigFile parses WireGuard configuration files (INI syntax) and some additional comments in the file
// TODO: fix/implement
func (m *ManagementUtil) parseConfigFile(interfaceName string) (InterfaceConfig, []PeerConfig, error) {
	configFile := "TODO" //filepath.Join(m.configPath, interfaceName+".conf")

	file, err := os.Open(configFile)
	if err != nil {
		return InterfaceConfig{}, nil, errors.Wrapf(err, "unable to open config file for interface %s", interfaceName)
	}
	scanner := bufio.NewScanner(file)

	peerSection := false
	iface := InterfaceConfig{}
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "#"): // A comment line
			line = line[1:]
			commentParts := strings.SplitN(line, "=", 1)
			fmt.Println(commentParts, peerSection)
		case strings.HasPrefix(line, "["): // Config section
			line = strings.ToLower(line[1 : len(line)-1])
			switch line {
			case "peer":
				peerSection = true
			case "interface":
				peerSection = false
			default:
				return InterfaceConfig{}, nil, errors.Errorf("configuration file contains unsupported section %s", line)
			}
		default: //Config option
			optionParts := strings.SplitN(line, "=", 1)
			if len(optionParts) != 2 {
				return InterfaceConfig{}, nil, errors.Errorf("configuration file contains invalid line %s", line)
			}
			option := strings.ToLower(strings.TrimSpace(optionParts[0]))
			value := strings.TrimSpace(optionParts[1])
			peerOption := false
			switch option {
			// Interface
			case "privatekey":
				key, err := wgtypes.ParseKey(value)
				if err != nil {
					return InterfaceConfig{}, nil, errors.Wrapf(err, "interface section has no valid private Key")
				}
				iface.KeyPair = KeyPair{
					PrivateKey: key.String(),
					PublicKey:  key.PublicKey().String(),
				}
			case "address":
				iface.AddressStr = value
			case "listenport":
				port, err := strconv.Atoi(value)
				if err != nil {
					return InterfaceConfig{}, nil, errors.Wrapf(err, "interface section has invalid listen port Value")
				}
				iface.ListenPort = port
			case "postup":
				iface.PostUp = value
			case "postdown":
				iface.PostDown = value
			case "preup":
				iface.PreUp = value
			case "predown":
				iface.PreDown = value
			case "mtu":
				mtu, err := strconv.Atoi(value)
				if err != nil {
					return InterfaceConfig{}, nil, errors.Wrapf(err, "interface section has invalid MTU Value")
				}
				iface.Mtu = mtu
			case "dns":
				iface.DnsStr = value
			case "table":
				iface.RoutingTable = value
			case "fwmark":
				fwMark, err := strconv.Atoi(value)
				if err != nil {
					return InterfaceConfig{}, nil, errors.Wrapf(err, "interface section has invalid fwmark Value")
				}
				iface.FirewallMark = int32(fwMark)
			case "saveconfig":
				saveConfig, err := strconv.ParseBool(value)
				if err != nil {
					return InterfaceConfig{}, nil, errors.Wrapf(err, "interface section has invalid save-config Value")
				}
				iface.SaveConfig = saveConfig
			// Peer
			case "endpoint":
				peerOption = true
			case "publickey":
				peerOption = true
			case "allowedips":
				peerOption = true
			case "persistentkeepalive":
				peerOption = true
			case "presharedkey":
				peerOption = true
			}

			if peerSection != peerOption {
				return InterfaceConfig{}, nil, errors.Errorf("config section contains invalid option %s", option)
			}

			fmt.Println(value)
		}
		if strings.HasPrefix(line, "#") {
			fmt.Println("comment")
		}
		fmt.Println(line)
	}

	if err := scanner.Err(); err != nil {
		return InterfaceConfig{}, nil, errors.Wrapf(err, "unable to scan config file for interface %s", interfaceName)
	}

	return InterfaceConfig{}, nil, nil
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
