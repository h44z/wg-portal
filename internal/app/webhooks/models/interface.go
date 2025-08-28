package models

import (
	"time"

	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// Interface represents an interface model for webhooks. For details about the fields, see the domain.Interface struct.
type Interface struct {
	CreatedBy string    `json:"CreatedBy"`
	UpdatedBy string    `json:"UpdatedBy"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`

	Identifier string `json:"Identifier"`
	PrivateKey string `json:"PrivateKey"`
	PublicKey  string `json:"PublicKey"`
	ListenPort int    `json:"ListenPort"`

	Addresses    []string `json:"Addresses"`
	DnsStr       string   `json:"DnsStr"`
	DnsSearchStr string   `json:"DnsSearchStr"`

	Mtu          int    `json:"Mtu"`
	FirewallMark uint32 `json:"FirewallMark"`
	RoutingTable string `json:"RoutingTable"`

	PreUp    string `json:"PreUp"`
	PostUp   string `json:"PostUp"`
	PreDown  string `json:"PreDown"`
	PostDown string `json:"PostDown"`

	SaveConfig bool `json:"SaveConfig"`

	DisplayName    string     `json:"DisplayName"`
	Type           string     `json:"Type"`
	DriverType     string     `json:"DriverType"`
	Disabled       *time.Time `json:"Disabled,omitempty"`
	DisabledReason string     `json:"DisabledReason,omitempty"`

	PeerDefNetworkStr          string `json:"PeerDefNetworkStr,omitempty"`
	PeerDefDnsStr              string `json:"PeerDefDnsStr,omitempty"`
	PeerDefDnsSearchStr        string `json:"PeerDefDnsSearchStr,omitempty"`
	PeerDefEndpoint            string `json:"PeerDefEndpoint,omitempty"`
	PeerDefAllowedIPsStr       string `json:"PeerDefAllowedIPsStr,omitempty"`
	PeerDefMtu                 int    `json:"PeerDefMtu,omitempty"`
	PeerDefPersistentKeepalive int    `json:"PeerDefPersistentKeepalive,omitempty"`
	PeerDefFirewallMark        uint32 `json:"PeerDefFirewallMark,omitempty"`
	PeerDefRoutingTable        string `json:"PeerDefRoutingTable,omitempty"`

	PeerDefPreUp    string `json:"PeerDefPreUp,omitempty"`
	PeerDefPostUp   string `json:"PeerDefPostUp,omitempty"`
	PeerDefPreDown  string `json:"PeerDefPreDown,omitempty"`
	PeerDefPostDown string `json:"PeerDefPostDown,omitempty"`
}

// NewInterface creates a new Interface model from a domain.Interface.
func NewInterface(src domain.Interface) Interface {
	return Interface{
		CreatedBy:                  src.CreatedBy,
		UpdatedBy:                  src.UpdatedBy,
		CreatedAt:                  src.CreatedAt,
		UpdatedAt:                  src.UpdatedAt,
		Identifier:                 string(src.Identifier),
		PrivateKey:                 src.KeyPair.PrivateKey,
		PublicKey:                  src.KeyPair.PublicKey,
		ListenPort:                 src.ListenPort,
		Addresses:                  domain.CidrsToStringSlice(src.Addresses),
		DnsStr:                     src.DnsStr,
		DnsSearchStr:               src.DnsSearchStr,
		Mtu:                        src.Mtu,
		FirewallMark:               src.FirewallMark,
		RoutingTable:               src.RoutingTable,
		PreUp:                      src.PreUp,
		PostUp:                     src.PostUp,
		PreDown:                    src.PreDown,
		PostDown:                   src.PostDown,
		SaveConfig:                 src.SaveConfig,
		DisplayName:                string(src.Identifier),
		Type:                       string(src.Type),
		DriverType:                 src.DriverType,
		Disabled:                   src.Disabled,
		DisabledReason:             src.DisabledReason,
		PeerDefNetworkStr:          src.PeerDefNetworkStr,
		PeerDefDnsStr:              src.PeerDefDnsStr,
		PeerDefDnsSearchStr:        src.PeerDefDnsSearchStr,
		PeerDefEndpoint:            src.PeerDefEndpoint,
		PeerDefAllowedIPsStr:       src.PeerDefAllowedIPsStr,
		PeerDefMtu:                 src.PeerDefMtu,
		PeerDefPersistentKeepalive: src.PeerDefPersistentKeepalive,
		PeerDefFirewallMark:        src.PeerDefFirewallMark,
		PeerDefRoutingTable:        src.PeerDefRoutingTable,
		PeerDefPreUp:               src.PeerDefPreUp,
		PeerDefPostUp:              src.PeerDefPostUp,
		PeerDefPreDown:             src.PeerDefPreDown,
		PeerDefPostDown:            src.PeerDefPostDown,
	}
}
