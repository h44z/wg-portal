package domain

import (
	"net"
	"net/netip"
	"strings"

	"github.com/vishvananda/netlink"
)

type Cidr struct {
	Cidr      string `gorm:"primaryKey;column:cidr"` // Sqlite/GORM does not support composite primary keys...
	Addr      string `gorm:"column:addr"`
	NetLength int    `gorm:"column:net_len"`
}

func (c Cidr) Prefix() netip.Prefix {
	return netip.PrefixFrom(netip.MustParseAddr(c.Addr), c.NetLength)
}

func (c Cidr) String() string {
	return c.Prefix().String()
}

func (c Cidr) IsValid() bool {
	return c.Prefix().IsValid()
}

func CidrFromString(str string) (Cidr, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(str))
	if err != nil {
		return Cidr{}, err
	}
	return CidrFromPrefix(prefix), nil
}

func CidrsFromString(str string) ([]Cidr, error) {
	strParts := strings.Split(str, ",")
	cidrs := make([]Cidr, len(strParts))

	for i, cidrStr := range strParts {
		cidr, err := CidrFromString(cidrStr)
		if err != nil {
			return nil, err
		}
		cidrs[i] = cidr
	}

	return cidrs, nil
}

func CidrsFromArray(strs []string) ([]Cidr, error) {
	cidrs := make([]Cidr, len(strs))

	for i, cidrStr := range strs {
		cidr, err := CidrFromString(cidrStr)
		if err != nil {
			return nil, err
		}
		cidrs[i] = cidr
	}

	return cidrs, nil
}

func CidrFromPrefix(prefix netip.Prefix) Cidr {
	return Cidr{
		Cidr:      prefix.String(),
		Addr:      prefix.Addr().String(),
		NetLength: prefix.Bits(),
	}
}

func CidrFromIpNet(ipNet net.IPNet) Cidr {
	prefix, _ := CidrFromString(ipNet.String())
	return prefix
}

func CidrFromNetlinkAddr(addr netlink.Addr) Cidr {
	prefix, _ := CidrFromString(addr.IPNet.String())
	return prefix
}

func (c Cidr) IpNet() *net.IPNet {
	ip, cidr, _ := net.ParseCIDR(c.String())
	cidr.IP = ip
	return cidr
}

func (c Cidr) NetlinkAddr() *netlink.Addr {
	return &netlink.Addr{
		IPNet: c.IpNet(),
	}
}

func (c Cidr) IsV4() bool {
	return c.Prefix().Addr().Is4()
}

// BroadcastAddr returns the last address in the given network (for IPv6), or the broadcast address.
func (c Cidr) BroadcastAddr() Cidr {
	prefix := c.Prefix()
	if !prefix.IsValid() {
		return Cidr{}
	}
	a16 := prefix.Addr().As16()
	var off uint8
	var bits uint8 = 128
	if prefix.Addr().Is4() {
		off = 12
		bits = 32
	}
	for b := uint8(prefix.Bits()); b < bits; b++ {
		byteNum, bitInByte := b/8, 7-(b%8)
		a16[off+byteNum] |= 1 << uint(bitInByte)
	}
	if prefix.Addr().Is4() {
		addr := netip.AddrFrom16(a16).Unmap()
		return Cidr{
			Cidr:      netip.PrefixFrom(addr, prefix.Bits()).String(),
			Addr:      addr.String(),
			NetLength: prefix.Bits(),
		}
	} else {
		addr := netip.AddrFrom16(a16) // doesn't unmap
		return Cidr{
			Cidr:      netip.PrefixFrom(addr, prefix.Bits()).String(),
			Addr:      addr.String(), // doesn't unmap
			NetLength: prefix.Bits(),
		}
	}
}

// NetworkAddr returns the network address in the given prefix.
func (c Cidr) NetworkAddr() Cidr {
	prefix := c.Prefix()
	if !prefix.IsValid() {
		return Cidr{}
	}

	return CidrFromPrefix(prefix.Masked())
}

func (c Cidr) FirstAddr() Cidr {
	prefix := c.Prefix()
	firstAddr := prefix.Masked().Addr().Next()
	return Cidr{
		Cidr:      netip.PrefixFrom(firstAddr, c.NetLength).String(),
		Addr:      firstAddr.String(),
		NetLength: prefix.Bits(),
	}
}

func (c Cidr) NextAddr() Cidr {
	prefix := c.Prefix()
	nextAddr := prefix.Addr().Next()
	return Cidr{
		Cidr:      netip.PrefixFrom(nextAddr, c.NetLength).String(),
		Addr:      nextAddr.String(),
		NetLength: prefix.Bits(),
	}
}

func (c Cidr) HostAddr() Cidr {
	return Cidr{
		Cidr:      netip.PrefixFrom(c.Prefix().Addr(), c.Prefix().Addr().BitLen()).String(),
		Addr:      c.Addr,
		NetLength: c.Prefix().Addr().BitLen(),
	}
}

func (c Cidr) NextSubnet() Cidr {
	prefix := c.Prefix()
	nextAddr := c.BroadcastAddr().Prefix().Addr().Next()
	return Cidr{
		Cidr:      netip.PrefixFrom(nextAddr, c.NetLength).String(),
		Addr:      nextAddr.String(),
		NetLength: prefix.Bits(),
	}
}

func CidrsToString(slice []Cidr) string {
	return strings.Join(CidrsToStringSlice(slice), ",")
}

func CidrsToStringSlice(slice []Cidr) []string {
	cidrs := make([]string, len(slice))

	for i, cidr := range slice {
		cidrs[i] = cidr.String()
	}

	return cidrs
}

func (c Cidr) Contains(other Cidr) bool {
	_, subnet, _ := net.ParseCIDR(c.String())
	otherIP, _, _ := net.ParseCIDR(other.String())

	return subnet.Contains(otherIP)
}

// ContainsDefaultRoute returns true if the given CIDRs contain a default route.
func ContainsDefaultRoute(cidrs []Cidr) bool {
	for _, allowedIP := range cidrs {
		if allowedIP.Prefix().Bits() == 0 {
			return true
		}
	}

	return false
}

// CidrsPerFamily returns a slice of CIDRs, one for each family (IPv4 and IPv6).
func CidrsPerFamily(cidrs []Cidr) (ipv4, ipv6 []Cidr) {
	for _, cidr := range cidrs {
		if cidr.IsV4() {
			ipv4 = append(ipv4, cidr)
		} else {
			ipv6 = append(ipv6, cidr)
		}
	}
	return
}
