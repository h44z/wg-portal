package lowlevel

import (
	"github.com/vishvananda/netlink"
)

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
}

type NetlinkManager struct {
}

func (n NetlinkManager) LinkAdd(link netlink.Link) error { return netlink.LinkAdd(link) }

func (n NetlinkManager) LinkDel(link netlink.Link) error { return netlink.LinkDel(link) }

func (n NetlinkManager) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

func (n NetlinkManager) LinkSetUp(link netlink.Link) error { return netlink.LinkSetUp(link) }

func (n NetlinkManager) LinkSetDown(link netlink.Link) error { return netlink.LinkSetDown(link) }

func (n NetlinkManager) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

func (n NetlinkManager) AddrReplace(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrReplace(link, addr)
}

func (n NetlinkManager) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (n NetlinkManager) AddrList(link netlink.Link) ([]netlink.Addr, error) {
	listIPv4, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	listIPv6, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return nil, err
	}

	ipAddresses := make([]netlink.Addr, 0, len(listIPv4)+len(listIPv6))
	ipAddresses = append(ipAddresses, listIPv4...)
	ipAddresses = append(ipAddresses, listIPv6...)

	return ipAddresses, nil
}
