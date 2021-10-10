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

	Identifier InterfaceIdentifier // device name, for example: wg0
	KeyPair    KeyPair             // private/public Key of the server interface
	ListenPort int                 // the listening port, for example: 51820

	AddressStr string // the interface ip addresses, comma separated
	DnsStr     string // the dns server that should be set if the interface is up, comma separated

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
	Identifier InterfaceIdentifier // the interface identifier
	Type       InterfaceType       // the interface type
	PublicKey  string              // the interface public key

	AddressStr   StringConfigOption // the interface ip addresses, comma separated
	DnsStr       StringConfigOption // the dns server that should be set if the interface is up, comma separated
	Mtu          IntConfigOption    // the device MTU
	FirewallMark Int32ConfigOption  // a firewall mark
	RoutingTable StringConfigOption // the routing table

	PreUp    StringConfigOption // action that is executed before the device is up
	PostUp   StringConfigOption // action that is executed after the device is up
	PreDown  StringConfigOption // action that is executed before the device is down
	PostDown StringConfigOption // action that is executed after the device is down
}

type PeerConfig struct {
	BaseModel

	// WireGuard specific (for the [peer] section of the config file)

	Endpoint            StringConfigOption // the endpoint address
	AllowedIPsStr       StringConfigOption // all allowed ip subnets, comma seperated
	ExtraAllowedIPsStr  string             // all allowed ip subnets on the server side, comma seperated
	KeyPair             KeyPair            // private/public Key of the peer
	PresharedKey        string             // the pre-shared Key of the peer
	PersistentKeepalive IntConfigOption    // the persistent keep-alive interval

	// WG Portal specific

	DisplayName    string         // a nice display name/ description for the peer
	Identifier     PeerIdentifier // peer unique identifier
	UserIdentifier UserIdentifier // the owner

	// Interface settings for the peer, used to generate the [interface] section in the peer config file
	Interface *PeerInterfaceConfig
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
	Uid     UserIdentifier `gorm:"primaryKey"`
	Email   string         `form:"email" binding:"required,email"`
	Source  UserSource
	IsAdmin bool

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
	DeletedAt gorm.DeletedAt `gorm:"index" json:",omitempty"`
}
