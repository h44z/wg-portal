package wireguard

import (
	"database/sql"
	"time"
)

// ConfigOption is an Overridable configuration option
type ConfigOption struct {
	Value       interface{}
	Overridable bool
}

type StringConfigOption struct {
	ConfigOption
}

func (o StringConfigOption) GetValue() string {
	return o.Value.(string)
}

type IntConfigOption struct {
	ConfigOption
}

func (o IntConfigOption) GetValue() int {
	return o.Value.(int)
}

type Int32ConfigOption struct {
	ConfigOption
}

func (o Int32ConfigOption) GetValue() int32 {
	return o.Value.(int32)
}

type BoolConfigOption struct {
	ConfigOption
}

func (o BoolConfigOption) GetValue() bool {
	return o.Value.(bool)
}

type InterfaceType string

const (
	InterfaceTypeServer InterfaceType = "server"
	InterfaceTypeClient InterfaceType = "client"
)

type DeviceIdentifier string
type PeerIdentifier string

type InterfaceConfig struct {
	// WireGuard specific (for the [interface] section of the config file)

	DeviceName DeviceIdentifier // device name, for example: wg0
	KeyPair    KeyPair          // private/public Key of the server interface
	ListenPort int              // the listening port, for example: 51820

	AddressStr string // the interface ip addresses, comma separated
	Dns        string // the dns server that should be set if the interface is up

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
	PeerDefDns                 string // the default dns server for the peer
	PeerDefEndpoint            string // the default endpoint for the peer
	PeerDefAllowedIPsString    string // the default allowed IP string for the peer
	PeerDefMtu                 int    // the default device MTU
	PeerDefPersistentKeepalive int    // the default persistent keep-alive Value
	PeerDefFirewallMark        int32  // default firewall mark
	PeerDefRoutingTable        string // the default routing table

	PeerDefPreUp    string // default action that is executed before the device is up
	PeerDefPostUp   string // default action that is executed after the device is up
	PeerDefPreDown  string // default action that is executed before the device is down
	PeerDefPostDown string // default action that is executed after the device is down
}

type PeerConfig struct {
	// WireGuard specific (for the [peer] section of the config file)

	Endpoint              StringConfigOption // the endpoint address
	AllowedIPsString      StringConfigOption // all allowed ip subnets, comma seperated
	ExtraAllowedIPsString string             // all allowed ip subnets on the server side, comma seperated
	KeyPair               KeyPair            // private/public Key of the peer
	PresharedKey          string             // the pre-shared Key of the peer
	PersistentKeepalive   IntConfigOption    // the persistent keep-alive interval

	// WG Portal specific

	Identifier string         // a nice display name/ description for the peer
	Uid        PeerIdentifier // peer unique identifier

	// Interface settings for the peer, used to generate the [interface] section in the peer config file

	AddressStr   StringConfigOption // the interface ip addresses, comma separated
	Dns          StringConfigOption // the dns server that should be set if the interface is up
	Mtu          IntConfigOption    // the device MTU
	FirewallMark Int32ConfigOption  // a firewall mark
	RoutingTable StringConfigOption // the routing table

	PreUp    StringConfigOption // action that is executed before the device is up
	PostUp   StringConfigOption // action that is executed after the device is up
	PreDown  StringConfigOption // action that is executed before the device is down
	PostDown StringConfigOption // action that is executed after the device is down

	// Internal stats

	DeactivatedAt sql.NullTime
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type InterfaceConfigPersister interface {
	PersistInterface(cfg InterfaceConfig)
	LoadInterface(cfg InterfaceConfig)
	DeleteInterface(cfg InterfaceConfig)
}

type PeerConfigPersister interface {
	PersistPeer(cfg PeerConfig)
	LoadPeer(cfg PeerConfig)
	DeletePeer(cfg PeerConfig)
}

type ConfigPersister interface {
	InterfaceConfigPersister
	PeerConfigPersister
}
