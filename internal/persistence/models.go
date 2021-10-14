package persistence

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type BaseModel struct {
	CreatedBy  string
	UpdatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DisabledAt sql.NullTime
}

type InterfaceIdentifier string
type PeerIdentifier string
type UserIdentifier string

type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

type PreSharedKey string

type InterfaceType string

const (
	InterfaceTypeServer InterfaceType = "server"
	InterfaceTypeClient InterfaceType = "client"
)

type InterfaceConfig struct {
	BaseModel

	// WireGuard specific (for the [interface] section of the config file)

	Identifier InterfaceIdentifier `gorm:"primaryKey"` // device name, for example: wg0
	KeyPair                        // private/public Key of the server interface
	ListenPort int                 // the listening port, for example: 51820

	AddressStr   string // the interface ip addresses, comma separated
	DnsStr       string // the dns server that should be set if the interface is up, comma separated
	DnsSearchStr string // the dns search option string that should be set if the interface is up, will be appended to DnsStr

	Mtu          int    // the device MTU
	FirewallMark int32  // a firewall mark
	RoutingTable string // the routing table

	PreUp    string // action that is executed before the device is up
	PostUp   string // action that is executed after the device is up
	PreDown  string // action that is executed before the device is down
	PostDown string // action that is executed after the device is down

	SaveConfig bool // automatically persist config changes to the wgX.conf file

	// WG Portal specific
	Enabled     bool          // flag that specifies if the interface is enabled (up) or nor (down)
	DisplayName string        // a nice display name/ description for the interface
	Type        InterfaceType // the interface type, either InterfaceTypeServer or InterfaceTypeClient
	DriverType  string        // the interface driver type (linux, software, ...)

	// Default settings for the peer, used for new peers, those settings will be published to ConfigOption options of
	// the peer config

	PeerDefNetworkStr          string // the default subnets from which peers will get their IP addresses, comma seperated
	PeerDefDnsStr              string // the default dns server for the peer
	PeerDefDnsSearchStr        string // the default dns search options for the peer
	PeerDefEndpoint            string // the default endpoint for the peer
	PeerDefAllowedIPsStr       string // the default allowed IP string for the peer
	PeerDefMtu                 int    // the default device MTU
	PeerDefPersistentKeepalive int    // the default persistent keep-alive Value
	PeerDefFirewallMark        int32  // default firewall mark
	PeerDefRoutingTable        string // the default routing table

	PeerDefPreUp    string // default action that is executed before the device is up
	PeerDefPostUp   string // default action that is executed after the device is up
	PeerDefPreDown  string // default action that is executed before the device is down
	PeerDefPostDown string // default action that is executed after the device is down
}

type PeerInterfaceConfig struct {
	Identifier InterfaceIdentifier `gorm:"index;column:iface_identifier"` // the interface identifier
	Type       InterfaceType       `gorm:"column:iface_type"`             // the interface type
	PublicKey  string              `gorm:"column:iface_pubkey"`           // the interface public key

	AddressStr   StringConfigOption `gorm:"embedded;embeddedPrefix:iface_address_str_"`    // the interface ip addresses, comma separated
	DnsStr       StringConfigOption `gorm:"embedded;embeddedPrefix:iface_dns_str_"`        // the dns server that should be set if the interface is up, comma separated
	DnsSearchStr StringConfigOption `gorm:"embedded;embeddedPrefix:iface_dns_search_str_"` // the dns search option string that should be set if the interface is up, will be appended to DnsStr
	Mtu          IntConfigOption    `gorm:"embedded;embeddedPrefix:iface_mtu_"`            // the device MTU
	FirewallMark Int32ConfigOption  `gorm:"embedded;embeddedPrefix:iface_firewall_mark_"`  // a firewall mark
	RoutingTable StringConfigOption `gorm:"embedded;embeddedPrefix:iface_routing_table_"`  // the routing table

	PreUp    StringConfigOption `gorm:"embedded;embeddedPrefix:iface_pre_up_"`    // action that is executed before the device is up
	PostUp   StringConfigOption `gorm:"embedded;embeddedPrefix:iface_post_up_"`   // action that is executed after the device is up
	PreDown  StringConfigOption `gorm:"embedded;embeddedPrefix:iface_pre_down_"`  // action that is executed before the device is down
	PostDown StringConfigOption `gorm:"embedded;embeddedPrefix:iface_post_down_"` // action that is executed after the device is down
}

type PeerConfig struct {
	BaseModel

	// WireGuard specific (for the [peer] section of the config file)

	Endpoint            StringConfigOption `gorm:"embedded;embeddedPrefix:endpoint_"`        // the endpoint address
	AllowedIPsStr       StringConfigOption `gorm:"embedded;embeddedPrefix:allowed_ips_str_"` // all allowed ip subnets, comma seperated
	ExtraAllowedIPsStr  string             // all allowed ip subnets on the server side, comma seperated
	KeyPair                                // private/public Key of the peer
	PresharedKey        string             // the pre-shared Key of the peer
	PersistentKeepalive IntConfigOption    `gorm:"embedded;embeddedPrefix:persistent_keep_alive_"` // the persistent keep-alive interval

	// WG Portal specific

	DisplayName    string         // a nice display name/ description for the peer
	Identifier     PeerIdentifier `gorm:"primaryKey"` // peer unique identifier
	UserIdentifier UserIdentifier `gorm:"index"`      // the owner

	// Interface settings for the peer, used to generate the [interface] section in the peer config file
	Interface *PeerInterfaceConfig `gorm:"embedded"`
}

type UserSource string

const (
	UserSourceLdap     UserSource = "ldap" // LDAP / ActiveDirectory
	UserSourceDatabase UserSource = "db"   // sqlite / mysql database
	UserSourceOIDC     UserSource = "oidc" // open id connect, TODO: implement
)

type PrivateString string

func (PrivateString) MarshalJSON() ([]byte, error) {
	return []byte(`""`), nil
}

func (PrivateString) String() string {
	return ""
}

// User is the user model that gets linked to peer entries, by default an empty user model with only the email address is created
type User struct {
	// required fields
	Identifier UserIdentifier `gorm:"primaryKey"`
	Email      string         `form:"email" binding:"required,email"`
	Source     UserSource
	IsAdmin    bool

	// optional fields
	Firstname  string `form:"firstname" binding:"omitempty"`
	Lastname   string `form:"lastname" binding:"omitempty"`
	Phone      string `form:"phone" binding:"omitempty"`
	Department string `form:"department" binding:"omitempty"`

	// optional, integrated password authentication
	Password PrivateString `form:"password" binding:"omitempty"`

	// database internal fields
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index" json:",omitempty"` // used as a "deactivated" flag
}
