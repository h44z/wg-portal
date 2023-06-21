package model

import (
	"github.com/h44z/wg-portal/internal/domain"
	"time"
)

type Peer struct {
	Identifier          string `json:"Identifier" example:"super_nice_peer"` // peer unique identifier
	DisplayName         string `json:"DisplayName"`                          // a nice display name/ description for the peer
	UserIdentifier      string `json:"UserIdentifier"`                       // the owner
	InterfaceIdentifier string `json:"InterfaceIdentifier"`                  // the interface id
	Disabled            bool   `json:"Disabled"`                             // flag that specifies if the peer is enabled (up) or not (down)
	DisabledReason      string `json:"DisabledReason"`                       // the reason why the peer has been disabled

	Endpoint            string `json:"Endpoint"`            // the endpoint address
	EndpointPublicKey   string `json:"EndpointPublicKey"`   // the endpoint public key
	AllowedIPs          string `json:"AllowedIPs"`          // all allowed ip subnets, comma seperated
	ExtraAllowedIPs     string `json:"ExtraAllowedIPs"`     // all allowed ip subnets on the server side, comma seperated
	PresharedKey        string `json:"PresharedKey"`        // the pre-shared Key of the peer
	PersistentKeepalive int    `json:"PersistentKeepalive"` // the persistent keep-alive interval

	PrivateKey string `json:"PrivateKey" example:"abcdef=="` // private Key of the server peer
	PublicKey  string `json:"PublicKey" example:"abcdef=="`  // public Key of the server peer

	Mode string // the peer interface type (server, client, any)

	Addresses    []string `json:"Addresses"`    // the interface ip addresses
	Dns          string   `json:"Dns"`          // the dns server that should be set if the interface is up, comma separated
	DnsSearch    string   `json:"DnsSearch"`    // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu          int      `json:"Mtu"`          // the device MTU
	FirewallMark int32    `json:"FirewallMark"` // a firewall mark
	RoutingTable string   `json:"RoutingTable"` // the routing table

	PreUp    string `json:"PreUp"`    // action that is executed before the device is up
	PostUp   string `json:"PostUp"`   // action that is executed after the device is up
	PreDown  string `json:"PreDown"`  // action that is executed before the device is down
	PostDown string `json:"PostDown"` // action that is executed after the device is down
}

func NewPeer(src *domain.Peer) *Peer {
	return &Peer{
		Identifier:          string(src.Identifier),
		DisplayName:         src.DisplayName,
		UserIdentifier:      string(src.UserIdentifier),
		InterfaceIdentifier: string(src.InterfaceIdentifier),
		Disabled:            src.IsDisabled(),
		DisabledReason:      src.DisabledReason,
		Endpoint:            src.Endpoint.GetValue(),
		EndpointPublicKey:   src.EndpointPublicKey,
		AllowedIPs:          src.AllowedIPsStr.GetValue(),
		ExtraAllowedIPs:     src.ExtraAllowedIPsStr,
		PresharedKey:        string(src.PresharedKey),
		PersistentKeepalive: src.PersistentKeepalive.GetValue(),
		PrivateKey:          src.Interface.PrivateKey,
		PublicKey:           src.Interface.PublicKey,
		Mode:                string(src.Interface.Type),
		Addresses:           domain.CidrsToStringSlice(src.Interface.Addresses),
		Dns:                 src.Interface.DnsStr.GetValue(),
		DnsSearch:           src.Interface.DnsSearchStr.GetValue(),
		Mtu:                 src.Interface.Mtu.GetValue(),
		FirewallMark:        src.Interface.FirewallMark.GetValue(),
		RoutingTable:        src.Interface.RoutingTable.GetValue(),
		PreUp:               src.Interface.PreUp.GetValue(),
		PostUp:              src.Interface.PostUp.GetValue(),
		PreDown:             src.Interface.PreDown.GetValue(),
		PostDown:            src.Interface.PostDown.GetValue(),
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

	res := &domain.Peer{
		BaseModel:           domain.BaseModel{},
		Endpoint:            domain.StringConfigOption{},
		EndpointPublicKey:   src.EndpointPublicKey,
		AllowedIPsStr:       domain.StringConfigOption{},
		ExtraAllowedIPsStr:  "",
		PresharedKey:        "",
		PersistentKeepalive: domain.IntConfigOption{},
		DisplayName:         "",
		Identifier:          "",
		UserIdentifier:      "",
		InterfaceIdentifier: "",
		Temporary:           nil,
		Disabled:            nil,
		DisabledReason:      "",
		ExpiresAt:           nil,
		Notes:               "",
		Interface:           domain.PeerInterfaceConfig{},
	}

	if src.Disabled {
		res.Disabled = &now
	}

	return res
}
