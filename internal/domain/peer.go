package domain

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
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

	Endpoint            ConfigOption[string] `gorm:"embedded;embeddedPrefix:endpoint_"`        // the endpoint address
	EndpointPublicKey   ConfigOption[string] `gorm:"embedded;embeddedPrefix:endpoint_pubkey_"` // the endpoint public key
	AllowedIPsStr       ConfigOption[string] `gorm:"embedded;embeddedPrefix:allowed_ips_str_"` // all allowed ip subnets, comma seperated
	ExtraAllowedIPsStr  string               // all allowed ip subnets on the server side, comma seperated
	PresharedKey        PreSharedKey         `gorm:"serializer:encstr"`                              // the pre-shared Key of the peer
	PersistentKeepalive ConfigOption[int]    `gorm:"embedded;embeddedPrefix:persistent_keep_alive_"` // the persistent keep-alive interval

	// WG Portal specific

	DisplayName          string              // a nice display name/ description for the peer
	Identifier           PeerIdentifier      `gorm:"primaryKey;column:identifier"`      // peer unique identifier
	UserIdentifier       UserIdentifier      `gorm:"index;column:user_identifier"`      // the owner
	InterfaceIdentifier  InterfaceIdentifier `gorm:"index;column:interface_identifier"` // the interface id
	Disabled             *time.Time          `gorm:"column:disabled"`                   // if this field is set, the peer is disabled
	DisabledReason       string              // the reason why the peer has been disabled
	ExpiresAt            *time.Time          `gorm:"column:expires_at"`         // expiry dates for peers
	Notes                string              `form:"notes" binding:"omitempty"` // a note field for peers
	AutomaticallyCreated bool                `gorm:"column:auto_created"`       // specifies if the peer was automatically created

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

	if p.DisplayName != "" {
		filename = p.DisplayName
		filename = strings.ReplaceAll(filename, " ", "_")
		// Eliminate the automatically detected peer part,
		// as it makes the filename indistinguishable among multiple auto-detected peers.
		filename = strings.ReplaceAll(filename, "Autodetected_", "")
		filename = allowedFileNameRegex.ReplaceAllString(filename, "")
		filename = internal.TruncateString(filename, 16)
		filename += ".conf"
	} else {
		filename = fmt.Sprintf("wg_%s", internal.TruncateString(string(p.Identifier), 8))
		filename = allowedFileNameRegex.ReplaceAllString(filename, "")
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

func (p *Peer) GenerateDisplayName(prefix string) {
	if prefix != "" {
		prefix = fmt.Sprintf("%s ", strings.TrimSpace(prefix)) // add a space after the prefix
	}
	p.DisplayName = fmt.Sprintf("%sPeer %s", prefix, internal.TruncateString(string(p.Identifier), 8))
}

// OverwriteUserEditableFields overwrites the user editable fields of the peer with the values from the userPeer
func (p *Peer) OverwriteUserEditableFields(userPeer *Peer, cfg *config.Config) {
	p.DisplayName = userPeer.DisplayName
	if cfg.Core.EditableKeys {
		p.Interface.PublicKey = userPeer.Interface.PublicKey
		p.Interface.PrivateKey = userPeer.Interface.PrivateKey
		p.PresharedKey = userPeer.PresharedKey
	}
	p.Interface.Mtu = userPeer.Interface.Mtu
	p.PersistentKeepalive = userPeer.PersistentKeepalive
	p.ExpiresAt = userPeer.ExpiresAt
	p.Disabled = userPeer.Disabled
	p.DisabledReason = userPeer.DisabledReason
}

type PeerInterfaceConfig struct {
	KeyPair // private/public Key of the peer

	Type InterfaceType `gorm:"column:iface_type"` // the interface type (server, client, any)

	Addresses         []Cidr               `gorm:"many2many:peer_addresses;"`                     // the interface ip addresses
	CheckAliveAddress string               `gorm:"column:check_alive_address"`                    // optional ip address or DNS name that is used for ping checks
	DnsStr            ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_dns_str_"`        // the dns server that should be set if the interface is up, comma separated
	DnsSearchStr      ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_dns_search_str_"` // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu               ConfigOption[int]    `gorm:"embedded;embeddedPrefix:iface_mtu_"`            // the device MTU
	FirewallMark      ConfigOption[uint32] `gorm:"embedded;embeddedPrefix:iface_firewall_mark_"`  // a firewall mark
	RoutingTable      ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_routing_table_"`  // the routing table

	PreUp    ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_pre_up_"`    // action that is executed before the device is up
	PostUp   ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_post_up_"`   // action that is executed after the device is up
	PreDown  ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_pre_down_"`  // action that is executed before the device is down
	PostDown ConfigOption[string] `gorm:"embedded;embeddedPrefix:iface_post_down_"` // action that is executed after the device is down
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

func (p PhysicalPeer) GetAllowedIPs() []net.IPNet {
	allowedIPs := make([]net.IPNet, len(p.AllowedIPs))
	for i, ip := range p.AllowedIPs {
		allowedIPs[i] = *ip.IpNet()
	}

	return allowedIPs
}

func ConvertPhysicalPeer(pp *PhysicalPeer) *Peer {
	peer := &Peer{
		Endpoint:            NewConfigOption(pp.Endpoint, true),
		EndpointPublicKey:   NewConfigOption("", true),
		AllowedIPsStr:       NewConfigOption("", true),
		ExtraAllowedIPsStr:  "",
		PresharedKey:        pp.PresharedKey,
		PersistentKeepalive: NewConfigOption(pp.PersistentKeepalive, true),
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
	if p.Interface.Type == InterfaceTypeServer {
		allowedIPs, _ := CidrsFromString(p.AllowedIPsStr.GetValue())
		extraAllowedIPs, _ := CidrsFromString(p.ExtraAllowedIPsStr)
		pp.AllowedIPs = append(allowedIPs, extraAllowedIPs...)
	} else {
		allowedIPs := make([]Cidr, len(p.Interface.Addresses))
		for i, ip := range p.Interface.Addresses {
			allowedIPs[i] = ip.HostAddr()
		}
		extraAllowedIPs, _ := CidrsFromString(p.ExtraAllowedIPsStr)
		pp.AllowedIPs = append(allowedIPs, extraAllowedIPs...)
	}
	pp.PresharedKey = p.PresharedKey
	pp.PublicKey = p.Interface.PublicKey
	pp.PersistentKeepalive = p.PersistentKeepalive.GetValue()
}

type PeerCreationRequest struct {
	UserIdentifiers []string
	Suffix          string
}
