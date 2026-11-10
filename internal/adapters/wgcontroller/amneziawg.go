package wgcontroller

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

type AmneziaWGRepo interface {
	WgCtrlRepo
	AmneziaWGOptions(string) (domain.AmneziaWGOptions, error)
	ConfigureAmneziaWGDevice(string, wgtypes.Config, domain.AmneziaWGOptions) error
}

// AmneziaWGController manages AmneziaWG kernel interfaces and their obfuscation
// options. Its local helper uses the same AWG client for Linux routing, DNS,
// hooks, and peer lifecycle operations.
type AmneziaWGController struct {
	awg   AmneziaWGRepo
	local LocalController
}

func NewAmneziaWGController(cfg *config.Config) (*AmneziaWGController, error) {
	awg, err := lowlevel.NewAmneziaWGClient()
	if err != nil {
		return nil, err
	}
	return newAmneziaWGController(cfg, awg, &lowlevel.NetlinkManager{}), nil
}

func newAmneziaWGController(cfg *config.Config, awg AmneziaWGRepo, nl NetlinkClient) *AmneziaWGController {
	return &AmneziaWGController{
		awg: awg,
		local: LocalController{
			cfg:                   cfg,
			wg:                    awg,
			nl:                    nl,
			shellCmd:              "bash",                            // we only support bash at the moment
			resolvConfIfacePrefix: cfg.Backend.LocalResolvconfPrefix, // WireGuard interfaces have a tun. prefix in resolvconf
		},
	}
}

func (c AmneziaWGController) GetId() domain.InterfaceBackend {
	return config.AmneziaWGBackendName
}

func (c AmneziaWGController) GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error) {
	devices, err := c.awg.Devices()
	if err != nil {
		return nil, fmt.Errorf("AmneziaWG device list error: %w", err)
	}
	interfaces := make([]domain.PhysicalInterface, 0, len(devices))
	for _, device := range devices {
		iface, err := c.convertInterface(device)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

func (c AmneziaWGController) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error) {
	device, err := c.awg.Device(string(id))
	if err != nil {
		return nil, err
	}
	iface, err := c.convertInterface(device)
	if err != nil {
		return nil, err
	}
	return &iface, nil
}

func (c AmneziaWGController) convertInterface(device *wgtypes.Device) (domain.PhysicalInterface, error) {
	iface, err := c.local.convertWireGuardInterface(device)
	if err != nil {
		return domain.PhysicalInterface{}, err
	}
	iface.ImportSource = domain.ControllerTypeAmneziaWG
	options, err := c.awg.AmneziaWGOptions(device.Name)
	if err != nil {
		return domain.PhysicalInterface{}, fmt.Errorf("AmneziaWG options for %s: %w", device.Name, err)
	}
	iface.SetExtras(domain.AmneziaWGInterfaceExtras{Options: options})
	return iface, nil
}

func (c AmneziaWGController) SaveInterface(ctx context.Context, id domain.InterfaceIdentifier,
	updateFunc func(*domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	iface, err := c.GetInterface(ctx, id)
	if errors.Is(err, os.ErrNotExist) {
		if err := c.local.nl.LinkAdd(&netlink.GenericLink{Name: string(id), LinkType: "amneziawg"}); err != nil {
			return fmt.Errorf("AmneziaWG link add failed: %w", err)
		}
		iface, err = c.GetInterface(ctx, id)
	}
	if err != nil {
		return err
	}
	if updateFunc != nil {
		iface, err = updateFunc(iface)
		if err != nil {
			return err
		}
	}
	extras := iface.GetExtras().(domain.AmneziaWGInterfaceExtras)
	if err := extras.Options.Validate(); err != nil {
		return err
	}
	privateKey, err := wgtypes.NewKey(iface.KeyPair.GetPrivateKeyBytes())
	if err != nil {
		return err
	}
	var fwMark *int
	if iface.FirewallMark != 0 {
		value := int(iface.FirewallMark)
		fwMark = &value
	}
	if err := c.local.updateLowLevelInterface(iface); err != nil {
		return err
	}
	return c.awg.ConfigureAmneziaWGDevice(string(id), wgtypes.Config{
		PrivateKey: &privateKey, ListenPort: &iface.ListenPort, FirewallMark: fwMark,
	}, extras.Options)
}

func (c AmneziaWGController) DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error {
	link, err := c.local.nl.LinkByName(string(id))
	if err != nil {
		if _, ok := errors.AsType[netlink.LinkNotFoundError](err); ok {
			return nil
		}
		return err
	}
	if link.Type() != "amneziawg" {
		return fmt.Errorf("interface %s is not an AmneziaWG kernel device", id)
	}
	return c.local.nl.LinkDel(link)
}

func (c AmneziaWGController) GetPeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error) {
	peers, err := c.local.GetPeers(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range peers {
		peers[i].ImportSource = domain.ControllerTypeAmneziaWG
	}
	return peers, nil
}

func (c AmneziaWGController) GetPeer(ctx context.Context, id domain.InterfaceIdentifier, peerID domain.PeerIdentifier) (*domain.PhysicalPeer, error) {
	peer, err := c.local.getPeer(id, peerID)
	if err != nil {
		return nil, err
	}
	peer.ImportSource = domain.ControllerTypeAmneziaWG
	return peer, nil
}

func (c AmneziaWGController) SavePeer(ctx context.Context, id domain.InterfaceIdentifier, peerID domain.PeerIdentifier,
	updateFunc func(*domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	return c.local.SavePeer(ctx, id, peerID, func(peer *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		peer.ImportSource = domain.ControllerTypeAmneziaWG
		return updateFunc(peer)
	})
}

func (c AmneziaWGController) DeletePeer(ctx context.Context, id domain.InterfaceIdentifier, peerID domain.PeerIdentifier) error {
	return c.local.DeletePeer(ctx, id, peerID)
}

func (c AmneziaWGController) ExecuteInterfaceHook(ctx context.Context, id domain.InterfaceIdentifier, command string) error {
	return c.local.ExecuteInterfaceHook(ctx, id, command)
}

func (c AmneziaWGController) SetDNS(ctx context.Context, id domain.InterfaceIdentifier, dns, search string) error {
	return c.local.SetDNS(ctx, id, dns, search)
}

func (c AmneziaWGController) UnsetDNS(ctx context.Context, id domain.InterfaceIdentifier, dns, search string) error {
	return c.local.UnsetDNS(ctx, id, dns, search)
}

func (c AmneziaWGController) SetRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	return c.local.SetRoutes(ctx, info)
}

func (c AmneziaWGController) RemoveRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	return c.local.RemoveRoutes(ctx, info)
}

func (c AmneziaWGController) PingAddresses(ctx context.Context, addr string) (*domain.PingerResult, error) {
	return c.local.PingAddresses(ctx, addr)
}
