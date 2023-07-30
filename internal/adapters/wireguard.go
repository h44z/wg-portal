package adapters

import (
	"context"
	"errors"
	"fmt"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"os"
)

// WgRepo implements all low-level WireGuard interactions.
type WgRepo struct {
	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient
}

func NewWireGuardRepository() *WgRepo {
	wg, err := wgctrl.New()
	if err != nil {
		panic("failed to init wgctrl: " + err.Error())
	}

	nl := &lowlevel.NetlinkManager{}

	repo := &WgRepo{
		wg: wg,
		nl: nl,
	}

	return repo
}

func (r *WgRepo) GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error) {
	devices, err := r.wg.Devices()
	if err != nil {
		return nil, fmt.Errorf("device list error: %w", err)
	}

	interfaces := make([]domain.PhysicalInterface, 0, len(devices))
	for _, device := range devices {
		interfaceModel, err := r.convertWireGuardInterface(device)
		if err != nil {
			return nil, fmt.Errorf("interface convert failed for %s: %w", device.Name, err)
		}
		interfaces = append(interfaces, interfaceModel)
	}

	return interfaces, nil
}

func (r *WgRepo) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	return r.getInterface(id)
}

func (r *WgRepo) GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error) {
	device, err := r.wg.Device(string(deviceId))
	if err != nil {
		return nil, fmt.Errorf("device error: %w", err)
	}

	peers := make([]domain.PhysicalPeer, 0, len(device.Peers))
	for _, peer := range device.Peers {
		peerModel, err := r.convertWireGuardPeer(&peer)
		if err != nil {
			return nil, fmt.Errorf("peer convert failed for %v: %w", peer.PublicKey, err)
		}
		peers = append(peers, peerModel)
	}

	return peers, nil
}

func (r *WgRepo) GetPeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error) {
	return r.getPeer(deviceId, id)
}

func (r *WgRepo) convertWireGuardInterface(device *wgtypes.Device) (domain.PhysicalInterface, error) {
	// read data from wgctrl interface

	iface := domain.PhysicalInterface{
		Identifier: domain.InterfaceIdentifier(device.Name),
		KeyPair: domain.KeyPair{
			PrivateKey: device.PrivateKey.String(),
			PublicKey:  device.PublicKey.String(),
		},
		ListenPort:    device.ListenPort,
		Addresses:     nil,
		Mtu:           0,
		FirewallMark:  int32(device.FirewallMark),
		DeviceUp:      false,
		ImportSource:  "wgctrl",
		DeviceType:    device.Type.String(),
		BytesUpload:   0,
		BytesDownload: 0,
	}

	// read data from netlink interface

	lowLevelInterface, err := r.nl.LinkByName(device.Name)
	if err != nil {
		return domain.PhysicalInterface{}, fmt.Errorf("netlink error for %s: %w", device.Name, err)
	}
	ipAddresses, err := r.nl.AddrList(lowLevelInterface)
	if err != nil {
		return domain.PhysicalInterface{}, fmt.Errorf("ip read error for %s: %w", device.Name, err)
	}

	for _, addr := range ipAddresses {
		iface.Addresses = append(iface.Addresses, domain.CidrFromNetlinkAddr(addr))
	}
	iface.Mtu = lowLevelInterface.Attrs().MTU
	iface.DeviceUp = lowLevelInterface.Attrs().OperState == netlink.OperUnknown // wg only supports unknown
	if stats := lowLevelInterface.Attrs().Statistics; stats != nil {
		iface.BytesUpload = stats.TxBytes
		iface.BytesDownload = stats.RxBytes
	}

	return iface, nil
}

func (r *WgRepo) convertWireGuardPeer(peer *wgtypes.Peer) (domain.PhysicalPeer, error) {
	peerModel := domain.PhysicalPeer{
		Identifier: domain.PeerIdentifier(peer.PublicKey.String()),
		Endpoint:   "",
		AllowedIPs: nil,
		KeyPair: domain.KeyPair{
			PublicKey: peer.PublicKey.String(),
		},
		PresharedKey:        "",
		PersistentKeepalive: int(peer.PersistentKeepaliveInterval.Seconds()),
		LastHandshake:       peer.LastHandshakeTime,
		ProtocolVersion:     peer.ProtocolVersion,
		BytesUpload:         uint64(peer.ReceiveBytes),
		BytesDownload:       uint64(peer.TransmitBytes),
	}

	for _, addr := range peer.AllowedIPs {
		peerModel.AllowedIPs = append(peerModel.AllowedIPs, domain.CidrFromIpNet(addr))
	}
	if peer.Endpoint != nil {
		peerModel.Endpoint = peer.Endpoint.String()
	}
	if peer.PresharedKey != (wgtypes.Key{}) {
		peerModel.PresharedKey = domain.PreSharedKey(peer.PresharedKey.String())
	}

	return peerModel, nil
}

func (r *WgRepo) SaveInterface(_ context.Context, id domain.InterfaceIdentifier, updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error)) error {
	physicalInterface, err := r.getOrCreateInterface(id)
	if err != nil {
		return err
	}

	if updateFunc != nil {
		physicalInterface, err = updateFunc(physicalInterface)
		if err != nil {
			return err
		}
	}

	if err := r.updateLowLevelInterface(physicalInterface); err != nil {
		return err
	}
	if err := r.updateWireGuardInterface(physicalInterface); err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) getOrCreateInterface(id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	device, err := r.getInterface(id)
	if err == nil {
		return device, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("device error: %w", err)
	}

	// create new device
	if err := r.createLowLevelInterface(id); err != nil {
		return nil, err
	}

	device, err = r.getInterface(id)
	return device, err
}

func (r *WgRepo) getInterface(id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	device, err := r.wg.Device(string(id))
	if err != nil {
		return nil, err
	}

	pi, err := r.convertWireGuardInterface(device)
	return &pi, err
}

func (r *WgRepo) createLowLevelInterface(id domain.InterfaceIdentifier) error {
	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(id),
		},
		LinkType: "wireguard",
	}
	err := r.nl.LinkAdd(link)
	if err != nil {
		return fmt.Errorf("link add failed: %w", err)
	}

	return nil
}

func (r *WgRepo) updateLowLevelInterface(pi *domain.PhysicalInterface) error {
	link, err := r.nl.LinkByName(string(pi.Identifier))
	if err != nil {
		return err
	}
	if pi.Mtu != 0 {
		if err := r.nl.LinkSetMTU(link, pi.Mtu); err != nil {
			return fmt.Errorf("mtu error: %w", err)
		}
	}

	for _, addr := range pi.Addresses {
		err := r.nl.AddrReplace(link, addr.NetlinkAddr())
		if err != nil {
			return fmt.Errorf("failed to set ip %s: %w", addr.String(), err)
		}
	}

	// Remove unwanted IP addresses
	rawAddresses, err := r.nl.AddrList(link)
	if err != nil {
		return fmt.Errorf("failed to fetch interface ips: %w", err)
	}
	for _, rawAddr := range rawAddresses {
		netlinkAddr := domain.CidrFromNetlinkAddr(rawAddr)
		remove := true
		for _, addr := range pi.Addresses {
			if addr == netlinkAddr {
				remove = false
				break
			}
		}

		if !remove {
			continue
		}

		err := r.nl.AddrDel(link, &rawAddr)
		if err != nil {
			return fmt.Errorf("failed to remove deprecated ip %s: %w", netlinkAddr.String(), err)
		}
	}

	// Update link state
	if pi.DeviceUp {
		if err := r.nl.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring up device: %w", err)
		}
	} else {
		if err := r.nl.LinkSetDown(link); err != nil {
			return fmt.Errorf("failed to bring down device: %w", err)
		}
	}

	return nil
}

func (r *WgRepo) updateWireGuardInterface(pi *domain.PhysicalInterface) error {
	pKey, err := wgtypes.NewKey(pi.KeyPair.GetPrivateKeyBytes())
	if err != nil {
		return err
	}

	var fwMark *int
	if pi.FirewallMark != 0 {
		*fwMark = int(pi.FirewallMark)
	}
	err = r.wg.ConfigureDevice(string(pi.Identifier), wgtypes.Config{
		PrivateKey:   &pKey,
		ListenPort:   &pi.ListenPort,
		FirewallMark: fwMark,
		ReplacePeers: false,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) SaveRoutes(_ context.Context, interfaceId domain.InterfaceIdentifier, table int, allowedIPs []domain.Cidr) error {
	if table == -1 {
		logrus.Trace("ignoring route update")
		return nil
	}

	link, err := r.nl.LinkByName(string(interfaceId))
	if err != nil {
		return err
	}

	if link.Attrs().OperState == netlink.OperDown {
		return nil // cannot set route for interface that is down
	}

	// try to mimic wg-quick (https://git.zx2c4.com/wireguard-tools/tree/src/wg-quick/linux.bash)
	for _, allowedIP := range allowedIPs {
		// if allowedIP.Prefix().Bits() == 0 { // default route handling - TODO
		err := r.nl.RouteReplace(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       allowedIP.IpNet(),
			Table:     table,
			Scope:     unix.RT_SCOPE_LINK,
			Type:      unix.RTN_UNICAST,
		})
		if err != nil {
			return fmt.Errorf("failed to add/update route %s: %w", allowedIP.String(), err)
		}
	}

	// Remove unwanted routes
	rawRoutes, err := r.nl.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     unix.RT_TABLE_UNSPEC, // all tables
		Scope:     unix.RT_SCOPE_LINK,
		Type:      unix.RTN_UNICAST,
	}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_TYPE|netlink.RT_FILTER_OIF)
	if err != nil {
		return fmt.Errorf("failed to fetch raw routes: %w", err)
	}
	for _, rawRoute := range rawRoutes {
		netlinkAddr := domain.CidrFromIpNet(*rawRoute.Dst)
		remove := true
		for _, allowedIP := range allowedIPs {
			if netlinkAddr == allowedIP {
				remove = false
				break
			}
		}

		if !remove {
			continue
		}

		err := r.nl.RouteDel(&rawRoute)
		if err != nil {
			return fmt.Errorf("failed to remove deprecated route %s: %w", netlinkAddr.String(), err)
		}
	}

	return nil
}

func (r *WgRepo) DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error {
	if err := r.deleteLowLevelInterface(id); err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) deleteLowLevelInterface(id domain.InterfaceIdentifier) error {
	link, err := r.nl.LinkByName(string(id))
	if err != nil {
		return fmt.Errorf("unable to find low level interface: %w", err)
	}

	err = r.nl.LinkDel(link)
	if err != nil {
		return fmt.Errorf("failed to delete low level interface: %w", err)
	}

	return nil
}

func (r *WgRepo) SavePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier, updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error)) error {
	physicalPeer, err := r.getOrCreatePeer(deviceId, id)
	if err != nil {
		return err
	}

	physicalPeer, err = updateFunc(physicalPeer)
	if err != nil {
		return err
	}

	if err := r.updatePeer(deviceId, physicalPeer); err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) getOrCreatePeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error) {
	peer, err := r.getPeer(deviceId, id)
	if err == nil {
		return peer, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("peer error: %w", err)
	}

	// create new peer
	err = r.wg.ConfigureDevice(string(deviceId), wgtypes.Config{Peers: []wgtypes.PeerConfig{{
		PublicKey: id.ToPublicKey(),
	}}})

	peer, err = r.getPeer(deviceId, id)
	return peer, nil
}

func (r *WgRepo) getPeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error) {
	if !id.IsPublicKey() {
		return nil, errors.New("invalid public key")
	}

	device, err := r.wg.Device(string(deviceId))
	if err != nil {
		return nil, err
	}

	publicKey := id.ToPublicKey()
	for _, peer := range device.Peers {
		if peer.PublicKey != publicKey {
			continue
		}

		peerModel, err := r.convertWireGuardPeer(&peer)
		return &peerModel, err
	}

	return nil, os.ErrNotExist
}

func (r *WgRepo) updatePeer(deviceId domain.InterfaceIdentifier, pp *domain.PhysicalPeer) error {
	cfg := wgtypes.PeerConfig{
		PublicKey:                   pp.GetPublicKey(),
		Remove:                      false,
		UpdateOnly:                  true,
		PresharedKey:                pp.GetPresharedKey(),
		Endpoint:                    pp.GetEndpointAddress(),
		PersistentKeepaliveInterval: pp.GetPersistentKeepaliveTime(),
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  pp.GetAllowedIPs(),
	}

	err := r.wg.ConfigureDevice(string(deviceId), wgtypes.Config{ReplacePeers: false, Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) DeletePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error {
	if !id.IsPublicKey() {
		return errors.New("invalid public key")
	}

	err := r.deletePeer(deviceId, id)
	if err != nil {
		return err
	}

	return nil
}

func (r *WgRepo) deletePeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error {
	cfg := wgtypes.PeerConfig{
		PublicKey: id.ToPublicKey(),
		Remove:    true,
	}

	err := r.wg.ConfigureDevice(string(deviceId), wgtypes.Config{ReplacePeers: false, Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return err
	}

	return nil
}
