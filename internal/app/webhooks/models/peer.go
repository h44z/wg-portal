package models

import (
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// Peer represents a peer model for webhooks.  For details about the fields, see the domain.Peer struct.
type Peer struct {
	CreatedBy string    `json:"CreatedBy"`
	UpdatedBy string    `json:"UpdatedBy"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`

	Endpoint            string `json:"Endpoint"`
	EndpointPublicKey   string `json:"EndpointPublicKey"`
	AllowedIPsStr       string `json:"AllowedIPsStr"`
	ExtraAllowedIPsStr  string `json:"ExtraAllowedIPsStr"`
	PresharedKey        string `json:"PresharedKey"`
	PersistentKeepalive int    `json:"PersistentKeepalive"`

	DisplayName          string     `json:"DisplayName"`
	Identifier           string     `json:"Identifier"`
	UserIdentifier       string     `json:"UserIdentifier"`
	InterfaceIdentifier  string     `json:"InterfaceIdentifier"`
	Disabled             *time.Time `json:"Disabled,omitempty"`
	DisabledReason       string     `json:"DisabledReason,omitempty"`
	ExpiresAt            *time.Time `json:"ExpiresAt,omitempty"`
	Notes                string     `json:"Notes,omitempty"`
	AutomaticallyCreated bool       `json:"AutomaticallyCreated"`

	PrivateKey string `json:"PrivateKey"`
	PublicKey  string `json:"PublicKey"`

	InterfaceType string `json:"InterfaceType"`

	Addresses         []string `json:"Addresses"`
	CheckAliveAddress string   `json:"CheckAliveAddress"`
	DnsStr            string   `json:"DnsStr"`
	DnsSearchStr      string   `json:"DnsSearchStr"`
	Mtu               int      `json:"Mtu"`
	FirewallMark      uint32   `json:"FirewallMark,omitempty"`
	RoutingTable      string   `json:"RoutingTable,omitempty"`

	PreUp    string `json:"PreUp,omitempty"`
	PostUp   string `json:"PostUp,omitempty"`
	PreDown  string `json:"PreDown,omitempty"`
	PostDown string `json:"PostDown,omitempty"`
}

// NewPeer creates a new Peer model from a domain.Peer.
func NewPeer(src domain.Peer) Peer {
	return Peer{
		CreatedBy:            src.CreatedBy,
		UpdatedBy:            src.UpdatedBy,
		CreatedAt:            src.CreatedAt,
		UpdatedAt:            src.UpdatedAt,
		Endpoint:             src.Endpoint.GetValue(),
		EndpointPublicKey:    src.EndpointPublicKey.GetValue(),
		AllowedIPsStr:        src.AllowedIPsStr.GetValue(),
		ExtraAllowedIPsStr:   src.ExtraAllowedIPsStr,
		PresharedKey:         string(src.PresharedKey),
		PersistentKeepalive:  src.PersistentKeepalive.GetValue(),
		DisplayName:          src.DisplayName,
		Identifier:           string(src.Identifier),
		UserIdentifier:       string(src.UserIdentifier),
		InterfaceIdentifier:  string(src.InterfaceIdentifier),
		Disabled:             src.Disabled,
		DisabledReason:       src.DisabledReason,
		ExpiresAt:            src.ExpiresAt,
		Notes:                src.Notes,
		AutomaticallyCreated: src.AutomaticallyCreated,
		PrivateKey:           src.Interface.KeyPair.PrivateKey,
		PublicKey:            src.Interface.KeyPair.PublicKey,
		InterfaceType:        string(src.Interface.Type),
		Addresses:            domain.CidrsToStringSlice(src.Interface.Addresses),
		CheckAliveAddress:    src.Interface.CheckAliveAddress,
		DnsStr:               src.Interface.DnsStr.GetValue(),
		DnsSearchStr:         src.Interface.DnsSearchStr.GetValue(),
		Mtu:                  src.Interface.Mtu.GetValue(),
		FirewallMark:         src.Interface.FirewallMark.GetValue(),
		RoutingTable:         src.Interface.RoutingTable.GetValue(),
		PreUp:                src.Interface.PreUp.GetValue(),
		PostUp:               src.Interface.PostUp.GetValue(),
		PreDown:              src.Interface.PreDown.GetValue(),
		PostDown:             src.Interface.PostDown.GetValue(),
	}
}
