package wireguard

import (
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
	if o.Value == nil {
		return ""
	}
	return o.Value.(string)
}

func NewStringConfigOption(value string, overridable bool) StringConfigOption {
	return StringConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type IntConfigOption struct {
	ConfigOption
}

func (o IntConfigOption) GetValue() int {
	if o.Value == nil {
		return 0
	}
	return o.Value.(int)
}

func NewIntConfigOption(value int, overridable bool) IntConfigOption {
	return IntConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type Int32ConfigOption struct {
	ConfigOption
}

func (o Int32ConfigOption) GetValue() int32 {
	if o.Value == nil {
		return 0
	}

	return o.Value.(int32)
}

func NewInt32ConfigOption(value int32, overridable bool) Int32ConfigOption {
	return Int32ConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type BoolConfigOption struct {
	ConfigOption
}

func (o BoolConfigOption) GetValue() bool {
	if o.Value == nil {
		return false
	}

	return o.Value.(bool)
}

func NewBoolConfigOption(value bool, overridable bool) BoolConfigOption {
	return BoolConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type InterfaceType string

const (
	InterfaceTypeServer InterfaceType = "server"
	InterfaceTypeClient InterfaceType = "client"
)

type DeviceIdentifier string
type PeerIdentifier string

type BaseConfig struct {
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InterfaceConfig struct {
	BaseConfig

	// WireGuard specific (for the [interface] section of the config file)

	DeviceName DeviceIdentifier // device name, for example: wg0
	KeyPair    KeyPair          // private/public Key of the server interface
	ListenPort int              // the listening port, for example: 51820

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

	// Internal stats

	DisabledAt *time.Time
}

type PeerConfig struct {
	BaseConfig

	// WireGuard specific (for the [peer] section of the config file)

	Endpoint            StringConfigOption // the endpoint address
	AllowedIPsStr       StringConfigOption // all allowed ip subnets, comma seperated
	ExtraAllowedIPsStr  string             // all allowed ip subnets on the server side, comma seperated
	KeyPair             KeyPair            // private/public Key of the peer
	PresharedKey        string             // the pre-shared Key of the peer
	PersistentKeepalive IntConfigOption    // the persistent keep-alive interval

	// WG Portal specific

	Identifier string         // a nice display name/ description for the peer
	Uid        PeerIdentifier // peer unique identifier

	// Interface settings for the peer, used to generate the [interface] section in the peer config file

	AddressStr   StringConfigOption // the interface ip addresses, comma separated
	DnsStr       StringConfigOption // the dns server that should be set if the interface is up, comma separated
	Mtu          IntConfigOption    // the device MTU
	FirewallMark Int32ConfigOption  // a firewall mark
	RoutingTable StringConfigOption // the routing table

	PreUp    StringConfigOption // action that is executed before the device is up
	PostUp   StringConfigOption // action that is executed after the device is up
	PreDown  StringConfigOption // action that is executed before the device is down
	PostDown StringConfigOption // action that is executed after the device is down

	// Internal stats

	DisabledAt *time.Time
}

// ConfigWriter provides methods for updating persistent backends (like a database or a WireGuard configuration file)
type ConfigWriter interface {
	SaveInterface(cfg InterfaceConfig, peers []PeerConfig) error
	SavePeer(peer PeerConfig, cfg InterfaceConfig) error
	DeleteInterface(cfg InterfaceConfig, peers []PeerConfig) error
	DeletePeer(peer PeerConfig, cfg InterfaceConfig) error
}

// ConfigLoader provides methods to load interface and peer configurations from a persistent backend.
type ConfigLoader interface {
	Load(identifier DeviceIdentifier) (InterfaceConfig, []PeerConfig, error)
	LoadAll(ignored ...DeviceIdentifier) (map[InterfaceConfig][]PeerConfig, error)
}
