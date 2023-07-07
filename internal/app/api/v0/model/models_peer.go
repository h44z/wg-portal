package model

import (
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
	"time"
)

type Peer struct {
	Identifier          string     `json:"Identifier" example:"super_nice_peer"` // peer unique identifier
	DisplayName         string     `json:"DisplayName"`                          // a nice display name/ description for the peer
	UserIdentifier      string     `json:"UserIdentifier"`                       // the owner
	InterfaceIdentifier string     `json:"InterfaceIdentifier"`                  // the interface id
	Disabled            bool       `json:"Disabled"`                             // flag that specifies if the peer is enabled (up) or not (down)
	DisabledReason      string     `json:"DisabledReason"`                       // the reason why the peer has been disabled
	ExpiresAt           *time.Time `json:"ExpiresAt"`                            // expiry dates for peers
	Notes               string     `json:"Notes"`                                // a note field for peers

	Endpoint            StringConfigOption      `json:"Endpoint"`            // the endpoint address
	EndpointPublicKey   StringConfigOption      `json:"EndpointPublicKey"`   // the endpoint public key
	AllowedIPs          StringSliceConfigOption `json:"AllowedIPs"`          // all allowed ip subnets, comma seperated
	ExtraAllowedIPs     []string                `json:"ExtraAllowedIPs"`     // all allowed ip subnets on the server side, comma seperated
	PresharedKey        string                  `json:"PresharedKey"`        // the pre-shared Key of the peer
	PersistentKeepalive IntConfigOption         `json:"PersistentKeepalive"` // the persistent keep-alive interval

	PrivateKey string `json:"PrivateKey" example:"abcdef=="` // private Key of the server peer
	PublicKey  string `json:"PublicKey" example:"abcdef=="`  // public Key of the server peer

	Mode string // the peer interface type (server, client, any)

	Addresses         []string                `json:"Addresses"`         // the interface ip addresses
	CheckAliveAddress string                  `json:"CheckAliveAddress"` // optional ip address or DNS name that is used for ping checks
	Dns               StringSliceConfigOption `json:"Dns"`               // the dns server that should be set if the interface is up, comma separated
	DnsSearch         StringSliceConfigOption `json:"DnsSearch"`         // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu               IntConfigOption         `json:"Mtu"`               // the device MTU
	FirewallMark      Int32ConfigOption       `json:"FirewallMark"`      // a firewall mark
	RoutingTable      StringConfigOption      `json:"RoutingTable"`      // the routing table

	PreUp    StringConfigOption `json:"PreUp"`    // action that is executed before the device is up
	PostUp   StringConfigOption `json:"PostUp"`   // action that is executed after the device is up
	PreDown  StringConfigOption `json:"PreDown"`  // action that is executed before the device is down
	PostDown StringConfigOption `json:"PostDown"` // action that is executed after the device is down
}

func NewPeer(src *domain.Peer) *Peer {
	return &Peer{
		Identifier:          string(src.Identifier),
		DisplayName:         src.DisplayName,
		UserIdentifier:      string(src.UserIdentifier),
		InterfaceIdentifier: string(src.InterfaceIdentifier),
		Disabled:            src.IsDisabled(),
		DisabledReason:      src.DisabledReason,
		ExpiresAt:           src.ExpiresAt,
		Notes:               src.Notes,
		Endpoint:            StringConfigOptionFromDomain(src.Endpoint),
		EndpointPublicKey:   StringConfigOptionFromDomain(src.EndpointPublicKey),
		AllowedIPs:          StringSliceConfigOptionFromDomain(src.AllowedIPsStr),
		ExtraAllowedIPs:     internal.SliceString(src.ExtraAllowedIPsStr),
		PresharedKey:        string(src.PresharedKey),
		PersistentKeepalive: IntConfigOptionFromDomain(src.PersistentKeepalive),
		PrivateKey:          src.Interface.PrivateKey,
		PublicKey:           src.Interface.PublicKey,
		Mode:                string(src.Interface.Type),
		Addresses:           domain.CidrsToStringSlice(src.Interface.Addresses),
		CheckAliveAddress:   src.Interface.CheckAliveAddress,
		Dns:                 StringSliceConfigOptionFromDomain(src.Interface.DnsStr),
		DnsSearch:           StringSliceConfigOptionFromDomain(src.Interface.DnsSearchStr),
		Mtu:                 IntConfigOptionFromDomain(src.Interface.Mtu),
		FirewallMark:        Int32ConfigOptionFromDomain(src.Interface.FirewallMark),
		RoutingTable:        StringConfigOptionFromDomain(src.Interface.RoutingTable),
		PreUp:               StringConfigOptionFromDomain(src.Interface.PreUp),
		PostUp:              StringConfigOptionFromDomain(src.Interface.PostUp),
		PreDown:             StringConfigOptionFromDomain(src.Interface.PreDown),
		PostDown:            StringConfigOptionFromDomain(src.Interface.PostDown),
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

	res := &domain.Peer{
		BaseModel:           domain.BaseModel{},
		Endpoint:            StringConfigOptionToDomain(src.Endpoint),
		EndpointPublicKey:   StringConfigOptionToDomain(src.EndpointPublicKey),
		AllowedIPsStr:       StringSliceConfigOptionToDomain(src.AllowedIPs),
		ExtraAllowedIPsStr:  internal.SliceToString(src.ExtraAllowedIPs),
		PresharedKey:        domain.PreSharedKey(src.PresharedKey),
		PersistentKeepalive: IntConfigOptionToDomain(src.PersistentKeepalive),
		DisplayName:         src.DisplayName,
		Identifier:          domain.PeerIdentifier(src.Identifier),
		UserIdentifier:      domain.UserIdentifier(src.UserIdentifier),
		InterfaceIdentifier: domain.InterfaceIdentifier(src.InterfaceIdentifier),
		Disabled:            nil, // set below
		DisabledReason:      src.DisabledReason,
		ExpiresAt:           src.ExpiresAt,
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
			Mtu:               IntConfigOptionToDomain(src.Mtu),
			FirewallMark:      Int32ConfigOptionToDomain(src.FirewallMark),
			RoutingTable:      StringConfigOptionToDomain(src.RoutingTable),
			PreUp:             StringConfigOptionToDomain(src.PreUp),
			PostUp:            StringConfigOptionToDomain(src.PostUp),
			PreDown:           StringConfigOptionToDomain(src.PreDown),
			PostDown:          StringConfigOptionToDomain(src.PostDown),
		},
	}

	if src.Disabled {
		res.Disabled = &now
	}

	return res
}

type MultiPeerRequest struct {
	Identifiers []string `json:"Identifiers"`
	Suffix      string   `json:"Suffix"`
}

func NewDomainPeerCreationRequest(src *MultiPeerRequest) *domain.PeerCreationRequest {
	return &domain.PeerCreationRequest{
		Identifiers: src.Identifiers,
		Suffix:      src.Suffix,
	}
}
