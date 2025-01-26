package model

import (
	"time"

	"github.com/h44z/wg-portal/internal"

	"github.com/h44z/wg-portal/internal/domain"
)

type Interface struct {
	Identifier     string `json:"Identifier" example:"wg0"`      // device name, for example: wg0
	DisplayName    string `json:"DisplayName"`                   // a nice display name/ description for the interface
	Mode           string `json:"Mode" example:"server"`         // the interface type, either 'server', 'client' or 'any'
	PrivateKey     string `json:"PrivateKey" example:"abcdef=="` // private Key of the server interface
	PublicKey      string `json:"PublicKey" example:"abcdef=="`  // public Key of the server interface
	Disabled       bool   `json:"Disabled"`                      // flag that specifies if the interface is enabled (up) or not (down)
	DisabledReason string `json:"DisabledReason"`                // the reason why the interface has been disabled
	SaveConfig     bool   `json:"SaveConfig"`                    // automatically persist config changes to the wgX.conf file

	ListenPort   int      `json:"ListenPort"`   // the listening port, for example: 51820
	Addresses    []string `json:"Addresses"`    // the interface ip addresses
	Dns          []string `json:"Dns"`          // the dns server that should be set if the interface is up, comma separated
	DnsSearch    []string `json:"DnsSearch"`    // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu          int      `json:"Mtu"`          // the device MTU
	FirewallMark uint32   `json:"FirewallMark"` // a firewall mark
	RoutingTable string   `json:"RoutingTable"` // the routing table

	PreUp    string `json:"PreUp"`    // action that is executed before the device is up
	PostUp   string `json:"PostUp"`   // action that is executed after the device is up
	PreDown  string `json:"PreDown"`  // action that is executed before the device is down
	PostDown string `json:"PostDown"` // action that is executed after the device is down

	PeerDefNetwork             []string `json:"PeerDefNetwork"`             // the default subnets from which peers will get their IP addresses, comma seperated
	PeerDefDns                 []string `json:"PeerDefDns"`                 // the default dns server for the peer
	PeerDefDnsSearch           []string `json:"PeerDefDnsSearch"`           // the default dns search options for the peer
	PeerDefEndpoint            string   `json:"PeerDefEndpoint"`            // the default endpoint for the peer
	PeerDefAllowedIPs          []string `json:"PeerDefAllowedIPs"`          // the default allowed IP string for the peer
	PeerDefMtu                 int      `json:"PeerDefMtu"`                 // the default device MTU
	PeerDefPersistentKeepalive int      `json:"PeerDefPersistentKeepalive"` // the default persistent keep-alive Value
	PeerDefFirewallMark        uint32   `json:"PeerDefFirewallMark"`        // default firewall mark
	PeerDefRoutingTable        string   `json:"PeerDefRoutingTable"`        // the default routing table

	PeerDefPreUp    string `json:"PeerDefPreUp"`    // default action that is executed before the device is up
	PeerDefPostUp   string `json:"PeerDefPostUp"`   // default action that is executed after the device is up
	PeerDefPreDown  string `json:"PeerDefPreDown"`  // default action that is executed before the device is down
	PeerDefPostDown string `json:"PeerDefPostDown"` // default action that is executed after the device is down

	// Calculated values

	EnabledPeers int `json:"EnabledPeers"`
	TotalPeers   int `json:"TotalPeers"`
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
		if srcPeers == nil {
			results[i] = *NewInterface(&src[i], nil)
		} else {
			results[i] = *NewInterface(&src[i], srcPeers[i])
		}
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
