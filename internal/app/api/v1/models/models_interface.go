package models

import (
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
)

// Interface represents a WireGuard interface.
type Interface struct {
	// Identifier is the unique identifier of the interface. It is always equal to the device name of the interface.
	Identifier string `json:"Identifier" example:"wg0" binding:"required"`
	// DisplayName is a nice display name / description for the interface.
	DisplayName string `json:"DisplayName" binding:"omitempty,max=64" example:"My Interface"`
	// Mode is the interface type, either 'server', 'client' or 'any'. The mode specifies how WireGuard Portal handles peers for this interface.
	Mode string `json:"Mode" example:"server" binding:"required,oneof=server client any"`
	// PrivateKey is the private key of the interface.
	PrivateKey string `json:"PrivateKey" example:"gI6EdUSYvn8ugXOt8QQD6Yc+JyiZxIhp3GInSWRfWGE=" binding:"required,len=44"`
	// PublicKey is the public key of the server interface. The public key is used by peers to connect to the server.
	PublicKey string `json:"PublicKey" example:"HIgo9xNzJMWLKASShiTqIybxZ0U3wGLiUeJ1PKf8ykw=" binding:"required,len=44"`
	// Disabled is a flag that specifies if the interface is enabled (up) or not (down). Disabled interfaces are not able to accept connections.
	Disabled bool `json:"Disabled" example:"false"`
	// DisabledReason is the reason why the interface has been disabled.
	DisabledReason string `json:"DisabledReason" binding:"required_if=Disabled true" example:"This is a reason why the interface has been disabled."`
	// SaveConfig is a flag that specifies if the configuration should be saved to the configuration file (wgX.conf in wg-quick format).
	SaveConfig bool `json:"SaveConfig" example:"false"`

	// ListenPort is the listening port, for example: 51820. The listening port is only required for server interfaces.
	ListenPort int `json:"ListenPort" binding:"omitempty,min=1,max=65535" example:"51820"`
	// Addresses is a list of IP addresses (in CIDR format) that are assigned to the interface.
	Addresses []string `json:"Addresses" binding:"omitempty,dive,cidr" example:"10.11.12.1/24"`
	// Dns is a list of DNS servers that should be set if the interface is up.
	Dns []string `json:"Dns" binding:"omitempty,dive,ip" example:"1.1.1.1"`
	// DnsSearch is the dns search option string that should be set if the interface is up, will be appended to Dns servers.
	DnsSearch []string `json:"DnsSearch" binding:"omitempty,dive,fqdn" example:"wg.local"`
	// Mtu is the device MTU of the interface.
	Mtu int `json:"Mtu" binding:"omitempty,min=1,max=9000" example:"1420"`
	// FirewallMark is an optional firewall mark which is used to handle interface traffic.
	FirewallMark uint32 `json:"FirewallMark"`
	// RoutingTable is an optional routing table which is used to route interface traffic.
	RoutingTable string `json:"RoutingTable"`

	// PreUp is an optional action that is executed before the device is up.
	PreUp string `json:"PreUp" example:"echo 'Interface is up'"`
	// PostUp is an optional action that is executed after the device is up.
	PostUp string `json:"PostUp" example:"iptables -A FORWARD -i %i -j ACCEPT"`
	// PreDown is an optional action that is executed before the device is down.
	PreDown string `json:"PreDown" example:"iptables -D FORWARD -i %i -j ACCEPT"`
	// PostDown is an optional action that is executed after the device is down.
	PostDown string `json:"PostDown" example:"echo 'Interface is down'"`

	// PeerDefNetwork specifies the default subnets from which new peers will get their IP addresses. The subnet is specified in CIDR format.
	PeerDefNetwork []string `json:"PeerDefNetwork" example:"10.11.12.0/24"`
	// PeerDefDns specifies the default dns servers for a new peer.
	PeerDefDns []string `json:"PeerDefDns" example:"8.8.8.8"`
	// PeerDefDnsSearch specifies the default dns search options for a new peer.
	PeerDefDnsSearch []string `json:"PeerDefDnsSearch" example:"wg.local"`
	// PeerDefEndpoint specifies the default endpoint for a new peer.
	PeerDefEndpoint string `json:"PeerDefEndpoint" example:"wg.example.com:51820"`
	// PeerDefAllowedIPs specifies the default allowed IP addresses for a new peer.
	PeerDefAllowedIPs []string `json:"PeerDefAllowedIPs" example:"10.11.12.0/24"`
	// PeerDefMtu specifies the default device MTU for a new peer.
	PeerDefMtu int `json:"PeerDefMtu" example:"1420"`
	// PeerDefPersistentKeepalive specifies the default persistent keep-alive value in seconds for a new peer.
	PeerDefPersistentKeepalive int `json:"PeerDefPersistentKeepalive" example:"25"`
	// PeerDefFirewallMark specifies the default firewall mark for a new peer.
	PeerDefFirewallMark uint32 `json:"PeerDefFirewallMark"`
	// PeerDefRoutingTable specifies the default routing table for a new peer.
	PeerDefRoutingTable string `json:"PeerDefRoutingTable"`

	// PeerDefPreUp specifies the default action that is executed before the device is up for a new peer.
	PeerDefPreUp string `json:"PeerDefPreUp"`
	// PeerDefPostUp specifies the default action that is executed after the device is up for a new peer.
	PeerDefPostUp string `json:"PeerDefPostUp"`
	// PeerDefPreDown specifies the default action that is executed before the device is down for a new peer.
	PeerDefPreDown string `json:"PeerDefPreDown"`
	// PeerDefPostDown specifies the default action that is executed after the device is down for a new peer.
	PeerDefPostDown string `json:"PeerDefPostDown"`

	// Calculated values

	// EnabledPeers is the number of enabled peers for this interface. Only enabled peers are able to connect.
	EnabledPeers int `json:"EnabledPeers" readonly:"true"`
	// TotalPeers is the total number of peers for this interface.
	TotalPeers int `json:"TotalPeers" readonly:"true"`
	// Filename is the name of the config file for this interface.
	// This value is read only and is not settable by the user.
	Filename string `json:"Filename" example:"wg0.conf" binding:"omitempty,max=21" readonly:"true"`
}

func NewInterface(src *domain.Interface, peers []domain.Peer) *Interface {
	iface := &Interface{
		Identifier:                 string(src.Identifier),
		DisplayName:                src.DisplayName,
		Mode:                       string(src.Type),
		PrivateKey:                 src.PrivateKey,
		PublicKey:                  src.PublicKey,
		Disabled:                   src.IsDisabled(),
		DisabledReason:             src.DisabledReason,
		SaveConfig:                 src.SaveConfig,
		ListenPort:                 src.ListenPort,
		Addresses:                  domain.CidrsToStringSlice(src.Addresses),
		Dns:                        internal.SliceString(src.DnsStr),
		DnsSearch:                  internal.SliceString(src.DnsSearchStr),
		Mtu:                        src.Mtu,
		FirewallMark:               src.FirewallMark,
		RoutingTable:               src.RoutingTable,
		PreUp:                      src.PreUp,
		PostUp:                     src.PostUp,
		PreDown:                    src.PreDown,
		PostDown:                   src.PostDown,
		PeerDefNetwork:             internal.SliceString(src.PeerDefNetworkStr),
		PeerDefDns:                 internal.SliceString(src.PeerDefDnsStr),
		PeerDefDnsSearch:           internal.SliceString(src.PeerDefDnsSearchStr),
		PeerDefEndpoint:            src.PeerDefEndpoint,
		PeerDefAllowedIPs:          internal.SliceString(src.PeerDefAllowedIPsStr),
		PeerDefMtu:                 src.PeerDefMtu,
		PeerDefPersistentKeepalive: src.PeerDefPersistentKeepalive,
		PeerDefFirewallMark:        src.PeerDefFirewallMark,
		PeerDefRoutingTable:        src.PeerDefRoutingTable,
		PeerDefPreUp:               src.PeerDefPreUp,
		PeerDefPostUp:              src.PeerDefPostUp,
		PeerDefPreDown:             src.PeerDefPreDown,
		PeerDefPostDown:            src.PeerDefPostDown,

		EnabledPeers: 0,
		TotalPeers:   0,
		Filename:     src.GetConfigFileName(),
	}

	if len(peers) > 0 {
		iface.TotalPeers = len(peers)

		activePeers := 0
		for _, peer := range peers {
			if !peer.IsDisabled() {
				activePeers++
			}
		}
		iface.EnabledPeers = activePeers
	}

	return iface
}

func NewInterfaces(src []domain.Interface, srcPeers [][]domain.Peer) []Interface {
	results := make([]Interface, len(src))
	for i := range src {
		results[i] = *NewInterface(&src[i], srcPeers[i])
	}

	return results
}

func NewDomainInterface(src *Interface) *domain.Interface {
	now := time.Now()

	cidrs, _ := domain.CidrsFromArray(src.Addresses)

	res := &domain.Interface{
		BaseModel:  domain.BaseModel{},
		Identifier: domain.InterfaceIdentifier(src.Identifier),
		KeyPair: domain.KeyPair{
			PrivateKey: src.PrivateKey,
			PublicKey:  src.PublicKey,
		},
		ListenPort:                 src.ListenPort,
		Addresses:                  cidrs,
		DnsStr:                     internal.SliceToString(src.Dns),
		DnsSearchStr:               internal.SliceToString(src.DnsSearch),
		Mtu:                        src.Mtu,
		FirewallMark:               src.FirewallMark,
		RoutingTable:               src.RoutingTable,
		PreUp:                      src.PreUp,
		PostUp:                     src.PostUp,
		PreDown:                    src.PreDown,
		PostDown:                   src.PostDown,
		SaveConfig:                 src.SaveConfig,
		DisplayName:                src.DisplayName,
		Type:                       domain.InterfaceType(src.Mode),
		DriverType:                 "",  // currently unused
		Disabled:                   nil, // set below
		DisabledReason:             src.DisabledReason,
		PeerDefNetworkStr:          internal.SliceToString(src.PeerDefNetwork),
		PeerDefDnsStr:              internal.SliceToString(src.PeerDefDns),
		PeerDefDnsSearchStr:        internal.SliceToString(src.PeerDefDnsSearch),
		PeerDefEndpoint:            src.PeerDefEndpoint,
		PeerDefAllowedIPsStr:       internal.SliceToString(src.PeerDefAllowedIPs),
		PeerDefMtu:                 src.PeerDefMtu,
		PeerDefPersistentKeepalive: src.PeerDefPersistentKeepalive,
		PeerDefFirewallMark:        src.PeerDefFirewallMark,
		PeerDefRoutingTable:        src.PeerDefRoutingTable,
		PeerDefPreUp:               src.PeerDefPreUp,
		PeerDefPostUp:              src.PeerDefPostUp,
		PeerDefPreDown:             src.PeerDefPreDown,
		PeerDefPostDown:            src.PeerDefPostDown,
	}

	if src.Disabled {
		res.Disabled = &now
	}

	return res
}
