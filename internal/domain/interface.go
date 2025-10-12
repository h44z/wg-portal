package domain

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
)

const (
	InterfaceTypeServer InterfaceType = "server"
	InterfaceTypeClient InterfaceType = "client"
	InterfaceTypeAny    InterfaceType = "any"
)

var allowedFileNameRegex = regexp.MustCompile("[^a-zA-Z0-9-_]+")

type InterfaceIdentifier string
type InterfaceType string
type InterfaceBackend string

type Interface struct {
	BaseModel

	// WireGuard specific (for the [interface] section of the config file)

	Identifier InterfaceIdentifier `gorm:"primaryKey"` // device name, for example: wg0
	KeyPair                        // private/public Key of the server interface
	ListenPort int                 // the listening port, for example: 51820

	Addresses    []Cidr `gorm:"many2many:interface_addresses;"` // the interface ip addresses
	DnsStr       string // the dns server that should be set if the interface is up, comma separated
	DnsSearchStr string // the dns search option string that should be set if the interface is up, will be appended to DnsStr

	Mtu          int    // the device MTU
	FirewallMark uint32 // a firewall mark
	RoutingTable string // the routing table number or "off" if the routing table should not be managed

	PreUp    string // action that is executed before the device is up
	PostUp   string // action that is executed after the device is up
	PreDown  string // action that is executed before the device is down
	PostDown string // action that is executed after the device is down

	SaveConfig bool // automatically persist config changes to the wgX.conf file

	// WG Portal specific
	DisplayName    string           // a nice display name/ description for the interface
	Type           InterfaceType    // the interface type, either InterfaceTypeServer or InterfaceTypeClient
	Backend        InterfaceBackend // the backend that is used to manage the interface (wgctrl, mikrotik, ...)
	DriverType     string           // the interface driver type (linux, software, ...)
	Disabled       *time.Time       `gorm:"index"` // flag that specifies if the interface is enabled (up) or not (down)
	DisabledReason string           // the reason why the interface has been disabled

	// Default settings for the peer, used for new peers, those settings will be published to ConfigOption options of
	// the peer config

	PeerDefNetworkStr          string // the default subnets from which peers will get their IP addresses, comma seperated
	PeerDefDnsStr              string // the default dns server for the peer
	PeerDefDnsSearchStr        string // the default dns search options for the peer
	PeerDefEndpoint            string // the default endpoint for the peer
	PeerDefAllowedIPsStr       string // the default allowed IP string for the peer
	PeerDefMtu                 int    // the default device MTU
	PeerDefPersistentKeepalive int    // the default persistent keep-alive Value
	PeerDefFirewallMark        uint32 // default firewall mark
	PeerDefRoutingTable        string // the default routing table

	PeerDefPreUp    string // default action that is executed before the device is up
	PeerDefPostUp   string // default action that is executed after the device is up
	PeerDefPreDown  string // default action that is executed before the device is down
	PeerDefPostDown string // default action that is executed after the device is down
}

// PublicInfo returns a copy of the interface with only the public information.
// Sensible information like keys are not included.
func (i *Interface) PublicInfo() Interface {
	return Interface{
		Identifier:  i.Identifier,
		DisplayName: i.DisplayName,
		Type:        i.Type,
		Disabled:    i.Disabled,
	}
}

// Validate performs checks to ensure that the interface is valid.
func (i *Interface) Validate() error {
	// validate peer default endpoint, add port if needed
	if i.PeerDefEndpoint != "" {
		host, port, err := net.SplitHostPort(i.PeerDefEndpoint)
		switch {
		case err != nil && !strings.Contains(err.Error(), "missing port in address"):
			return fmt.Errorf("invalid default endpoint: %w", err)
		case err != nil && strings.Contains(err.Error(), "missing port in address"):
			// In this case, the entire string is the host, and there's no port.
			host = i.PeerDefEndpoint
			port = strconv.Itoa(i.ListenPort)
		}

		i.PeerDefEndpoint = net.JoinHostPort(host, port)
	}

	return nil
}

func (i *Interface) IsDisabled() bool {
	if i == nil {
		return true
	}
	return i.Disabled != nil
}

func (i *Interface) AddressStr() string {
	return CidrsToString(i.Addresses)
}

func (i *Interface) CopyCalculatedAttributes(src *Interface) {
	i.BaseModel = src.BaseModel
}

func (i *Interface) GetConfigFileName() string {
	filename := allowedFileNameRegex.ReplaceAllString(string(i.Identifier), "")
	filename = internal.TruncateString(filename, 16)
	filename += ".conf"

	return filename
}

// GetAllowedIPs returns the allowed IPs for the interface depending on the interface type and peers.
// For example, if the interface type is Server, the allowed IPs are the IPs of the peers.
// If the interface type is Client, the allowed IPs correspond to the AllowedIPsStr of the peers.
func (i *Interface) GetAllowedIPs(peers []Peer) []Cidr {
	var allowedCidrs []Cidr

	switch i.Type {
	case InterfaceTypeServer, InterfaceTypeAny:
		for _, peer := range peers {
			for _, ip := range peer.Interface.Addresses {
				allowedCidrs = append(allowedCidrs, ip.HostAddr())
			}
			if peer.ExtraAllowedIPsStr != "" {
				extraIPs, err := CidrsFromString(peer.ExtraAllowedIPsStr)
				if err == nil {
					allowedCidrs = append(allowedCidrs, extraIPs...)
				}
			}
		}
	case InterfaceTypeClient:
		for _, peer := range peers {
			allowedIPs, err := CidrsFromString(peer.AllowedIPsStr.GetValue())
			if err == nil {
				allowedCidrs = append(allowedCidrs, allowedIPs...)
			}
		}
	}

	return allowedCidrs
}

func (i *Interface) ManageRoutingTable() bool {
	routingTableStr := strings.ToLower(i.RoutingTable)
	return routingTableStr != "off"
}

// GetRoutingTable returns the routing table number or
//
//	-1 if RoutingTable was set to "off" or an error occurred
func (i *Interface) GetRoutingTable() int {

	routingTableStr := strings.ToLower(i.RoutingTable)
	switch {
	case routingTableStr == "":
		return 0
	case routingTableStr == "off":
		return -1
	case strings.HasPrefix(routingTableStr, "0x"):
		if i.Backend != config.LocalBackendName {
			return 0 // ignore numeric routing table numbers for non-local controllers
		}
		numberStr := strings.ReplaceAll(routingTableStr, "0x", "")
		routingTable, err := strconv.ParseUint(numberStr, 16, 64)
		if err != nil {
			slog.Error("failed to parse routing table number", "table", routingTableStr, "error", err)
			return -1
		}
		if routingTable > math.MaxInt32 {
			slog.Error("routing table number too large", "table", routingTable, "max", math.MaxInt32)
			return -1
		}
		return int(routingTable)
	default:
		if i.Backend != config.LocalBackendName {
			return 0 // ignore numeric routing table numbers for non-local controllers
		}
		routingTable, err := strconv.Atoi(routingTableStr)
		if err != nil {
			slog.Error("failed to parse routing table number", "table", routingTableStr, "error", err)
			return -1
		}
		if routingTable > math.MaxInt32 {
			slog.Error("routing table number too large", "table", routingTable, "max", math.MaxInt32)
			return -1
		}
		return routingTable
	}
}

type PhysicalInterface struct {
	Identifier InterfaceIdentifier // device name, for example: wg0
	KeyPair                        // private/public Key of the server interface
	ListenPort int                 // the listening port, for example: 51820

	Addresses []Cidr // the interface ip addresses

	Mtu          int    // the device MTU
	FirewallMark uint32 // a firewall mark

	DeviceUp bool // device status

	ImportSource string // import source (wgctrl, file, ...)
	DeviceType   string // device type (Linux kernel, userspace, ...)

	BytesUpload   uint64
	BytesDownload uint64

	backendExtras any // additional backend-specific extras, e.g., domain.MikrotikInterfaceExtras
}

func (p *PhysicalInterface) GetExtras() any {
	return p.backendExtras
}

func (p *PhysicalInterface) SetExtras(extras any) {
	switch extras.(type) {
	case MikrotikInterfaceExtras: // OK
	default: // we only support MikrotikInterfaceExtras for now
		panic(fmt.Sprintf("unsupported interface backend extras type %T", extras))
	}

	p.backendExtras = extras
}

func ConvertPhysicalInterface(pi *PhysicalInterface) *Interface {
	networks := make([]Cidr, 0, len(pi.Addresses))
	for _, addr := range pi.Addresses {
		networks = append(networks, addr.NetworkAddr())
	}

	// create a new basic interface with the data from the physical interface
	iface := &Interface{
		Identifier:                 pi.Identifier,
		KeyPair:                    pi.KeyPair,
		ListenPort:                 pi.ListenPort,
		Addresses:                  pi.Addresses,
		DnsStr:                     "",
		DnsSearchStr:               "",
		Mtu:                        pi.Mtu,
		FirewallMark:               pi.FirewallMark,
		RoutingTable:               "",
		PreUp:                      "",
		PostUp:                     "",
		PreDown:                    "",
		PostDown:                   "",
		SaveConfig:                 false,
		DisplayName:                string(pi.Identifier),
		Type:                       InterfaceTypeAny,
		DriverType:                 pi.DeviceType,
		Disabled:                   nil,
		PeerDefNetworkStr:          CidrsToString(networks),
		PeerDefDnsStr:              "",
		PeerDefDnsSearchStr:        "",
		PeerDefEndpoint:            "",
		PeerDefAllowedIPsStr:       CidrsToString(networks),
		PeerDefMtu:                 pi.Mtu,
		PeerDefPersistentKeepalive: 0,
		PeerDefFirewallMark:        0,
		PeerDefRoutingTable:        "",
		PeerDefPreUp:               "",
		PeerDefPostUp:              "",
		PeerDefPreDown:             "",
		PeerDefPostDown:            "",
	}

	if pi.GetExtras() == nil {
		return iface
	}

	// enrich the data with controller-specific extras
	now := time.Now()
	switch pi.ImportSource {
	case ControllerTypeMikrotik:
		extras := pi.GetExtras().(MikrotikInterfaceExtras)
		iface.DisplayName = extras.Comment
		if extras.Disabled {
			iface.Disabled = &now
		} else {
			iface.Disabled = nil
		}
	}

	return iface
}

func MergeToPhysicalInterface(pi *PhysicalInterface, i *Interface) {
	pi.Identifier = i.Identifier
	pi.PublicKey = i.PublicKey
	pi.PrivateKey = i.PrivateKey
	pi.ListenPort = i.ListenPort
	pi.Mtu = i.Mtu
	pi.FirewallMark = i.FirewallMark
	pi.DeviceUp = !i.IsDisabled()
	pi.Addresses = i.Addresses

	switch pi.ImportSource {
	case ControllerTypeMikrotik:
		extras := MikrotikInterfaceExtras{
			Comment:  i.DisplayName,
			Disabled: i.IsDisabled(),
		}
		pi.SetExtras(extras)
	}
}

type RoutingTableInfo struct {
	Interface  Interface
	AllowedIps []Cidr
	FwMark     uint32
	Table      int
	TableStr   string // the routing table number as string (used by mikrotik, linux uses the numeric value)
	IsDeleted  bool   // true if the interface was deleted, false otherwise
}

func (r RoutingTableInfo) String() string {
	v4, v6 := CidrsPerFamily(r.AllowedIps)
	return fmt.Sprintf("%s: fwmark=%d; table=%d; routes_4=%d; routes_6=%d", r.Interface.Identifier, r.FwMark, r.Table,
		len(v4), len(v6))
}

func (r RoutingTableInfo) ManagementEnabled() bool {
	if r.Table == -1 {
		return false
	}

	return true
}

func (r RoutingTableInfo) GetRoutingTable() int {
	if r.Table <= 0 {
		return int(r.FwMark) // use the dynamic routing table which has the same number as the firewall mark
	}

	return r.Table
}

type IpFamily int

const (
	IpFamilyIPv4 IpFamily = unix.AF_INET
	IpFamilyIPv6 IpFamily = unix.AF_INET6
)

func (f IpFamily) String() string {
	switch f {
	case IpFamilyIPv4:
		return "IPv4"
	case IpFamilyIPv6:
		return "IPv6"
	default:
		return "unknown"
	}
}

// RouteRule represents a routing table rule.
type RouteRule struct {
	InterfaceId InterfaceIdentifier
	IpFamily    IpFamily
	FwMark      uint32
	Table       int
	HasDefault  bool
}
