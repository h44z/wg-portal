package wireguard

import (
	"fmt"
	"net"

	"github.com/milosgajdos/tenus"
)

const WireGuardDefaultMTU = 1420

func (m *Manager) GetIPAddress() ([]string, error) {
	wgInterface, err := tenus.NewLinkFrom(m.Cfg.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve WireGuard interface %s: %w", m.Cfg.DeviceName, err)
	}

	// Get golang net.interface
	iface := wgInterface.NetInterface()
	if iface == nil { // Not sure if this check is really necessary
		return nil, fmt.Errorf("could not retrieve WireGuard net.interface: %w", err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve WireGuard ip addresses: %w", err)
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
		if ip == nil {
			continue // something is wrong?
		}

		maskSize, _ := mask.Size()
		cidr := fmt.Sprintf("%s/%d", ip.String(), maskSize)
		ipAddresses = append(ipAddresses, cidr)
	}

	return ipAddresses, nil
}

func (m *Manager) SetIPAddress(cidrs []string) error {
	wgInterface, err := tenus.NewLinkFrom(m.Cfg.DeviceName)
	if err != nil {
		return fmt.Errorf("could not retrieve WireGuard interface %s: %w", m.Cfg.DeviceName, err)
	}

	// First remove existing IP addresses
	existingIPs, err := m.GetIPAddress()
	if err != nil {
		return err
	}
	for _, cidr := range existingIPs {
		wgIp, wgIpNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("unable to parse cidr %s: %w", cidr, err)
		}

		if err := wgInterface.UnsetLinkIp(wgIp, wgIpNet); err != nil {
			return fmt.Errorf("failed to unset ip %s: %w", cidr, err)
		}
	}

	// Next set new IP adrresses
	for _, cidr := range cidrs {
		wgIp, wgIpNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("unable to parse cidr %s: %w", cidr, err)
		}

		if err := wgInterface.SetLinkIp(wgIp, wgIpNet); err != nil {
			return fmt.Errorf("failed to set ip %s: %w", cidr, err)
		}
	}

	return nil
}

func (m *Manager) GetMTU() (int, error) {
	wgInterface, err := tenus.NewLinkFrom(m.Cfg.DeviceName)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve WireGuard interface %s: %w", m.Cfg.DeviceName, err)
	}

	// Get golang net.interface
	iface := wgInterface.NetInterface()
	if iface == nil { // Not sure if this check is really necessary
		return 0, fmt.Errorf("could not retrieve WireGuard net.interface: %w", err)
	}

	return iface.MTU, nil
}

func (m *Manager) SetMTU(mtu int) error {
	wgInterface, err := tenus.NewLinkFrom(m.Cfg.DeviceName)
	if err != nil {
		return fmt.Errorf("could not retrieve WireGuard interface %s: %w", m.Cfg.DeviceName, err)
	}

	if mtu == 0 {
		mtu = WireGuardDefaultMTU
	}

	if err := wgInterface.SetLinkMTU(mtu); err != nil {
		return fmt.Errorf("could not set MTU on interface %s: %w", m.Cfg.DeviceName, err)
	}

	return nil
}
