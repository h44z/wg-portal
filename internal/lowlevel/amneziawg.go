package lowlevel

import (
	"errors"
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/domain"
)

// AmneziaWGClient implements the wgctrl client contract using the AmneziaWG
// kernel's Generic Netlink family.
type AmneziaWGClient struct {
	version    uint8
	exchange   func(command uint8, flags uint16, attrs []*nl.RtAttr) ([][]byte, error)
	linkByName func(string) (netlink.Link, error)
	links      func() ([]netlink.Link, error)
}

func NewAmneziaWGClient() (*AmneziaWGClient, error) {
	family, err := amneziaWGFamily(netlink.GenlFamilyGet)
	if err != nil {
		return nil, err
	}
	return &AmneziaWGClient{
		version: uint8(family.Version), linkByName: netlink.LinkByName, links: netlink.LinkList,
		exchange: func(command uint8, flags uint16, attrs []*nl.RtAttr) ([][]byte, error) {
			req := nl.NewNetlinkRequest(int(family.ID), int(flags))
			req.AddData(&nl.Genlmsg{Command: command, Version: uint8(family.Version)})
			for _, attr := range attrs {
				req.AddData(attr)
			}
			return req.Execute(unix.NETLINK_GENERIC, family.ID)
		},
	}, nil
}

func amneziaWGFamily(lookup func(string) (*netlink.GenlFamily, error)) (*netlink.GenlFamily, error) {
	family, err := lookup("amneziawg")
	if err != nil {
		return nil, fmt.Errorf("AmneziaWG kernel support is unavailable; install and load the amneziawg module on the host (modprobe amneziawg): %w", err)
	}
	if family == nil || family.Version < 1 || family.Version > 3 {
		return nil, fmt.Errorf("unsupported AmneziaWG Generic Netlink version (supported: 1, 2, 3)")
	}
	return family, nil
}

// Requests use their own short-lived sockets, which the netlink library closes.
func (c *AmneziaWGClient) Close() error { return nil }

func (c *AmneziaWGClient) kernelLink(name string) error {
	link, err := c.linkByName(name)
	if err != nil {
		if _, ok := errors.AsType[netlink.LinkNotFoundError](err); ok {
			return os.ErrNotExist
		}
		return err
	}
	if link.Type() != "amneziawg" {
		return fmt.Errorf("interface %s is not an AmneziaWG kernel device", name)
	}
	return nil
}

func (c *AmneziaWGClient) Devices() ([]*wgtypes.Device, error) {
	links, err := c.links()
	if err != nil {
		return nil, err
	}
	var devices []*wgtypes.Device
	for _, link := range links {
		if link.Type() != "amneziawg" {
			continue
		}
		device, err := c.Device(link.Attrs().Name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func (c *AmneziaWGClient) Device(name string) (*wgtypes.Device, error) {
	device, _, err := c.readDevice(name)
	return device, err
}

func (c *AmneziaWGClient) AmneziaWGOptions(name string) (domain.AmneziaWGOptions, error) {
	_, options, err := c.readDevice(name)
	return options, err
}

func (c *AmneziaWGClient) readDevice(name string) (*wgtypes.Device, domain.AmneziaWGOptions, error) {
	if err := c.kernelLink(name); err != nil {
		return nil, nil, err
	}
	messages, err := c.exchange(awgGetDevice, unix.NLM_F_DUMP, []*nl.RtAttr{nl.NewRtAttr(unix.WGDEVICE_A_IFNAME, nl.ZeroTerminated(name))})
	if errors.Is(err, unix.ENODEV) || errors.Is(err, unix.ENOENT) {
		return nil, nil, os.ErrNotExist
	}
	if err != nil {
		return nil, nil, fmt.Errorf("AmneziaWG device read: %w", err)
	}
	if len(messages) == 0 {
		return nil, nil, os.ErrNotExist
	}
	device, options, err := parseAmneziaWGDevice(messages, c.version)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid AmneziaWG device response: %w", err)
	}
	if device.Name != name {
		return nil, nil, fmt.Errorf("unexpected AmneziaWG interface in response")
	}
	return device, options, nil
}

func (c *AmneziaWGClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	return c.ConfigureAmneziaWGDevice(name, cfg, nil)
}

func (c *AmneziaWGClient) ConfigureAmneziaWGDevice(name string, cfg wgtypes.Config, options domain.AmneziaWGOptions) error {
	// Encode every batch before changing the device, so invalid input cannot
	// leave an otherwise valid prefix of the configuration applied.
	batches, err := amneziaWGConfigBatches(name, cfg, options, c.version)
	if err != nil {
		return err
	}
	if err := c.kernelLink(name); err != nil {
		return err
	}
	for _, attrs := range batches {
		if _, err := c.exchange(awgSetDevice, unix.NLM_F_ACK, attrs); err != nil {
			if errors.Is(err, unix.ENODEV) || errors.Is(err, unix.ENOENT) {
				return os.ErrNotExist
			}
			return fmt.Errorf("AmneziaWG device configuration: %w", err)
		}
	}
	return nil
}
