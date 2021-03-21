package wireguard

import (
	"fmt"
	"net"

	"github.com/pkg/errors"

	"github.com/milosgajdos/tenus"
)

const DefaultMTU = 1420

func (m *Manager) GetIPAddress(device string) ([]string, error) {
	wgInterface, err := tenus.NewLinkFrom(device)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve WireGuard interface %s", device)
	}

	// Get golang net.interface
	iface := wgInterface.NetInterface()
	if iface == nil { // Not sure if this check is really necessary
		return nil, errors.Wrap(err, "could not retrieve WireGuard net.interface")
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve WireGuard ip addresses")
	}

	ipAddresses := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		var ip net.IP
		var mask net.IPMask
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
			mask = v.Mask
		case *net.IPAddr:
			ip = v.IP
			mask = ip.DefaultMask()
		}
		if ip == nil || mask == nil {
			continue // something is wrong?
		}

		maskSize, _ := mask.Size()
		cidr := fmt.Sprintf("%s/%d", ip.String(), maskSize)
		ipAddresses = append(ipAddresses, cidr)
	}

	return ipAddresses, nil
}

func (m *Manager) SetIPAddress(device string, cidrs []string) error {
	wgInterface, err := tenus.NewLinkFrom(device)
	if err != nil {
		return errors.Wrapf(err, "could not retrieve WireGuard interface %s", device)
	}

	// First remove existing IP addresses
	existingIPs, err := m.GetIPAddress(device)
	if err != nil {
		return errors.Wrap(err, "could not retrieve IP addresses")
	}
	for _, cidr := range existingIPs {
		wgIp, wgIpNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return errors.Wrapf(err, "unable to parse cidr %s", cidr)
		}

		if err := wgInterface.UnsetLinkIp(wgIp, wgIpNet); err != nil {
			return errors.Wrapf(err, "failed to unset ip %s", cidr)
		}
	}

	// Next set new IP addresses
	for _, cidr := range cidrs {
		wgIp, wgIpNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return errors.Wrapf(err, "unable to parse cidr %s", cidr)
		}

		if err := wgInterface.SetLinkIp(wgIp, wgIpNet); err != nil {
			return errors.Wrapf(err, "failed to set ip %s", cidr)
		}
	}

	return nil
}

func (m *Manager) GetMTU(device string) (int, error) {
	wgInterface, err := tenus.NewLinkFrom(device)
	if err != nil {
		return 0, errors.Wrapf(err, "could not retrieve WireGuard interface %s", device)
	}

	// Get golang net.interface
	iface := wgInterface.NetInterface()
	if iface == nil { // Not sure if this check is really necessary
		return 0, errors.Wrap(err, "could not retrieve WireGuard net.interface")
	}

	return iface.MTU, nil
}

func (m *Manager) SetMTU(device string, mtu int) error {
	wgInterface, err := tenus.NewLinkFrom(device)
	if err != nil {
		return errors.Wrapf(err, "could not retrieve WireGuard interface %s", device)
	}

	if mtu == 0 {
		mtu = DefaultMTU
	}

	if err := wgInterface.SetLinkMTU(mtu); err != nil {
		return errors.Wrapf(err, "could not set MTU on interface %s", device)
	}

	return nil
}
