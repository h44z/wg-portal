package wireguard

import (
	"bytes"
	"net"
	"sort"
	"strings"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

func (m *wgCtrlManager) GetAllUsedIPs(id persistence.InterfaceIdentifier) ([]*netlink.Addr, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if !m.deviceExists(id) {
		return nil, errors.New("interface does not exist")
	}

	var usedAddresses []*netlink.Addr
	for _, peer := range m.peers[id] {
		addresses, err := parseIpAddressString(peer.Interface.AddressStr.GetValue())
		if err != nil {
			return nil, errors.WithMessagef(err, "unable to parse addresses of peer %s", peer.Identifier)
		}

		usedAddresses = append(usedAddresses, addresses...)
	}

	sort.Slice(usedAddresses, func(i, j int) bool {
		return bytes.Compare(usedAddresses[i].IP, usedAddresses[j].IP) < 0
	})

	return usedAddresses, nil
}

func (m *wgCtrlManager) GetUsedIPs(id persistence.InterfaceIdentifier, subnetCidr string) ([]*netlink.Addr, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if !m.deviceExists(id) {
		return nil, errors.New("interface does not exist")
	}

	subnet, err := parseCIDR(subnetCidr)
	if err != nil {
		return nil, errors.WithMessagef(err, "unable to parse subnet addresses")
	}

	var usedAddresses []*netlink.Addr
	for _, peer := range m.peers[id] {
		addresses, err := parseIpAddressString(peer.Interface.AddressStr.GetValue())
		if err != nil {
			return nil, errors.WithMessagef(err, "unable to parse addresses of peer %s", peer.Identifier)
		}

		for _, address := range addresses {
			if subnet.Contains(address.IP) {
				usedAddresses = append(usedAddresses, address)
			}
		}
	}

	sort.Slice(usedAddresses, func(i, j int) bool {
		return bytes.Compare(usedAddresses[i].IP, usedAddresses[j].IP) < 0
	})

	return usedAddresses, nil
}

func (m *wgCtrlManager) GetFreshIp(id persistence.InterfaceIdentifier, subnetCidr string, increment ...bool) (*netlink.Addr, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if !m.deviceExists(id) {
		return nil, errors.New("interface does not exist")
	}

	subnet, err := parseCIDR(subnetCidr)
	if err != nil {
		return nil, errors.WithMessagef(err, "unable to parse subnet addresses")
	}
	isV4 := isV4(subnet)

	usedIPs, err := m.GetUsedIPs(id, subnetCidr) // highest IP is at the end of the array
	if err != nil {
		return nil, errors.WithMessagef(err, "unable to load used IP addresses")
	}

	// these two addresses are not usable
	broadcastAddr := broadcastAddr(subnet)
	networkAddr := subnet.IP
	// start with the lowest IP and check all others
	ip := &netlink.Addr{
		IPNet: &net.IPNet{IP: subnet.IP.Mask(subnet.Mask).To16(), Mask: subnet.Mask},
	}
	if len(increment) != 0 && increment[0] == true && len(usedIPs) > 0 {
		// start with the maximum used IP and check all above
		ip = &netlink.Addr{
			IPNet: &net.IPNet{IP: make([]byte, 16), Mask: subnet.Mask},
		}
		copy(ip.IP, usedIPs[len(usedIPs)-1].IP)
	}

	for ; subnet.Contains(ip.IP); increaseIP(ip) {
		if bytes.Compare(ip.IP, networkAddr) == 0 {
			continue
		}
		if isV4 && bytes.Compare(ip.IP, broadcastAddr.IP) == 0 {
			continue
		}

		ok := true
		for _, r := range usedIPs {
			if bytes.Compare(ip.IP, r.IP) == 0 {
				ok = false
				break
			}
		}

		if ok {
			return ip, nil
		}
	}

	return nil, errors.New("ip range exceeded")
}

//  http://play.golang.org/p/m8TNTtygK0
func increaseIP(ip *netlink.Addr) {
	for j := len(ip.IP) - 1; j >= 0; j-- {
		ip.IP[j]++
		if ip.IP[j] > 0 {
			break
		}
	}
}

// BroadcastAddr returns the last address in the given network (for IPv6), or the broadcast address.
func broadcastAddr(n *netlink.Addr) *netlink.Addr {
	// The golang net package doesn't make it easy to calculate the broadcast address. :(
	var broadcast = net.IPv6zero
	var mask = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff} // ensure that mask also has 16 bytes (also for IPv4)
	if len(n.Mask) == 4 {
		for i := 0; i < 4; i++ {
			mask[12+i] = n.Mask[i]
		}
	} else {
		for i := 0; i < 16; i++ {
			mask[i] = n.Mask[i]
		}
	}
	for i := 0; i < len(n.IP); i++ {
		broadcast[i] = n.IP[i] | ^mask[i]
	}
	return &netlink.Addr{
		IPNet: &net.IPNet{IP: broadcast, Mask: n.Mask},
	}
}

func isV4(n *netlink.Addr) bool {
	if n.IP.To4() != nil {
		return true
	}

	return false
}

func parseCIDR(cidr string) (*netlink.Addr, error) {
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to parse cidr")
	}

	// Use the 16byte representation for all IP families.
	if len(addr.IP) != 16 {
		addr.IP = addr.IP.To16()
	}

	return addr, nil
}

func parseIpAddressString(addrStr string) ([]*netlink.Addr, error) {
	rawAddresses := strings.Split(addrStr, ",")
	addresses := make([]*netlink.Addr, 0, len(rawAddresses))
	for i := range rawAddresses {
		rawAddress := strings.TrimSpace(rawAddresses[i])
		if rawAddress == "" {
			continue // skip empty string
		}
		address, err := parseCIDR(rawAddress)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse IP address %s", rawAddress)
		}

		addresses = append(addresses, address)
	}

	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i].IP, addresses[j].IP) < 0
	})

	return addresses, nil
}

func ipAddressesToString(addresses []netlink.Addr) string {
	addressesStr := make([]string, len(addresses))
	for i := range addresses {
		addressesStr[i] = addresses[i].String()
	}

	return strings.Join(addressesStr, ",")
}
