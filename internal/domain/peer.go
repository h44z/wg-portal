package domain

import (
	"fmt"
	"github.com/h44z/wg-portal/internal"
	"net"
	"regexp"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type PeerIdentifier string

func (i PeerIdentifier) IsPublicKey() bool {
	_, err := wgtypes.ParseKey(string(i))
	if err != nil {
		return false
	}
	return true
}

func (i PeerIdentifier) ToPublicKey() wgtypes.Key {
	publicKey, _ := wgtypes.ParseKey(string(i))
	return publicKey
}

type Peer struct {
	BaseModel

	// WireGuard specific (for the [peer] section of the config file)

	Endpoint            StringConfigOption `gorm:"embedded;embeddedPrefix:endpoint_"`        // the endpoint address
	EndpointPublicKey   StringConfigOption `gorm:"embedded;embeddedPrefix:endpoint_pubkey_"` // the endpoint public key
	AllowedIPsStr       StringConfigOption `gorm:"embedded;embeddedPrefix:allowed_ips_str_"` // all allowed ip subnets, comma seperated
	ExtraAllowedIPsStr  string             // all allowed ip subnets on the server side, comma seperated
	PresharedKey        PreSharedKey       // the pre-shared Key of the peer
	PersistentKeepalive IntConfigOption    `gorm:"embedded;embeddedPrefix:persistent_keep_alive_"` // the persistent keep-alive interval

	// WG Portal specific

	DisplayName         string              // a nice display name/ description for the peer
	Identifier          PeerIdentifier      `gorm:"primaryKey;column:identifier"`      // peer unique identifier
	UserIdentifier      UserIdentifier      `gorm:"index;column:user_identifier"`      // the owner
	InterfaceIdentifier InterfaceIdentifier `gorm:"index;column:interface_identifier"` // the interface id
	Disabled            *time.Time          `gorm:"column:disabled"`                   // if this field is set, the peer is disabled
	DisabledReason      string              // the reason why the peer has been disabled
	ExpiresAt           *time.Time          `gorm:"column:expires_at"`         // expiry dates for peers
	Notes               string              `form:"notes" binding:"omitempty"` // a note field for peers

	// Interface settings for the peer, used to generate the [interface] section in the peer config file
	Interface PeerInterfaceConfig `gorm:"embedded"`
}

func (p *Peer) IsDisabled() bool {
	return p.Disabled != nil
}

func (p *Peer) IsExpired() bool {
	if p.ExpiresAt == nil {
		return false
	}
	if p.ExpiresAt.Before(time.Now()) {
		return true
	}
	return false
}

func (p *Peer) CheckAliveAddress() string {
	if p.Interface.CheckAliveAddress != "" {
		return p.Interface.CheckAliveAddress
	}

	if len(p.Interface.Addresses) > 0 {
		return p.Interface.Addresses[0].Addr // take the first peer address
	}

	return ""
}

func (p *Peer) CopyCalculatedAttributes(src *Peer) {
	p.BaseModel = src.BaseModel
}

func (p *Peer) GetConfigFileName() string {
	filename := ""
	reg := regexp.MustCompile("[^a-zA-Z0-9-_]+")

	if p.DisplayName != "" {
		filename = p.DisplayName
		filename = strings.ReplaceAll(filename, " ", "_")
		filename = reg.ReplaceAllString(filename, "")
		filename = internal.TruncateString(filename, 16)
		filename += ".conf"
	} else {
		filename = fmt.Sprintf("wg_%s", internal.TruncateString(string(p.Identifier), 8))
		filename = reg.ReplaceAllString(filename, "")
		filename += ".conf"
	}

	return filename
}

func (p *Peer) ApplyInterfaceDefaults(in *Interface) {
	p.Endpoint.TrySetValue(in.PeerDefEndpoint)
	p.EndpointPublicKey.TrySetValue(in.PublicKey)
	p.AllowedIPsStr.TrySetValue(in.PeerDefAllowedIPsStr)
	p.PersistentKeepalive.TrySetValue(in.PeerDefPersistentKeepalive)
	p.Interface.DnsStr.TrySetValue(in.PeerDefDnsStr)
	p.Interface.DnsSearchStr.TrySetValue(in.PeerDefDnsSearchStr)
	p.Interface.Mtu.TrySetValue(in.PeerDefMtu)
	p.Interface.FirewallMark.TrySetValue(in.PeerDefFirewallMark)
	p.Interface.RoutingTable.TrySetValue(in.PeerDefRoutingTable)
	p.Interface.PreUp.TrySetValue(in.PeerDefPreUp)
	p.Interface.PostUp.TrySetValue(in.PeerDefPostUp)
	p.Interface.PreDown.TrySetValue(in.PeerDefPreDown)
	p.Interface.PostDown.TrySetValue(in.PeerDefPostDown)
}

type PeerInterfaceConfig struct {
	KeyPair // private/public Key of the peer

	Type InterfaceType `gorm:"column:iface_type"` // the interface type (server, client, any)

	Addresses         []Cidr             `gorm:"many2many:peer_addresses;"`                     // the interface ip addresses
	CheckAliveAddress string             `gorm:"column:check_alive_address"`                    // optional ip address or DNS name that is used for ping checks
	DnsStr            StringConfigOption `gorm:"embedded;embeddedPrefix:iface_dns_str_"`        // the dns server that should be set if the interface is up, comma separated
	DnsSearchStr      StringConfigOption `gorm:"embedded;embeddedPrefix:iface_dns_search_str_"` // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu               IntConfigOption    `gorm:"embedded;embeddedPrefix:iface_mtu_"`            // the device MTU
	FirewallMark      Int32ConfigOption  `gorm:"embedded;embeddedPrefix:iface_firewall_mark_"`  // a firewall mark
	RoutingTable      StringConfigOption `gorm:"embedded;embeddedPrefix:iface_routing_table_"`  // the routing table

	PreUp    StringConfigOption `gorm:"embedded;embeddedPrefix:iface_pre_up_"`    // action that is executed before the device is up
	PostUp   StringConfigOption `gorm:"embedded;embeddedPrefix:iface_post_up_"`   // action that is executed after the device is up
	PreDown  StringConfigOption `gorm:"embedded;embeddedPrefix:iface_pre_down_"`  // action that is executed before the device is down
	PostDown StringConfigOption `gorm:"embedded;embeddedPrefix:iface_post_down_"` // action that is executed after the device is down
}

func (p *PeerInterfaceConfig) AddressStr() string {
	return CidrsToString(p.Addresses)
}

type PhysicalPeer struct {
	Identifier PeerIdentifier // peer unique identifier

	Endpoint            string       // the endpoint address
	AllowedIPs          []Cidr       // all allowed ip subnets
	KeyPair                          // private/public Key of the peer, for imports it only contains the public key as the private key is not known to the server
	PresharedKey        PreSharedKey // the pre-shared Key of the peer
	PersistentKeepalive int          // the persistent keep-alive interval

	LastHandshake   time.Time
	ProtocolVersion int

	BytesUpload   uint64 // upload bytes are the number of bytes that the remote peer has sent to the server
	BytesDownload uint64 // upload bytes are the number of bytes that the remote peer has received from the server
}

func (p PhysicalPeer) GetPresharedKey() *wgtypes.Key {
	if p.PresharedKey == "" {
		return nil
	}
	key, err := wgtypes.ParseKey(string(p.PresharedKey))
	if err != nil {
		return nil
	}

	return &key
}

func (p PhysicalPeer) GetEndpointAddress() *net.UDPAddr {
	if p.Endpoint == "" {
		return nil
	}
	addr, err := net.ResolveUDPAddr("udp", p.Endpoint)
	if err != nil {
		return nil
	}

	return addr
}

func (p PhysicalPeer) GetPersistentKeepaliveTime() *time.Duration {
	if p.PersistentKeepalive == 0 {
		return nil
	}

	keepAliveDuration := time.Duration(p.PersistentKeepalive) * time.Second
	return &keepAliveDuration
}

func (p PhysicalPeer) GetAllowedIPs() ([]net.IPNet, error) {
	allowedIPs := make([]net.IPNet, len(p.AllowedIPs))
	for i, ip := range p.AllowedIPs {
		allowedIPs[i] = *ip.IpNet()
	}

	return allowedIPs, nil
}

func ConvertPhysicalPeer(pp *PhysicalPeer) *Peer {
	peer := &Peer{
		Endpoint:            StringConfigOption{Value: pp.Endpoint, Overridable: true},
		EndpointPublicKey:   StringConfigOption{Value: "", Overridable: true},
		AllowedIPsStr:       StringConfigOption{Value: "", Overridable: true},
		ExtraAllowedIPsStr:  "",
		PresharedKey:        pp.PresharedKey,
		PersistentKeepalive: IntConfigOption{Value: pp.PersistentKeepalive, Overridable: true},
		DisplayName:         string(pp.Identifier),
		Identifier:          pp.Identifier,
		UserIdentifier:      "",
		InterfaceIdentifier: "",
		Disabled:            nil,
		Interface: PeerInterfaceConfig{
			KeyPair: pp.KeyPair,
		},
	}

	return peer
}

func MergeToPhysicalPeer(pp *PhysicalPeer, p *Peer) {
	pp.Identifier = p.Identifier
	pp.Endpoint = p.Endpoint.GetValue()
	allowedIPs, _ := CidrsFromString(p.AllowedIPsStr.GetValue())
	extraAllowedIPs, _ := CidrsFromString(p.ExtraAllowedIPsStr)
	pp.AllowedIPs = append(allowedIPs, extraAllowedIPs...)
	pp.PresharedKey = p.PresharedKey
	pp.PublicKey = p.Interface.PublicKey
	pp.PersistentKeepalive = p.PersistentKeepalive.GetValue()
}

type PeerCreationRequest struct {
	UserIdentifiers []string
	Suffix          string
}
