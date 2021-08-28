package wireguard

import (
	"database/sql"
	"time"

	"gorm.io/gorm/clause"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type DatabaseBackend struct {
	db *gorm.DB
}

func NewDatabaseBackend(db *gorm.DB) (*DatabaseBackend, error) {
	backend := &DatabaseBackend{db: db}

	// Auto-Migrate Gorm models
	err := db.AutoMigrate(&dbInterfaceConfig{}, &dbDefaultPeerConfig{}, &dbPeerConfig{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to migrate WireGuard database")
	}

	return backend, nil
}

func (d DatabaseBackend) SaveInterface(cfg InterfaceConfig, _ []PeerConfig) error {
	iface, peerDefaults := convertInterface(cfg)

	if err := d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&iface).Error; err != nil {
		return errors.Wrapf(err, "failed to save interface %s to db", cfg.DeviceName)
	}
	if err := d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&peerDefaults).Error; err != nil {
		return errors.Wrapf(err, "failed to save peer defaults of %s to db", cfg.DeviceName)
	}

	return nil
}

func (d DatabaseBackend) SavePeer(cfg PeerConfig, iface InterfaceConfig) error {
	peer := convertPeer(cfg, iface.DeviceName)

	if err := d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&peer).Error; err != nil {
		return errors.Wrapf(err, "failed to save peer %s to db", cfg.Uid)
	}

	return nil
}

func (d DatabaseBackend) DeleteInterface(cfg InterfaceConfig, _ []PeerConfig) error {
	// Delete peers
	if err := d.db.Where("device_name = ?", cfg.DeviceName).Delete(&dbPeerConfig{}).Error; err != nil {
		return errors.Wrapf(err, "failed to delete peer for %s from db", cfg.DeviceName)
	}
	// Delete peer default settings
	if err := d.db.Where("device_name = ?", cfg.DeviceName).Delete(&dbDefaultPeerConfig{}).Error; err != nil {
		return errors.Wrapf(err, "failed to delete peer defaults for %s from db", cfg.DeviceName)
	}
	// Delete interface config
	if err := d.db.Where("device_name = ?", cfg.DeviceName).Delete(&dbInterfaceConfig{}).Error; err != nil {
		return errors.Wrapf(err, "failed to delete interface %s from db", cfg.DeviceName)
	}
	return nil
}

func (d DatabaseBackend) DeletePeer(cfg PeerConfig, iface InterfaceConfig) error {
	err := d.db.Where("device_name = ? AND uid = ?", iface.DeviceName, cfg.Uid).Delete(&dbPeerConfig{}).Error
	if err != nil {
		return errors.Wrapf(err, "failed to delete peer %s from db", cfg.Uid)
	}
	return nil
}

func (d DatabaseBackend) Load(identifier DeviceIdentifier) (InterfaceConfig, []PeerConfig, error) {
	var iface dbInterfaceConfig
	var peerDefaults dbDefaultPeerConfig
	var peers []dbPeerConfig

	if err := d.db.Where("device_name = ?", identifier).First(&iface).Error; err != nil {
		return InterfaceConfig{}, nil, errors.Wrapf(err, "failed to load interface %s from db", identifier)
	}
	if err := d.db.Where("device_name = ?", identifier).First(&peerDefaults).Error; err != nil {
		return InterfaceConfig{}, nil, errors.Wrapf(err, "failed to load peer defaults for %s from db", identifier)
	}
	if err := d.db.Where("device_name = ?", identifier).Find(&peers).Error; err != nil {
		return InterfaceConfig{}, nil, errors.Wrapf(err, "failed to load peers for %s from db", identifier)
	}

	interfaceConfig := InterfaceConfig{
		DeviceName:   DeviceIdentifier(iface.DeviceName),
		KeyPair:      KeyPair{PrivateKey: iface.PrivateKey, PublicKey: iface.PublicKey},
		ListenPort:   iface.ListenPort,
		AddressStr:   iface.AddressStr,
		DnsStr:       iface.DnsStr,
		Mtu:          iface.Mtu,
		FirewallMark: int32(iface.FirewallMark),
		RoutingTable: iface.RoutingTable,
		PreUp:        iface.PreUp,
		PostUp:       iface.PostUp,
		PreDown:      iface.PreDown,
		PostDown:     iface.PostDown,
		SaveConfig:   iface.SaveConfig,
		Enabled:      iface.Enabled,
		DisplayName:  iface.DisplayName,
		Type:         InterfaceType(iface.Type),
		DriverType:   iface.DriverType,

		PeerDefNetworkStr:          peerDefaults.NetworkStr,
		PeerDefDnsStr:              peerDefaults.DnsStr,
		PeerDefEndpoint:            peerDefaults.Endpoint,
		PeerDefAllowedIPsStr:       peerDefaults.AllowedIPsStr,
		PeerDefMtu:                 peerDefaults.Mtu,
		PeerDefPersistentKeepalive: peerDefaults.PersistentKeepalive,
		PeerDefFirewallMark:        int32(peerDefaults.FirewallMark),
		PeerDefRoutingTable:        peerDefaults.RoutingTable,
		PeerDefPreUp:               peerDefaults.PreUp,
		PeerDefPostUp:              peerDefaults.PostUp,
		PeerDefPreDown:             peerDefaults.PreDown,
		PeerDefPostDown:            peerDefaults.PostDown,

		DisabledAt: nil,
		BaseConfig: BaseConfig{
			CreatedAt: iface.CreatedAt,
			UpdatedAt: iface.UpdatedAt,
			CreatedBy: iface.CreatedBy,
			UpdatedBy: iface.UpdatedBy,
		},
	}
	if iface.DisabledAt.Valid {
		interfaceConfig.DisabledAt = &iface.DisabledAt.Time
	}

	peerConfigs := make([]PeerConfig, len(peers))
	for i, peer := range peers {
		peerConfigs[i] = PeerConfig{
			Endpoint:            NewStringConfigOption(peer.Endpoint, peer.OvrEndpoint),
			AllowedIPsStr:       NewStringConfigOption(peer.AllowedIPsStr, peer.OvrAllowedIPsStr),
			ExtraAllowedIPsStr:  peer.ExtraAllowedIPsStr,
			KeyPair:             KeyPair{PrivateKey: peer.PrivateKey, PublicKey: peer.PublicKey},
			PresharedKey:        peer.PresharedKey,
			PersistentKeepalive: NewIntConfigOption(peer.PersistentKeepalive, peer.OvrPersistentKeepalive),
			Identifier:          peer.Identifier,
			Uid:                 PeerIdentifier(peer.Uid),
			AddressStr:          NewStringConfigOption(peer.AddressStr, peer.OvrAddressStr),
			DnsStr:              NewStringConfigOption(peer.DnsStr, peer.OvrDnsStr),
			Mtu:                 NewIntConfigOption(peer.Mtu, peer.OvrMtu),
			FirewallMark:        NewInt32ConfigOption(int32(peer.FirewallMark), peer.OvrFirewallMark),
			RoutingTable:        NewStringConfigOption(peer.RoutingTable, peer.OvrRoutingTable),
			PreUp:               NewStringConfigOption(peer.PreUp, peer.OvrPreUp),
			PostUp:              NewStringConfigOption(peer.PostUp, peer.OvrPostUp),
			PreDown:             NewStringConfigOption(peer.PreDown, peer.OvrPreDown),
			PostDown:            NewStringConfigOption(peer.PostDown, peer.OvrPostDown),

			DisabledAt: nil,
			BaseConfig: BaseConfig{
				CreatedAt: iface.CreatedAt,
				UpdatedAt: iface.UpdatedAt,
				CreatedBy: iface.CreatedBy,
				UpdatedBy: iface.UpdatedBy,
			},
		}

		if peer.DisabledAt.Valid {
			peerConfigs[i].DisabledAt = &peer.DisabledAt.Time
		}
	}

	return interfaceConfig, peerConfigs, nil
}

func (d DatabaseBackend) LoadAll(ignored ...DeviceIdentifier) (map[InterfaceConfig][]PeerConfig, error) {
	interfaceIdentifiers := []DeviceIdentifier{} // TODO: fill this ?!

	result := make(map[InterfaceConfig][]PeerConfig)
	for _, identifier := range interfaceIdentifiers {
		iface, peers, err := d.Load(identifier)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load data for %s", identifier)
		}
		result[iface] = peers
	}

	return result, nil
}

//
//  --- Models
//

type dbBaseModel struct {
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type dbInterfaceConfig struct {
	dbBaseModel
	DisabledAt sql.NullTime

	// WireGuard specific (for the [interface] section of the config file)

	DeviceName string `gorm:"primaryKey"`
	PrivateKey string
	PublicKey  string
	ListenPort int

	AddressStr string
	DnsStr     string

	Mtu          int
	FirewallMark int
	RoutingTable string

	PreUp    string
	PostUp   string
	PreDown  string
	PostDown string

	SaveConfig bool

	// WG Portal specific
	Enabled     bool
	DisplayName string
	Type        string
	DriverType  string

	// Default settings for the peer, used for new peers, those settings will be published to ConfigOption options of
	// the peer config

	dbDefaultPeerConfig dbDefaultPeerConfig
}

func (d dbInterfaceConfig) TableName() string {
	return "interface"
}

type dbDefaultPeerConfig struct {
	dbBaseModel

	DeviceName string `gorm:"primaryKey"` // Foreign key

	NetworkStr          string // the default subnets from which peers will get their IP addresses, comma seperated
	DnsStr              string // the default dns server for the peer
	Endpoint            string // the default endpoint for the peer
	AllowedIPsStr       string // the default allowed IP string for the peer
	Mtu                 int    // the default device MTU
	PersistentKeepalive int    // the default persistent keep-alive Value
	FirewallMark        int    // default firewall mark
	RoutingTable        string // the default routing table

	PreUp    string // default action that is executed before the device is up
	PostUp   string // default action that is executed after the device is up
	PreDown  string // default action that is executed before the device is down
	PostDown string // default action that is executed after the device is down
}

func (d dbDefaultPeerConfig) TableName() string {
	return "peer_defaults"
}

type dbPeerConfig struct {
	dbBaseModel
	DisabledAt sql.NullTime

	DeviceName             string `gorm:"primaryKey"`
	Endpoint               string
	OvrEndpoint            bool
	AllowedIPsStr          string
	OvrAllowedIPsStr       bool
	ExtraAllowedIPsStr     string
	PrivateKey             string
	PublicKey              string
	PresharedKey           string
	PersistentKeepalive    int
	OvrPersistentKeepalive bool

	// WG Portal specific

	Identifier string
	Uid        string `gorm:"primaryKey"`

	// Interface settings for the peer, used to generate the [interface] section in the peer config file

	AddressStr      string
	OvrAddressStr   bool
	DnsStr          string
	OvrDnsStr       bool
	Mtu             int
	OvrMtu          bool
	FirewallMark    int
	OvrFirewallMark bool
	RoutingTable    string
	OvrRoutingTable bool

	PreUp       string
	OvrPreUp    bool
	PostUp      string
	OvrPostUp   bool
	PreDown     string
	OvrPreDown  bool
	PostDown    string
	OvrPostDown bool
}

func (d dbPeerConfig) TableName() string {
	return "peer"
}

func convertPeer(peer PeerConfig, devName DeviceIdentifier) dbPeerConfig {
	cfg := dbPeerConfig{
		DeviceName:             string(devName),
		Endpoint:               peer.Endpoint.GetValue(),
		OvrEndpoint:            peer.Endpoint.Overridable,
		AllowedIPsStr:          peer.AllowedIPsStr.GetValue(),
		OvrAllowedIPsStr:       peer.AllowedIPsStr.Overridable,
		ExtraAllowedIPsStr:     peer.ExtraAllowedIPsStr,
		PrivateKey:             peer.KeyPair.PrivateKey,
		PublicKey:              peer.KeyPair.PublicKey,
		PresharedKey:           peer.PresharedKey,
		PersistentKeepalive:    peer.PersistentKeepalive.GetValue(),
		OvrPersistentKeepalive: peer.PersistentKeepalive.Overridable,
		Identifier:             peer.Identifier,
		Uid:                    string(peer.Uid),
		AddressStr:             peer.AddressStr.GetValue(),
		OvrAddressStr:          peer.AddressStr.Overridable,
		DnsStr:                 peer.DnsStr.GetValue(),
		OvrDnsStr:              peer.DnsStr.Overridable,
		Mtu:                    peer.Mtu.GetValue(),
		OvrMtu:                 peer.Mtu.Overridable,
		FirewallMark:           int(peer.FirewallMark.GetValue()),
		OvrFirewallMark:        peer.FirewallMark.Overridable,
		RoutingTable:           peer.RoutingTable.GetValue(),
		OvrRoutingTable:        peer.RoutingTable.Overridable,
		PreUp:                  peer.PreUp.GetValue(),
		OvrPreUp:               peer.PreUp.Overridable,
		PostUp:                 peer.PostUp.GetValue(),
		OvrPostUp:              peer.PostUp.Overridable,
		PreDown:                peer.PreDown.GetValue(),
		OvrPreDown:             peer.PreDown.Overridable,
		PostDown:               peer.PostDown.GetValue(),
		OvrPostDown:            peer.PostDown.Overridable,
		DisabledAt:             sql.NullTime{Time: time.Time{}, Valid: peer.DisabledAt != nil},
	}
	if peer.DisabledAt != nil {
		cfg.DisabledAt.Time = *peer.DisabledAt
	}

	return cfg
}

func convertInterface(iface InterfaceConfig) (dbInterfaceConfig, dbDefaultPeerConfig) {
	cfg := dbInterfaceConfig{
		DeviceName:   string(iface.DeviceName),
		PrivateKey:   iface.KeyPair.PrivateKey,
		PublicKey:    iface.KeyPair.PublicKey,
		ListenPort:   iface.ListenPort,
		AddressStr:   iface.AddressStr,
		DnsStr:       iface.DnsStr,
		Mtu:          iface.Mtu,
		FirewallMark: int(iface.FirewallMark),
		RoutingTable: iface.RoutingTable,
		PreUp:        iface.PreUp,
		PostUp:       iface.PostUp,
		PreDown:      iface.PreDown,
		PostDown:     iface.PostDown,
		SaveConfig:   iface.SaveConfig,
		Enabled:      iface.Enabled,
		DisplayName:  iface.DisplayName,
		Type:         string(iface.Type),
		DriverType:   iface.DriverType,
		DisabledAt:   sql.NullTime{Time: time.Time{}, Valid: iface.DisabledAt != nil},
	}
	if iface.DisabledAt != nil {
		cfg.DisabledAt.Time = *iface.DisabledAt
	}
	peerDefaults := dbDefaultPeerConfig{
		DeviceName:          string(iface.DeviceName),
		NetworkStr:          iface.PeerDefNetworkStr,
		DnsStr:              iface.PeerDefDnsStr,
		Endpoint:            iface.PeerDefEndpoint,
		AllowedIPsStr:       iface.PeerDefAllowedIPsStr,
		Mtu:                 iface.PeerDefMtu,
		PersistentKeepalive: iface.PeerDefPersistentKeepalive,
		FirewallMark:        int(iface.PeerDefFirewallMark),
		RoutingTable:        iface.PeerDefRoutingTable,
		PreUp:               iface.PeerDefPreUp,
		PostUp:              iface.PeerDefPostUp,
		PreDown:             iface.PeerDefPreDown,
		PostDown:            iface.PeerDefPostDown,
	}

	return cfg, peerDefaults
}
