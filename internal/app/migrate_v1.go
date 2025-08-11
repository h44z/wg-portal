package app

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func migrateFromV1(db *gorm.DB, source, typ string) error {
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
		return fmt.Errorf("unsupported old version, update to database version %s first", latestVersion)
	}

	slog.Info("found valid V1 database", "version", lastVersion.Version)

	// validate target database
	if err := validateTargetDatabase(db); err != nil {
		return fmt.Errorf("target database validation failed: %w", err)
	}

	slog.Info("found valid target database, starting migration...")

	if err := migrateV1Users(oldDb, db); err != nil {
		return fmt.Errorf("user migration failed: %w", err)
	}

	if err := migrateV1Interfaces(oldDb, db); err != nil {
		return fmt.Errorf("user migration failed: %w", err)
	}

	if err := migrateV1Peers(oldDb, db); err != nil {
		return fmt.Errorf("peer migration failed: %w", err)
	}

	slog.Info("migrated V1 database successfully, please restart WireGuard Portal",
		"version", lastVersion.Version)

	return nil
}

// validateTargetDatabase checks if the target database is empty and ready for migration.
func validateTargetDatabase(db *gorm.DB) error {
	var count int64
	err := db.Model(&domain.User{}).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check user table: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("target database contains %d users, please use an empty database for migration", count)
	}

	err = db.Model(&domain.Interface{}).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check interface table: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("target database contains %d interfaces, please use an empty database for migration", count)
	}

	err = db.Model(&domain.Peer{}).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check peer table: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("target database contains %d peers, please use an empty database for migration", count)
	}

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
				CreatedBy: domain.CtxSystemV1Migrator,
				UpdatedBy: domain.CtxSystemV1Migrator,
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

		if err := newDb.Create(&newUser).Error; err != nil {
			return fmt.Errorf("failed to migrate user %s: %w", oldUser.Email, err)
		}

		slog.Debug("user migrated successfully", "identifier", newUser.Identifier)
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
		FirewallMark               uint32
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
				CreatedBy: domain.CtxSystemV1Migrator,
				UpdatedBy: domain.CtxSystemV1Migrator,
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

		// Create new interface with associations
		if err := newDb.Create(&newInterface).Error; err != nil {
			return fmt.Errorf("failed to migrate device %s: %w", oldDevice.DeviceName, err)
		}

		slog.Debug("interface migrated successfully", "identifier", newInterface.Identifier)
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
		err = newDb.First(&user, "identifier = ?",
			oldPeer.Email).Error // migrated users use the email address as identifier
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to find user %s for peer %s: %w", oldPeer.Email, oldPeer.PublicKey, err)
		}
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// create dummy user
			now := time.Now()
			user = domain.User{
				BaseModel: domain.BaseModel{
					CreatedBy: domain.CtxSystemV1Migrator,
					UpdatedBy: domain.CtxSystemV1Migrator,
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

			slog.Debug("dummy user migrated successfully", "identifier", user.Identifier)
		}
		newPeer := domain.Peer{
			BaseModel: domain.BaseModel{
				CreatedBy: domain.CtxSystemV1Migrator,
				UpdatedBy: domain.CtxSystemV1Migrator,
				CreatedAt: oldPeer.CreatedAt,
				UpdatedAt: oldPeer.UpdatedAt,
			},
			Endpoint:            domain.NewConfigOption(oldPeer.Endpoint, !oldPeer.IgnoreGlobalSettings),
			EndpointPublicKey:   domain.NewConfigOption(iface.PublicKey, !oldPeer.IgnoreGlobalSettings),
			AllowedIPsStr:       domain.NewConfigOption(oldPeer.AllowedIPsStr, !oldPeer.IgnoreGlobalSettings),
			ExtraAllowedIPsStr:  oldPeer.AllowedIPsSrvStr,
			PresharedKey:        domain.PreSharedKey(oldPeer.PresharedKey),
			PersistentKeepalive: domain.NewConfigOption(oldPeer.PersistentKeepalive, !oldPeer.IgnoreGlobalSettings),
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
				Type:         ifaceType,
				Addresses:    ips,
				DnsStr:       domain.NewConfigOption(oldPeer.DNSStr, !oldPeer.IgnoreGlobalSettings),
				DnsSearchStr: domain.NewConfigOption(iface.PeerDefDnsSearchStr, !oldPeer.IgnoreGlobalSettings),
				Mtu:          domain.NewConfigOption(oldPeer.Mtu, !oldPeer.IgnoreGlobalSettings),
				FirewallMark: domain.NewConfigOption(iface.PeerDefFirewallMark, !oldPeer.IgnoreGlobalSettings),
				RoutingTable: domain.NewConfigOption(iface.PeerDefRoutingTable, !oldPeer.IgnoreGlobalSettings),
				PreUp:        domain.NewConfigOption(iface.PeerDefPreUp, !oldPeer.IgnoreGlobalSettings),
				PostUp:       domain.NewConfigOption(iface.PeerDefPostUp, !oldPeer.IgnoreGlobalSettings),
				PreDown:      domain.NewConfigOption(iface.PeerDefPreDown, !oldPeer.IgnoreGlobalSettings),
				PostDown:     domain.NewConfigOption(iface.PeerDefPostDown, !oldPeer.IgnoreGlobalSettings),
			},
		}

		if err := newDb.Create(&newPeer).Error; err != nil {
			return fmt.Errorf("failed to migrate peer %s (%s): %w", oldPeer.Identifier, oldPeer.PublicKey, err)
		}

		slog.Debug("peer migrated successfully", "identifier", newPeer.Identifier)
	}

	return nil
}
