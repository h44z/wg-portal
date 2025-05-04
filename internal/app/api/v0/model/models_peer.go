package model

import (
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
)

const ExpiryDateTimeLayout = "\"2006-01-02\""

type ExpiryDate struct {
	*time.Time
}

// UnmarshalJSON will unmarshal using 2006-01-02 layout
func (d *ExpiryDate) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" || string(b) == "\"\"" {
		return nil
	}
	parsed, err := time.Parse(ExpiryDateTimeLayout, string(b))
	if err != nil {
		return err
	}

	if !parsed.IsZero() {
		d.Time = &parsed
	}
	return nil
}

// MarshalJSON will marshal using 2006-01-02 layout
func (d *ExpiryDate) MarshalJSON() ([]byte, error) {
	if d == nil || d.Time == nil {
		return []byte("null"), nil
	}

	s := d.Format(ExpiryDateTimeLayout)
	return []byte(s), nil
}

type Peer struct {
	Identifier          string     `json:"Identifier" example:"super_nice_peer"` // peer unique identifier
	DisplayName         string     `json:"DisplayName"`                          // a nice display name/ description for the peer
	UserIdentifier      string     `json:"UserIdentifier"`                       // the owner
	InterfaceIdentifier string     `json:"InterfaceIdentifier"`                  // the interface id
	Disabled            bool       `json:"Disabled"`                             // flag that specifies if the peer is enabled (up) or not (down)
	DisabledReason      string     `json:"DisabledReason"`                       // the reason why the peer has been disabled
	ExpiresAt           ExpiryDate `json:"ExpiresAt,omitempty"`                  // expiry dates for peers
	Notes               string     `json:"Notes"`                                // a note field for peers

	Endpoint            ConfigOption[string]   `json:"Endpoint"`            // the endpoint address
	EndpointPublicKey   ConfigOption[string]   `json:"EndpointPublicKey"`   // the endpoint public key
	AllowedIPs          ConfigOption[[]string] `json:"AllowedIPs"`          // all allowed ip subnets, comma seperated
	ExtraAllowedIPs     []string               `json:"ExtraAllowedIPs"`     // all allowed ip subnets on the server side, comma seperated
	PresharedKey        string                 `json:"PresharedKey"`        // the pre-shared Key of the peer
	PersistentKeepalive ConfigOption[int]      `json:"PersistentKeepalive"` // the persistent keep-alive interval

	PrivateKey string `json:"PrivateKey" example:"abcdef=="` // private Key of the server peer
	PublicKey  string `json:"PublicKey" example:"abcdef=="`  // public Key of the server peer

	Mode string // the peer interface type (server, client, any)

	Addresses         []string               `json:"Addresses"`         // the interface ip addresses
	CheckAliveAddress string                 `json:"CheckAliveAddress"` // optional ip address or DNS name that is used for ping checks
	Dns               ConfigOption[[]string] `json:"Dns"`               // the dns server that should be set if the interface is up, comma separated
	DnsSearch         ConfigOption[[]string] `json:"DnsSearch"`         // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu               ConfigOption[int]      `json:"Mtu"`               // the device MTU
	FirewallMark      ConfigOption[uint32]   `json:"FirewallMark"`      // a firewall mark
	RoutingTable      ConfigOption[string]   `json:"RoutingTable"`      // the routing table

	PreUp    ConfigOption[string] `json:"PreUp"`    // action that is executed before the device is up
	PostUp   ConfigOption[string] `json:"PostUp"`   // action that is executed after the device is up
	PreDown  ConfigOption[string] `json:"PreDown"`  // action that is executed before the device is down
	PostDown ConfigOption[string] `json:"PostDown"` // action that is executed after the device is down

	// Calculated values

	Filename string `json:"Filename"` // the filename of the config file, for example: wg_peer_x.conf
}

func NewPeer(src *domain.Peer) *Peer {
	return &Peer{
		Identifier:          string(src.Identifier),
		DisplayName:         src.DisplayName,
		UserIdentifier:      string(src.UserIdentifier),
		InterfaceIdentifier: string(src.InterfaceIdentifier),
		Disabled:            src.IsDisabled(),
		DisabledReason:      src.DisabledReason,
		ExpiresAt:           ExpiryDate{src.ExpiresAt},
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
		ExpiresAt:           src.ExpiresAt.Time,
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

type MultiPeerRequest struct {
	Identifiers []string `json:"Identifiers"`
	Suffix      string   `json:"Suffix"`
}

func NewDomainPeerCreationRequest(src *MultiPeerRequest) *domain.PeerCreationRequest {
	return &domain.PeerCreationRequest{
		UserIdentifiers: src.Identifiers,
		Suffix:          src.Suffix,
	}
}

type PeerMailRequest struct {
	Identifiers []string `json:"Identifiers"`
	LinkOnly    bool     `json:"LinkOnly"`
}

type PeerStats struct {
	Enabled bool `json:"Enabled" example:"true"` // peer stats tracking enabled

	Stats map[string]PeerStatData `json:"Stats"` // stats, map key = Peer identifier
}

func NewPeerStats(enabled bool, src []domain.PeerStatus) *PeerStats {
	stats := make(map[string]PeerStatData, len(src))

	for _, srcStat := range src {
		stats[string(srcStat.PeerId)] = PeerStatData{
			IsConnected:      srcStat.IsConnected(),
			IsPingable:       srcStat.IsPingable,
			LastPing:         srcStat.LastPing,
			BytesReceived:    srcStat.BytesReceived,
			BytesTransmitted: srcStat.BytesTransmitted,
			LastHandshake:    srcStat.LastHandshake,
			EndpointAddress:  srcStat.Endpoint,
			LastSessionStart: srcStat.LastSessionStart,
		}
	}

	return &PeerStats{
		Enabled: enabled,
		Stats:   stats,
	}
}

type PeerStatData struct {
	IsConnected bool `json:"IsConnected"`

	IsPingable bool       `json:"IsPingable"`
	LastPing   *time.Time `json:"LastPing"`

	BytesReceived    uint64 `json:"BytesReceived"`
	BytesTransmitted uint64 `json:"BytesTransmitted"`

	LastHandshake    *time.Time `json:"LastHandshake"`
	EndpointAddress  string     `json:"EndpointAddress"`
	LastSessionStart *time.Time `json:"LastSessionStart"`
}
