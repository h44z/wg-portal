package wgcontroller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type awgControllerNetlink struct {
	NetlinkClient
	link    netlink.Link
	deleted bool
}

func (n *awgControllerNetlink) LinkAdd(link netlink.Link) error { n.link = link; return nil }
func (n *awgControllerNetlink) LinkByName(string) (netlink.Link, error) {
	if n.link == nil {
		return nil, netlink.LinkNotFoundError{}
	}
	return n.link, nil
}
func (n *awgControllerNetlink) LinkDel(netlink.Link) error {
	n.deleted = true
	n.link = nil
	return nil
}
func (n *awgControllerNetlink) AddrList(netlink.Link) ([]netlink.Addr, error) { return nil, nil }
func (n *awgControllerNetlink) LinkSetUp(netlink.Link) error                  { return nil }
func (n *awgControllerNetlink) LinkSetDown(netlink.Link) error                { return nil }

type awgControllerClient struct {
	WgCtrlRepo
	nl         *awgControllerNetlink
	options    domain.AmneziaWGOptions
	configured domain.AmneziaWGOptions
	deleted    bool
	peer       *wgtypes.Peer
}

func (w *awgControllerClient) Devices() ([]*wgtypes.Device, error) {
	device, err := w.Device("awg0")
	if err != nil {
		return nil, err
	}
	return []*wgtypes.Device{device}, nil
}

func (w *awgControllerClient) Device(name string) (*wgtypes.Device, error) {
	if w.nl.link == nil {
		return nil, os.ErrNotExist
	}
	device := &wgtypes.Device{Name: name, Type: wgtypes.LinuxKernel}
	if w.peer != nil {
		device.Peers = []wgtypes.Peer{*w.peer}
	}
	return device, nil
}
func (w *awgControllerClient) AmneziaWGOptions(string) (domain.AmneziaWGOptions, error) {
	return w.options, nil
}
func (w *awgControllerClient) ConfigureAmneziaWGDevice(_ string, _ wgtypes.Config, options domain.AmneziaWGOptions) error {
	w.configured = options
	return nil
}
func (w *awgControllerClient) ConfigureDevice(_ string, cfg wgtypes.Config) error {
	for _, peer := range cfg.Peers {
		if peer.Remove {
			w.deleted = true
		}
	}
	return nil
}

func TestAmneziaWGControllerCreatesCorrectLinkAndPreservesOptions(t *testing.T) {
	nl := &awgControllerNetlink{}
	wg := &awgControllerClient{nl: nl, options: domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4"}}
	controller := newAmneziaWGController(&config.Config{}, wg, nl)
	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)
	require.NoError(t, controller.SaveInterface(context.Background(), "awg0", func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
		require.Equal(t, domain.AmneziaWGInterfaceExtras{Options: wg.options}, pi.GetExtras())
		domain.MergeToPhysicalInterface(pi, &domain.Interface{Backend: config.AmneziaWGBackendName, Identifier: "awg0", PrivateKey: key.String(), AmneziaWG: domain.AmneziaWGOptions{"Jc": "6"}})
		return pi, nil
	}))
	require.Equal(t, "amneziawg", nl.link.Type())
	require.Equal(t, domain.InterfaceBackend(config.AmneziaWGBackendName), controller.GetId())
	require.Equal(t, domain.AmneziaWGOptions{"H1": "100-200", "Jc": "6"}, wg.configured)
	physical, err := controller.GetInterface(context.Background(), "awg0")
	require.NoError(t, err)
	require.Equal(t, domain.ControllerTypeAmneziaWG, physical.ImportSource)
	require.Equal(t, domain.AmneziaWGInterfaceExtras{Options: wg.options}, physical.GetExtras())
	require.Equal(t, wg.options, domain.ConvertPhysicalInterface(physical).AmneziaWG)
	interfaces, err := controller.GetInterfaces(context.Background())
	require.NoError(t, err)
	require.Len(t, interfaces, 1)
	require.Equal(t, *physical, interfaces[0])
}

func TestAmneziaWGDisabledPeersUseLocalDeletion(t *testing.T) {
	key, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)
	public := key.PublicKey()
	nl := &awgControllerNetlink{link: &netlink.GenericLink{Name: "awg0", LinkType: "amneziawg"}}
	wg := &awgControllerClient{nl: nl, peer: &wgtypes.Peer{PublicKey: public}}
	controller := newAmneziaWGController(&config.Config{}, wg, nl)
	id := domain.PeerIdentifier(public.String())
	physical, err := controller.GetPeer(context.Background(), "awg0", id)
	require.NoError(t, err)
	require.Equal(t, domain.ControllerTypeAmneziaWG, physical.ImportSource)
	peers, err := controller.GetPeers(context.Background(), "awg0")
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.Equal(t, domain.ControllerTypeAmneziaWG, peers[0].ImportSource)
	require.False(t, domain.ConvertPhysicalPeer(&peers[0]).IsDisabled())
	disabled := time.Now()
	require.NoError(t, controller.SavePeer(context.Background(), "awg0", id, func(peer *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		require.Equal(t, domain.ControllerTypeAmneziaWG, peer.ImportSource)
		domain.MergeToPhysicalPeer(peer, &domain.Peer{Identifier: id, Disabled: &disabled})
		return peer, nil
	}))
	require.True(t, wg.deleted)
}

func TestAmneziaWGControllerDeletesOnlyAmneziaWGLinks(t *testing.T) {
	nl := &awgControllerNetlink{}
	controller := newAmneziaWGController(&config.Config{}, &awgControllerClient{nl: nl}, nl)
	require.NoError(t, controller.DeleteInterface(context.Background(), "missing"))
	nl.link = &netlink.GenericLink{Name: "wg0", LinkType: "wireguard"}
	require.ErrorContains(t, controller.DeleteInterface(context.Background(), "wg0"), "not an AmneziaWG kernel device")
	require.False(t, nl.deleted)
	nl.link = &netlink.GenericLink{Name: "awg0", LinkType: "amneziawg"}
	require.NoError(t, controller.DeleteInterface(context.Background(), "awg0"))
	require.True(t, nl.deleted)
}
