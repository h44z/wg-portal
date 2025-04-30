package models

import (
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
)

const ExpiryDateTimeLayout = "2006-01-02T15:04:05"

// Peer represents a WireGuard peer entry.
type Peer struct {
	// Identifier is the unique identifier of the peer. It is always equal to the public key of the peer.
	Identifier string `json:"Identifier" example:"xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=" binding:"required,len=44"`
	// DisplayName is a nice display name / description for the peer.
	DisplayName string `json:"DisplayName" example:"My Peer" binding:"omitempty,max=64"`
	// UserIdentifier is the identifier of the user that owns the peer.
	UserIdentifier string `json:"UserIdentifier" example:"uid-1234567"`
	// InterfaceIdentifier is the identifier of the interface the peer is linked to.
	InterfaceIdentifier string `json:"InterfaceIdentifier" binding:"required" example:"wg0"`
	// Disabled is a flag that specifies if the peer is enabled or not. Disabled peers are not able to connect.
	Disabled bool `json:"Disabled" example:"false"`
	// DisabledReason is the reason why the peer has been disabled.
	DisabledReason string `json:"DisabledReason" binding:"required_if=Disabled true" example:"This is a reason why the peer has been disabled."`
	// ExpiresAt is the expiry date of the peer  in YYYY-MM-DD format. An expired peer is not able to connect.
	ExpiresAt string `json:"ExpiresAt,omitempty" binding:"omitempty,datetime=2006-01-02T15:04:05"`
	// Notes is a note field for peers.
	Notes string `json:"Notes" example:"This is a note for the peer."`

	// Endpoint is the endpoint address of the peer.
	Endpoint ConfigOption[string] `json:"Endpoint"`
	// EndpointPublicKey is the endpoint public key.
	EndpointPublicKey ConfigOption[string] `json:"EndpointPublicKey"`
	// AllowedIPs is a list of allowed IP subnets for the peer.
	AllowedIPs ConfigOption[[]string] `json:"AllowedIPs"`
	// ExtraAllowedIPs is a list of additional allowed IP subnets for the peer. These allowed IP subnets are added on the server side.
	ExtraAllowedIPs []string `json:"ExtraAllowedIPs"`
	// PresharedKey is the optional pre-shared Key of the peer.
	PresharedKey string `json:"PresharedKey" example:"yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=" binding:"omitempty,len=44"`
	// PersistentKeepalive is the optional persistent keep-alive interval in seconds.
	PersistentKeepalive ConfigOption[int] `json:"PersistentKeepalive"`

	// PrivateKey is the private Key of the peer.
	PrivateKey string `json:"PrivateKey" example:"yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=" binding:"required,len=44"`
	// PublicKey is the public Key of the server peer.
	PublicKey string `json:"PublicKey" example:"TrMvSoP4jYQlY6RIzBgbssQqY3vxI2Pi+y71lOWWXX0=" binding:"omitempty,len=44"`

	// Mode is the peer interface type (server, client, any).
	Mode string `json:"Mode" example:"client" binding:"omitempty,oneof=server client any"`

	// Addresses is a list of IP addresses in CIDR format (both IPv4 and IPv6) for the peer.
	Addresses []string `json:"Addresses" example:"10.11.12.2/24" binding:"omitempty,dive,cidr"`
	// CheckAliveAddress is an optional ip address or DNS name that is used for ping checks.
	CheckAliveAddress string `json:"CheckAliveAddress" binding:"omitempty,ip|fqdn" example:"1.1.1.1"`
	// Dns is a list of DNS servers that should be set if the peer interface is up.
	Dns ConfigOption[[]string] `json:"Dns"`
	// DnsSearch is the dns search option string that should be set if the peer interface is up, will be appended to Dns servers.
	DnsSearch ConfigOption[[]string] `json:"DnsSearch"`
	// Mtu is the device MTU of the peer.
	Mtu ConfigOption[int] `json:"Mtu"`
	// FirewallMark is an optional firewall mark which is used to handle peer traffic.
	FirewallMark ConfigOption[uint32] `json:"FirewallMark"`
	// RoutingTable is an optional routing table which is used to route peer traffic.
	RoutingTable ConfigOption[string] `json:"RoutingTable"`

	// PreUp is an optional action that is executed before the device is up.
	PreUp ConfigOption[string] `json:"PreUp"`
	// PostUp is an optional action that is executed after the device is up.
	PostUp ConfigOption[string] `json:"PostUp"`
	// PreDown is an optional action that is executed before the device is down.
	PreDown ConfigOption[string] `json:"PreDown"`
	// PostDown is an optional action that is executed after the device is down.
	PostDown ConfigOption[string] `json:"PostDown"`

	// Filename is the name of the config file for this peer.
	// This value is read only and is not settable by the user.
	Filename string `json:"Filename" example:"wg_peer_x.conf" binding:"omitempty,max=21" readonly:"true"`
}

func NewPeer(src *domain.Peer) *Peer {
	expiresAt := ""
	if src.ExpiresAt != nil && !src.ExpiresAt.IsZero() {
		expiresAt = src.ExpiresAt.Format(ExpiryDateTimeLayout)
	}

	return &Peer{
		Identifier:          string(src.Identifier),
		DisplayName:         src.DisplayName,
		UserIdentifier:      string(src.UserIdentifier),
		InterfaceIdentifier: string(src.InterfaceIdentifier),
		Disabled:            src.IsDisabled(),
		DisabledReason:      src.DisabledReason,
		ExpiresAt:           expiresAt,
		Notes:               src.Notes,
		Endpoint:            ConfigOptionFromDomain(src.Endpoint),
		EndpointPublicKey:   ConfigOptionFromDomain(src.EndpointPublicKey),
		AllowedIPs:          StringSliceConfigOptionFromDomain(src.AllowedIPsStr),
		ExtraAllowedIPs:     internal.SliceString(src.ExtraAllowedIPsStr),
		PresharedKey:        string(src.PresharedKey),
		PersistentKeepalive: ConfigOptionFromDomain(src.PersistentKeepalive),
		PrivateKey:          src.Interface.PrivateKey,
		PublicKey:           src.Interface.PublicKey,
		Mode:                string(src.Interface.Type),
		Addresses:           domain.CidrsToStringSlice(src.Interface.Addresses),
		CheckAliveAddress:   src.Interface.CheckAliveAddress,
		Dns:                 StringSliceConfigOptionFromDomain(src.Interface.DnsStr),
		DnsSearch:           StringSliceConfigOptionFromDomain(src.Interface.DnsSearchStr),
		Mtu:                 ConfigOptionFromDomain(src.Interface.Mtu),
		FirewallMark:        ConfigOptionFromDomain(src.Interface.FirewallMark),
		RoutingTable:        ConfigOptionFromDomain(src.Interface.RoutingTable),
		PreUp:               ConfigOptionFromDomain(src.Interface.PreUp),
		PostUp:              ConfigOptionFromDomain(src.Interface.PostUp),
		PreDown:             ConfigOptionFromDomain(src.Interface.PreDown),
		PostDown:            ConfigOptionFromDomain(src.Interface.PostDown),
		Filename:            src.GetConfigFileName(),
	}
}

func NewPeers(src []domain.Peer) []Peer {
	results := make([]Peer, len(src))
	for i := range src {
		results[i] = *NewPeer(&src[i])
	}

	return results
}

func NewDomainPeer(src *Peer) *domain.Peer {
	now := time.Now()

	cidrs, _ := domain.CidrsFromArray(src.Addresses)
	var expiresAt *time.Time
	if src.ExpiresAt != "" {
		if t, err := time.Parse(ExpiryDateTimeLayout, src.ExpiresAt); err == nil {
			expiresAt = &t
		}
	}

	res := &domain.Peer{
		BaseModel:           domain.BaseModel{},
		Endpoint:            ConfigOptionToDomain(src.Endpoint),
		EndpointPublicKey:   ConfigOptionToDomain(src.EndpointPublicKey),
		AllowedIPsStr:       StringSliceConfigOptionToDomain(src.AllowedIPs),
		ExtraAllowedIPsStr:  internal.SliceToString(src.ExtraAllowedIPs),
		PresharedKey:        domain.PreSharedKey(src.PresharedKey),
		PersistentKeepalive: ConfigOptionToDomain(src.PersistentKeepalive),
		DisplayName:         src.DisplayName,
		Identifier:          domain.PeerIdentifier(src.Identifier),
		UserIdentifier:      domain.UserIdentifier(src.UserIdentifier),
		InterfaceIdentifier: domain.InterfaceIdentifier(src.InterfaceIdentifier),
		Disabled:            nil, // set below
		DisabledReason:      src.DisabledReason,
		ExpiresAt:           expiresAt,
		Notes:               src.Notes,
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{
				PrivateKey: src.PrivateKey,
				PublicKey:  src.PublicKey,
			},
			Type:              domain.InterfaceType(src.Mode),
			Addresses:         cidrs,
			CheckAliveAddress: src.CheckAliveAddress,
			DnsStr:            StringSliceConfigOptionToDomain(src.Dns),
			DnsSearchStr:      StringSliceConfigOptionToDomain(src.DnsSearch),
			Mtu:               ConfigOptionToDomain(src.Mtu),
			FirewallMark:      ConfigOptionToDomain(src.FirewallMark),
			RoutingTable:      ConfigOptionToDomain(src.RoutingTable),
			PreUp:             ConfigOptionToDomain(src.PreUp),
			PostUp:            ConfigOptionToDomain(src.PostUp),
			PreDown:           ConfigOptionToDomain(src.PreDown),
			PostDown:          ConfigOptionToDomain(src.PostDown),
		},
	}

	if src.Disabled {
		res.Disabled = &now
	}

	return res
}
