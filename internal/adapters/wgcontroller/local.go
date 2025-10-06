package wgcontroller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

// region dependencies

// WgCtrlRepo is used to control local WireGuard devices via the wgctrl-go library.
type WgCtrlRepo interface {
	io.Closer
	Devices() ([]*wgtypes.Device, error)
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, cfg wgtypes.Config) error
}

// A NetlinkClient is a type which can control a netlink device.
type NetlinkClient interface {
	LinkAdd(link netlink.Link) error
	LinkDel(link netlink.Link) error
	LinkByName(name string) (netlink.Link, error)
	LinkSetUp(link netlink.Link) error
	LinkSetDown(link netlink.Link) error
	LinkSetMTU(link netlink.Link, mtu int) error
	AddrReplace(link netlink.Link, addr *netlink.Addr) error
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrList(link netlink.Link) ([]netlink.Addr, error)
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	RouteAdd(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	RouteReplace(route *netlink.Route) error
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error)
	RuleAdd(rule *netlink.Rule) error
	RuleDel(rule *netlink.Rule) error
	RuleList(family int) ([]netlink.Rule, error)
}

// endregion dependencies

type LocalController struct {
	cfg *config.Config

	wg WgCtrlRepo
	nl NetlinkClient

	shellCmd              string
	resolvConfIfacePrefix string
}

// NewLocalController creates a new local controller instance.
// This repository is used to interact with the WireGuard kernel or userspace module.
func NewLocalController(cfg *config.Config) (*LocalController, error) {
	wg, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	nl := &lowlevel.NetlinkManager{}

	repo := &LocalController{
		cfg: cfg,

		wg: wg,
		nl: nl,

		shellCmd:              "bash",                            // we only support bash at the moment
		resolvConfIfacePrefix: cfg.Backend.LocalResolvconfPrefix, // WireGuard interfaces have a tun. prefix in resolvconf
	}

	return repo, nil
}

func (c LocalController) GetId() domain.InterfaceBackend {
	return config.LocalBackendName
}

// region wireguard-related

func (c LocalController) GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error) {
	devices, err := c.wg.Devices()
	if err != nil {
		return nil, fmt.Errorf("device list error: %w", err)
	}

	interfaces := make([]domain.PhysicalInterface, 0, len(devices))
	for _, device := range devices {
		interfaceModel, err := c.convertWireGuardInterface(device)
		if err != nil {
			return nil, fmt.Errorf("interface convert failed for %s: %w", device.Name, err)
		}
		interfaces = append(interfaces, interfaceModel)
	}

	return interfaces, nil
}

func (c LocalController) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	return c.getInterface(id)
}

func (c LocalController) convertWireGuardInterface(device *wgtypes.Device) (domain.PhysicalInterface, error) {
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
		FirewallMark:  uint32(device.FirewallMark),
		DeviceUp:      false,
		ImportSource:  domain.ControllerTypeLocal,
		DeviceType:    device.Type.String(),
		BytesUpload:   0,
		BytesDownload: 0,
	}

	// read data from netlink interface

	lowLevelInterface, err := c.nl.LinkByName(device.Name)
	if err != nil {
		return domain.PhysicalInterface{}, fmt.Errorf("netlink error for %s: %w", device.Name, err)
	}
	ipAddresses, err := c.nl.AddrList(lowLevelInterface)
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

func (c LocalController) GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	device, err := c.wg.Device(string(deviceId))
	if err != nil {
		return nil, fmt.Errorf("device error: %w", err)
	}

	peers := make([]domain.PhysicalPeer, 0, len(device.Peers))
	for _, peer := range device.Peers {
		peerModel, err := c.convertWireGuardPeer(&peer)
		if err != nil {
			return nil, fmt.Errorf("peer convert failed for %v: %w", peer.PublicKey, err)
		}
		peers = append(peers, peerModel)
	}

	return peers, nil
}

func (c LocalController) convertWireGuardPeer(peer *wgtypes.Peer) (domain.PhysicalPeer, error) {
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
		ImportSource:        domain.ControllerTypeLocal,
	}

	// Set local extras - local peers are never disabled in the kernel
	peerModel.SetExtras(domain.LocalPeerExtras{
		Disabled: false,
	})

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

func (c LocalController) SaveInterface(
	_ context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	physicalInterface, err := c.getOrCreateInterface(id)
	if err != nil {
		return err
	}

	if updateFunc != nil {
		physicalInterface, err = updateFunc(physicalInterface)
		if err != nil {
			return err
		}
	}

	if err := c.updateLowLevelInterface(physicalInterface); err != nil {
		return err
	}
	if err := c.updateWireGuardInterface(physicalInterface); err != nil {
		return err
	}

	return nil
}

func (c LocalController) getOrCreateInterface(id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	device, err := c.getInterface(id)
	if err == nil {
		return device, nil // interface exists
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("device error: %w", err) // unknown error
	}

	// create new device
	if err := c.createLowLevelInterface(id); err != nil {
		return nil, err
	}

	device, err = c.getInterface(id)
	return device, err
}

func (c LocalController) getInterface(id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	device, err := c.wg.Device(string(id))
	if err != nil {
		return nil, err
	}

	pi, err := c.convertWireGuardInterface(device)
	return &pi, err
}

func (c LocalController) createLowLevelInterface(id domain.InterfaceIdentifier) error {
	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{
			Name: string(id),
		},
		LinkType: "wireguard",
	}
	err := c.nl.LinkAdd(link)
	if err != nil {
		return fmt.Errorf("link add failed: %w", err)
	}

	return nil
}

func (c LocalController) updateLowLevelInterface(pi *domain.PhysicalInterface) error {
	link, err := c.nl.LinkByName(string(pi.Identifier))
	if err != nil {
		return err
	}
	if pi.Mtu != 0 {
		if err := c.nl.LinkSetMTU(link, pi.Mtu); err != nil {
			return fmt.Errorf("mtu error: %w", err)
		}
	}

	for _, addr := range pi.Addresses {
		err := c.nl.AddrReplace(link, addr.NetlinkAddr())
		if err != nil {
			return fmt.Errorf("failed to set ip %s: %w", addr.String(), err)
		}
	}

	// Remove unwanted IP addresses
	rawAddresses, err := c.nl.AddrList(link)
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

		err := c.nl.AddrDel(link, &rawAddr)
		if err != nil {
			return fmt.Errorf("failed to remove deprecated ip %s: %w", netlinkAddr.String(), err)
		}
	}

	// Update link state
	if pi.DeviceUp {
		if err := c.nl.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring up device: %w", err)
		}
	} else {
		if err := c.nl.LinkSetDown(link); err != nil {
			return fmt.Errorf("failed to bring down device: %w", err)
		}
	}

	return nil
}

func (c LocalController) updateWireGuardInterface(pi *domain.PhysicalInterface) error {
	pKey, err := wgtypes.NewKey(pi.KeyPair.GetPrivateKeyBytes())
	if err != nil {
		return err
	}

	var fwMark *int
	if pi.FirewallMark != 0 {
		intFwMark := int(pi.FirewallMark)
		fwMark = &intFwMark
	}
	err = c.wg.ConfigureDevice(string(pi.Identifier), wgtypes.Config{
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

func (c LocalController) DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error {
	if err := c.deleteLowLevelInterface(id); err != nil {
		return err
	}

	return nil
}

func (c LocalController) deleteLowLevelInterface(id domain.InterfaceIdentifier) error {
	link, err := c.nl.LinkByName(string(id))
	if err != nil {
		var linkNotFoundError netlink.LinkNotFoundError
		if errors.As(err, &linkNotFoundError) {
			return nil // ignore not found error
		}
		return fmt.Errorf("unable to find low level interface: %w", err)
	}

	err = c.nl.LinkDel(link)
	if err != nil {
		return fmt.Errorf("failed to delete low level interface: %w", err)
	}

	return nil
}

func (c LocalController) SavePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	physicalPeer, err := c.getOrCreatePeer(deviceId, id)
	if err != nil {
		return err
	}

	physicalPeer, err = updateFunc(physicalPeer)
	if err != nil {
		return err
	}

	// Check if the peer is disabled by looking at the backend extras
	// For local controller, disabled peers should be deleted
	if physicalPeer.GetExtras() != nil {
		switch extras := physicalPeer.GetExtras().(type) {
		case domain.LocalPeerExtras:
			if extras.Disabled {
				// Delete the peer instead of updating it
				return c.deletePeer(deviceId, id)
			}
		}
	}

	if err := c.updatePeer(deviceId, physicalPeer); err != nil {
		return err
	}

	return nil
}

func (c LocalController) getOrCreatePeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (
	*domain.PhysicalPeer,
	error,
) {
	peer, err := c.getPeer(deviceId, id)
	if err == nil {
		return peer, nil // peer exists
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("peer error: %w", err) // unknown error
	}

	// create new peer
	err = c.wg.ConfigureDevice(string(deviceId), wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: id.ToPublicKey(),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("peer create error for %s: %w", id.ToPublicKey(), err)
	}

	peer, err = c.getPeer(deviceId, id)
	if err != nil {
		return nil, fmt.Errorf("peer error after create: %w", err)
	}
	return peer, nil
}

func (c LocalController) getPeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (
	*domain.PhysicalPeer,
	error,
) {
	if !id.IsPublicKey() {
		return nil, errors.New("invalid public key")
	}

	device, err := c.wg.Device(string(deviceId))
	if err != nil {
		return nil, err
	}

	publicKey := id.ToPublicKey()
	for _, peer := range device.Peers {
		if peer.PublicKey != publicKey {
			continue
		}

		peerModel, err := c.convertWireGuardPeer(&peer)
		return &peerModel, err
	}

	return nil, os.ErrNotExist
}

func (c LocalController) updatePeer(deviceId domain.InterfaceIdentifier, pp *domain.PhysicalPeer) error {
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

	err := c.wg.ConfigureDevice(string(deviceId), wgtypes.Config{ReplacePeers: false, Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return err
	}

	return nil
}

func (c LocalController) DeletePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	if !id.IsPublicKey() {
		return errors.New("invalid public key")
	}

	err := c.deletePeer(deviceId, id)
	if err != nil {
		return err
	}

	return nil
}

func (c LocalController) deletePeer(deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error {
	cfg := wgtypes.PeerConfig{
		PublicKey: id.ToPublicKey(),
		Remove:    true,
	}

	err := c.wg.ConfigureDevice(string(deviceId), wgtypes.Config{ReplacePeers: false, Peers: []wgtypes.PeerConfig{cfg}})
	if err != nil {
		return err
	}

	return nil
}

// endregion wireguard-related

// region wg-quick-related

func (c LocalController) ExecuteInterfaceHook(
	_ context.Context,
	id domain.InterfaceIdentifier,
	hookCmd string,
) error {
	if hookCmd == "" {
		return nil
	}

	slog.Debug("executing interface hook", "interface", id, "hook", hookCmd)
	err := c.exec(hookCmd, id)
	if err != nil {
		return fmt.Errorf("failed to exec hook: %w", err)
	}

	return nil
}

func (c LocalController) SetDNS(_ context.Context, id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error {
	if dnsStr == "" && dnsSearchStr == "" {
		return nil
	}

	dnsServers := internal.SliceString(dnsStr)
	dnsSearchDomains := internal.SliceString(dnsSearchStr)

	dnsCommand := "resolvconf -a %resPref%i -m 0 -x"
	dnsCommandInput := make([]string, 0, len(dnsServers)+len(dnsSearchDomains))

	for _, dnsServer := range dnsServers {
		dnsCommandInput = append(dnsCommandInput, fmt.Sprintf("nameserver %s", dnsServer))
	}
	for _, searchDomain := range dnsSearchDomains {
		dnsCommandInput = append(dnsCommandInput, fmt.Sprintf("search %s", searchDomain))
	}

	err := c.exec(dnsCommand, id, dnsCommandInput...)
	if err != nil {
		return fmt.Errorf(
			"failed to set dns settings (is resolvconf available?, for systemd create this symlink: ln -s /usr/bin/resolvectl /usr/local/bin/resolvconf): %w",
			err,
		)
	}

	return nil
}

func (c LocalController) UnsetDNS(_ context.Context, id domain.InterfaceIdentifier, _, _ string) error {
	dnsCommand := "resolvconf -d %resPref%i -f"

	err := c.exec(dnsCommand, id)
	if err != nil {
		return fmt.Errorf("failed to unset dns settings: %w", err)
	}

	return nil
}

func (c LocalController) replaceCommandPlaceHolders(command string, interfaceId domain.InterfaceIdentifier) string {
	command = strings.ReplaceAll(command, "%resPref", c.resolvConfIfacePrefix)
	return strings.ReplaceAll(command, "%i", string(interfaceId))
}

func (c LocalController) exec(command string, interfaceId domain.InterfaceIdentifier, stdin ...string) error {
	commandWithInterfaceName := c.replaceCommandPlaceHolders(command, interfaceId)
	cmd := exec.Command(c.shellCmd, "-ce", commandWithInterfaceName)
	if len(stdin) > 0 {
		b := &bytes.Buffer{}
		for _, ln := range stdin {
			if _, err := fmt.Fprint(b, ln+"\n"); err != nil {
				return err
			}
		}
		cmd.Stdin = b
	}
	out, err := cmd.CombinedOutput() // execute and wait for output
	if err != nil {
		slog.Warn("failed to executed shell command",
			"command", commandWithInterfaceName, "stdin", stdin, "output", string(out), "error", err)
		return fmt.Errorf("failed to exexute shell command %s: %w", commandWithInterfaceName, err)
	}
	slog.Debug("executed shell command",
		"command", commandWithInterfaceName,
		"output", string(out))
	return nil
}

// endregion wg-quick-related

// region routing-related

// SetRoutes sets the routes for the given interface. If no routes are provided, the function is a no-op.
func (c LocalController) SetRoutes(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	table int,
	fwMark uint32,
	cidrs []domain.Cidr,
) error {
	return nil
}

// RemoveRoutes removes the routes for the given interface. If no routes are provided, the function is a no-op.
func (c LocalController) RemoveRoutes(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	table int,
	fwMark uint32,
	oldCidrs []domain.Cidr,
) error {
	return nil
}

// endregion routing-related

// region statistics-related

func (c LocalController) PingAddresses(
	ctx context.Context,
	addr string,
) (*domain.PingerResult, error) {
	pinger, err := probing.NewPinger(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate pinger for %s: %w", addr, err)
	}

	checkCount := 1
	pinger.SetPrivileged(!c.cfg.Statistics.PingUnprivileged)
	pinger.Count = checkCount
	pinger.Timeout = 2 * time.Second
	err = pinger.RunWithContext(ctx) // Blocks until finished.
	if err != nil {
		return nil, fmt.Errorf("failed to ping %s: %w", addr, err)
	}

	stats := pinger.Statistics()

	return &domain.PingerResult{
		PacketsRecv: stats.PacketsRecv,
		PacketsSent: stats.PacketsSent,
		Rtts:        stats.Rtts,
	}, nil
}

// endregion statistics-related
