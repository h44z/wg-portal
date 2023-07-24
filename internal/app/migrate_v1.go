package app

import (
	"errors"
	"fmt"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os"
	"time"
)

func migrateFromV1(cfg *config.Config, db *gorm.DB, source, typ string) error {
	sourceType := config.SupportedDatabase(typ)
	switch sourceType {
	case config.DatabaseMySQL, config.DatabasePostgres, config.DatabaseMsSQL:
	case config.DatabaseSQLite:
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("invalid source database: %w", err)
		}
	default:
		return errors.New("unsupported database")
	}

	oldDb, err := adapters.NewDatabase(config.DatabaseConfig{
		Type: sourceType,
		DSN:  source,
	})
	if err != nil {
		return fmt.Errorf("failed to open old database: %w", err)
	}

	// check if old db is a valid WireGuard Portal v1 database
	type DatabaseMigrationInfo struct {
		Version string `gorm:"primaryKey"`
		Applied time.Time
	}

	lastVersion := DatabaseMigrationInfo{}
	err = oldDb.Order("applied desc, version desc").FirstOrInit(&lastVersion).Error
	if err != nil {
		return fmt.Errorf("unable to validate old database: %w", err)
	}
	latestVersion := "1.0.9"
	if lastVersion.Version != latestVersion {
		return fmt.Errorf("unsupported old version, update to database version %s first: %w", latestVersion, err)
	}

	logrus.Infof("Found valid V1 database with version: %s", lastVersion.Version)

	if err := migrateV1Users(oldDb, db); err != nil {
		return fmt.Errorf("user migration failed: %w", err)
	}

	if err := migrateV1Interfaces(oldDb, db); err != nil {
		return fmt.Errorf("user migration failed: %w", err)
	}

	if err := migrateV1Peers(oldDb, db); err != nil {
		return fmt.Errorf("peer migration failed: %w", err)
	}

	logrus.Infof("Migrated V1 database with version %s, please restart WireGuard Portal", lastVersion.Version)

	return nil
}

func migrateV1Users(oldDb, newDb *gorm.DB) error {
	type User struct {
		Email     string `gorm:"primaryKey"`
		Source    string
		IsAdmin   bool
		Firstname string
		Lastname  string
		Phone     string
		Password  string
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt gorm.DeletedAt `gorm:"index"`
	}

	var oldUsers []User
	err := oldDb.Find(&oldUsers).Error
	if err != nil {
		return fmt.Errorf("unable to fetch old user records: %w", err)
	}

	for _, oldUser := range oldUsers {
		var deletionTime *time.Time
		deletionReason := ""
		if oldUser.DeletedAt.Valid {
			delTime := oldUser.DeletedAt.Time
			deletionTime = &delTime
			deletionReason = "disabled prior to migration"
		}
		newUser := domain.User{
			BaseModel: domain.BaseModel{
				CreatedBy: "v1migrator",
				UpdatedBy: "v1migrator",
				CreatedAt: oldUser.CreatedAt,
				UpdatedAt: oldUser.UpdatedAt,
			},
			Identifier:      domain.UserIdentifier(oldUser.Email),
			Email:           oldUser.Email,
			Source:          domain.UserSource(oldUser.Source),
			ProviderName:    "",
			IsAdmin:         oldUser.IsAdmin,
			Firstname:       oldUser.Firstname,
			Lastname:        oldUser.Lastname,
			Phone:           oldUser.Phone,
			Department:      "",
			Notes:           "",
			Password:        domain.PrivateString(oldUser.Password),
			Disabled:        deletionTime,
			DisabledReason:  deletionReason,
			Locked:          nil,
			LockedReason:    "",
			LinkedPeerCount: 0,
		}

		if err := newDb.Save(&newUser).Error; err != nil {
			return fmt.Errorf("failed to migrate user %s: %w", oldUser.Email, err)
		}

		logrus.Debugf(" - User %s migrated", newUser.Identifier)
	}

	return nil
}

func migrateV1Interfaces(oldDb, newDb *gorm.DB) error {
	type Device struct {
		Type                       string
		DeviceName                 string `gorm:"primaryKey"`
		DisplayName                string
		PrivateKey                 string
		ListenPort                 int
		FirewallMark               int32
		PublicKey                  string
		Mtu                        int
		IPsStr                     string
		DNSStr                     string
		RoutingTable               string
		PreUp                      string
		PostUp                     string
		PreDown                    string
		PostDown                   string
		SaveConfig                 bool
		DefaultEndpoint            string
		DefaultAllowedIPsStr       string
		DefaultPersistentKeepalive int
		CreatedAt                  time.Time
		UpdatedAt                  time.Time
	}

	var oldDevices []Device
	err := oldDb.Find(&oldDevices).Error
	if err != nil {
		return fmt.Errorf("unable to fetch old device records: %w", err)
	}

	for _, oldDevice := range oldDevices {
		ips, err := domain.CidrsFromString(oldDevice.IPsStr)
		if err != nil {
			return fmt.Errorf("failed to parse %s ip addresses: %w", oldDevice.DeviceName, err)
		}
		networks := make([]domain.Cidr, len(ips))
		for i, ip := range ips {
			networks[i] = domain.CidrFromIpNet(*ip.IpNet())
		}
		newInterface := domain.Interface{
			BaseModel: domain.BaseModel{
				CreatedBy: "v1migrator",
				UpdatedBy: "v1migrator",
				CreatedAt: oldDevice.CreatedAt,
				UpdatedAt: oldDevice.UpdatedAt,
			},
			Identifier: domain.InterfaceIdentifier(oldDevice.DeviceName),
			KeyPair: domain.KeyPair{
				PrivateKey: oldDevice.PrivateKey,
				PublicKey:  oldDevice.PublicKey,
			},
			ListenPort:                 oldDevice.ListenPort,
			Addresses:                  ips,
			DnsStr:                     "",
			DnsSearchStr:               "",
			Mtu:                        oldDevice.Mtu,
			FirewallMark:               oldDevice.FirewallMark,
			RoutingTable:               oldDevice.RoutingTable,
			PreUp:                      oldDevice.PreUp,
			PostUp:                     oldDevice.PostUp,
			PreDown:                    oldDevice.PreDown,
			PostDown:                   oldDevice.PostDown,
			SaveConfig:                 oldDevice.SaveConfig,
			DisplayName:                oldDevice.DisplayName,
			Type:                       domain.InterfaceType(oldDevice.Type),
			DriverType:                 "",
			Disabled:                   nil,
			DisabledReason:             "",
			PeerDefNetworkStr:          domain.CidrsToString(networks),
			PeerDefDnsStr:              oldDevice.DNSStr,
			PeerDefDnsSearchStr:        "",
			PeerDefEndpoint:            oldDevice.DefaultEndpoint,
			PeerDefAllowedIPsStr:       oldDevice.DefaultAllowedIPsStr,
			PeerDefMtu:                 oldDevice.Mtu,
			PeerDefPersistentKeepalive: oldDevice.DefaultPersistentKeepalive,
			PeerDefFirewallMark:        0,
			PeerDefRoutingTable:        "",
			PeerDefPreUp:               "",
			PeerDefPostUp:              "",
			PeerDefPreDown:             "",
			PeerDefPostDown:            "",
		}

		if err := newDb.Save(&newInterface).Error; err != nil {
			return fmt.Errorf("failed to migrate device %s: %w", oldDevice.DeviceName, err)
		}

		logrus.Debugf(" - Interface %s migrated", newInterface.Identifier)
	}

	return nil
}

func migrateV1Peers(oldDb, newDb *gorm.DB) error {
	type Peer struct {
		UID                  string
		DeviceName           string `gorm:"index"`
		Identifier           string
		Email                string `gorm:"index" form:"mail" binding:"required,email"`
		IgnoreGlobalSettings bool
		PublicKey            string `gorm:"primaryKey"`
		PresharedKey         string
		AllowedIPsStr        string
		AllowedIPsSrvStr     string
		Endpoint             string
		PersistentKeepalive  int
		PrivateKey           string
		IPsStr               string
		DNSStr               string
		Mtu                  int
		DeactivatedAt        *time.Time `json:",omitempty"`
		DeactivatedReason    string     `json:",omitempty"`
		ExpiresAt            *time.Time
		CreatedBy            string
		UpdatedBy            string
		CreatedAt            time.Time
		UpdatedAt            time.Time
	}

	var oldPeers []Peer
	err := oldDb.Find(&oldPeers).Error
	if err != nil {
		return fmt.Errorf("unable to fetch old peer records: %w", err)
	}

	for _, oldPeer := range oldPeers {
		ips, err := domain.CidrsFromString(oldPeer.IPsStr)
		if err != nil {
			return fmt.Errorf("failed to parse %s ip addresses: %w", oldPeer.PublicKey, err)
		}
		var disableTime *time.Time
		disableReason := ""
		if oldPeer.DeactivatedAt != nil {
			disTime := *oldPeer.DeactivatedAt
			disableTime = &disTime
			disableReason = oldPeer.DeactivatedReason
		}
		var expiryTime *time.Time
		if oldPeer.ExpiresAt != nil {
			expTime := *oldPeer.ExpiresAt
			expiryTime = &expTime
		}
		var iface domain.Interface
		var ifaceType domain.InterfaceType
		err = newDb.First(&iface, "identifier = ?", oldPeer.DeviceName).Error
		if err != nil {
			return fmt.Errorf("failed to find interface %s for peer %s: %w", oldPeer.DeviceName, oldPeer.PublicKey, err)
		}
		switch iface.Type {
		case domain.InterfaceTypeClient:
			ifaceType = domain.InterfaceTypeServer
		case domain.InterfaceTypeServer:
			ifaceType = domain.InterfaceTypeClient
		case domain.InterfaceTypeAny:
			ifaceType = domain.InterfaceTypeAny
		}
		var user domain.User
		err = newDb.First(&user, "identifier = ?", oldPeer.Email).Error // migrated users use the email address as identifier
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to find user %s for peer %s: %w", oldPeer.Email, oldPeer.PublicKey, err)
		}
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// create dummy user
			now := time.Now()
			user = domain.User{
				BaseModel: domain.BaseModel{
					CreatedBy: "v1migrator",
					UpdatedBy: "v1migrator",
					CreatedAt: now,
					UpdatedAt: now,
				},
				Identifier:   domain.UserIdentifier(oldPeer.Email),
				Email:        oldPeer.Email,
				Source:       domain.UserSourceDatabase,
				ProviderName: "",
				IsAdmin:      false,
				Locked:       &now,
				LockedReason: domain.DisabledReasonMigrationDummy,
				Notes:        "created by migration from v1",
			}

			if err := newDb.Save(&user).Error; err != nil {
				return fmt.Errorf("failed to migrate dummy user %s: %w", oldPeer.Email, err)
			}

			logrus.Debugf(" - Dummy User %s migrated", user.Identifier)
		}
		newPeer := domain.Peer{
			BaseModel: domain.BaseModel{
				CreatedBy: "v1migrator",
				UpdatedBy: "v1migrator",
				CreatedAt: oldPeer.CreatedAt,
				UpdatedAt: oldPeer.UpdatedAt,
			},
			Endpoint: domain.StringConfigOption{
				Value: oldPeer.Endpoint, Overridable: !oldPeer.IgnoreGlobalSettings,
			},
			EndpointPublicKey: domain.StringConfigOption{
				Value: iface.PublicKey, Overridable: !oldPeer.IgnoreGlobalSettings,
			},
			AllowedIPsStr: domain.StringConfigOption{
				Value: oldPeer.AllowedIPsStr, Overridable: !oldPeer.IgnoreGlobalSettings,
			},
			ExtraAllowedIPsStr: oldPeer.AllowedIPsSrvStr,
			PresharedKey:       domain.PreSharedKey(oldPeer.PresharedKey),
			PersistentKeepalive: domain.IntConfigOption{
				Value: oldPeer.PersistentKeepalive, Overridable: !oldPeer.IgnoreGlobalSettings,
			},
			DisplayName:         oldPeer.Identifier,
			Identifier:          domain.PeerIdentifier(oldPeer.PublicKey),
			UserIdentifier:      user.Identifier,
			InterfaceIdentifier: iface.Identifier,
			Disabled:            disableTime,
			DisabledReason:      disableReason,
			ExpiresAt:           expiryTime,
			Notes:               "",
			Interface: domain.PeerInterfaceConfig{
				KeyPair: domain.KeyPair{
					PrivateKey: oldPeer.PrivateKey,
					PublicKey:  oldPeer.PublicKey,
				},
				Type:      ifaceType,
				Addresses: ips,
				DnsStr: domain.StringConfigOption{
					Value: oldPeer.DNSStr, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				DnsSearchStr: domain.StringConfigOption{
					Value: iface.PeerDefDnsSearchStr, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				Mtu: domain.IntConfigOption{
					Value: oldPeer.Mtu, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				FirewallMark: domain.Int32ConfigOption{
					Value: iface.PeerDefFirewallMark, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				RoutingTable: domain.StringConfigOption{
					Value: iface.PeerDefRoutingTable, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				PreUp: domain.StringConfigOption{
					Value: iface.PeerDefPreUp, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				PostUp: domain.StringConfigOption{
					Value: iface.PeerDefPostUp, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				PreDown: domain.StringConfigOption{
					Value: iface.PeerDefPreDown, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
				PostDown: domain.StringConfigOption{
					Value: iface.PeerDefPostDown, Overridable: !oldPeer.IgnoreGlobalSettings,
				},
			},
		}

		if err := newDb.Save(&newPeer).Error; err != nil {
			return fmt.Errorf("failed to migrate peer %s (%s): %w", oldPeer.Identifier, oldPeer.PublicKey, err)
		}

		logrus.Debugf(" - Peer %s migrated", newPeer.Identifier)
	}

	return nil
}
