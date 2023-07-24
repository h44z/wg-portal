package adapters

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// SchemaVersion describes the current database schema version. It must be incremented if a manual migration is needed.
var SchemaVersion uint64 = 1

// SysStat stores the current database schema version and the timestamp when it was applied.
type SysStat struct {
	MigratedAt    time.Time `gorm:"column:migrated_at"`
	SchemaVersion uint64    `gorm:"primaryKey,column:schema_version"`
}

// GormLogger is a custom logger for Gorm, making it use logrus.
type GormLogger struct {
	SlowThreshold           time.Duration
	SourceField             string
	IgnoreErrRecordNotFound bool
	Debug                   bool
}

func NewLogger(slowThreshold time.Duration, debug bool) *GormLogger {
	return &GormLogger{
		SlowThreshold:           slowThreshold,
		Debug:                   debug,
		IgnoreErrRecordNotFound: true,
		SourceField:             "src",
	}
}

func (l *GormLogger) LogMode(logger.LogLevel) logger.Interface {
	return l
}

func (l *GormLogger) Info(ctx context.Context, s string, args ...interface{}) {
	logrus.WithContext(ctx).Infof(s, args)
}

func (l *GormLogger) Warn(ctx context.Context, s string, args ...interface{}) {
	logrus.WithContext(ctx).Warnf(s, args)
}

func (l *GormLogger) Error(ctx context.Context, s string, args ...interface{}) {
	logrus.WithContext(ctx).Errorf(s, args)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	fields := logrus.Fields{
		"rows":     rows,
		"duration": elapsed,
	}
	if l.SourceField != "" {
		fields[l.SourceField] = utils.FileWithLineNum()
	}
	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && l.IgnoreErrRecordNotFound) {
		fields[logrus.ErrorKey] = err
		logrus.WithContext(ctx).WithFields(fields).Errorf("%s", sql)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold {
		logrus.WithContext(ctx).WithFields(fields).Warnf("%s", sql)
		return
	}

	if l.Debug {
		logrus.WithContext(ctx).WithFields(fields).Tracef("%s", sql)
	}
}

func NewDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var gormDb *gorm.DB
	var err error

	switch cfg.Type {
	case config.DatabaseMySQL:
		gormDb, err = gorm.Open(gormMySQL.Open(cfg.DSN), &gorm.Config{
			Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open MySQL database: %w", err)
		}

		sqlDB, _ := gormDb.DB()
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(10)
		err = sqlDB.Ping() // This DOES open a connection if necessary. This makes sure the database is accessible
		if err != nil {
			return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
		}
	case config.DatabaseMsSQL:
		gormDb, err = gorm.Open(sqlserver.Open(cfg.DSN), &gorm.Config{
			Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlserver database: %w", err)
		}
	case config.DatabasePostgres:
		gormDb, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
			Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open Postgres database: %w", err)
		}
	case config.DatabaseSQLite:
		if _, err = os.Stat(filepath.Dir(cfg.DSN)); os.IsNotExist(err) {
			if err = os.MkdirAll(filepath.Dir(cfg.DSN), 0700); err != nil {
				return nil, fmt.Errorf("failed to create database base directory: %w", err)
			}
		}
		gormDb, err = gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{
			Logger:                                   NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", err)
		}
		sqlDB, _ := gormDb.DB()
		sqlDB.SetMaxOpenConns(1)
	}

	return gormDb, nil
}

// SqlRepo is a SQL database repository implementation.
// Currently, it supports MySQL, SQLite, Microsoft SQL and Postgresql database systems.
type SqlRepo struct {
	db *gorm.DB
}

func NewSqlRepository(db *gorm.DB) (*SqlRepo, error) {
	repo := &SqlRepo{
		db: db,
	}

	err := repo.migrate()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return repo, nil
}

func (r *SqlRepo) migrate() error {
	logrus.Tracef("sysstat migration: %v", r.db.AutoMigrate(&SysStat{}))
	logrus.Tracef("user migration: %v", r.db.AutoMigrate(&domain.User{}))
	logrus.Tracef("interface migration: %v", r.db.AutoMigrate(&domain.Interface{}))
	logrus.Tracef("peer migration: %v", r.db.AutoMigrate(&domain.Peer{}))
	logrus.Tracef("peer status migration: %v", r.db.AutoMigrate(&domain.PeerStatus{}))
	logrus.Tracef("interface status migration: %v", r.db.AutoMigrate(&domain.InterfaceStatus{}))
	logrus.Tracef("audit data migration: %v", r.db.AutoMigrate(&domain.AuditEntry{}))

	existingSysStat := SysStat{}
	r.db.Where("schema_version = ?", SchemaVersion).First(&existingSysStat)
	if existingSysStat.SchemaVersion == 0 {
		sysStat := SysStat{
			MigratedAt:    time.Now(),
			SchemaVersion: SchemaVersion,
		}
		if err := r.db.Create(&sysStat).Error; err != nil {
			return fmt.Errorf("failed to write sysstat entry for schema version %d: %w", SchemaVersion, err)
		}
		logrus.Debugf("sysstat entry for schema version %d written", SchemaVersion)
	}

	return nil
}

// region interfaces

func (r *SqlRepo) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error) {
	var in domain.Interface

	err := r.db.WithContext(ctx).Preload("Addresses").First(&in, id).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &in, nil
}

func (r *SqlRepo) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error) {
	in, err := r.GetInterface(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load interface: %w", err)
	}

	peers, err := r.GetInterfacePeers(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load peers: %w", err)
	}

	return in, peers, nil
}

func (r *SqlRepo) GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var stats []domain.PeerStatus

	err := r.db.WithContext(ctx).Where("identifier IN ?", ids).Find(&stats).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *SqlRepo) GetAllInterfaces(ctx context.Context) ([]domain.Interface, error) {
	var interfaces []domain.Interface

	err := r.db.WithContext(ctx).Preload("Addresses").Find(&interfaces).Error
	if err != nil {
		return nil, err
	}

	return interfaces, nil
}

func (r *SqlRepo) FindInterfaces(ctx context.Context, search string) ([]domain.Interface, error) {
	var users []domain.Interface

	searchValue := "%" + strings.ToLower(search) + "%"
	err := r.db.WithContext(ctx).
		Where("identifier LIKE ?", searchValue).
		Or("display_name LIKE ?", searchValue).
		Preload("Addresses").
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r *SqlRepo) SaveInterface(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.Interface) (*domain.Interface, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		in, err := r.getOrCreateInterface(tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		in, err = updateFunc(in)
		if err != nil {
			return err
		}

		err = r.upsertInterface(tx, in)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreateInterface(tx *gorm.DB, id domain.InterfaceIdentifier) (*domain.Interface, error) {
	var in domain.Interface

	// interfaceDefaults will be applied to newly created interface records
	interfaceDefaults := domain.Interface{
		BaseModel: domain.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Identifier: id,
	}

	err := tx.Attrs(interfaceDefaults).FirstOrCreate(&in, id).Error
	if err != nil {
		return nil, err
	}

	return &in, nil
}

func (r *SqlRepo) upsertInterface(tx *gorm.DB, in *domain.Interface) error {
	err := tx.Save(in).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := r.db.WithContext(ctx).Where("interface_identifier = ?", id).Delete(&domain.Peer{}).Error
		if err != nil {
			return err
		}

		err = r.db.WithContext(ctx).Delete(&domain.InterfaceStatus{InterfaceId: id}).Error
		if err != nil {
			return err
		}

		err = r.db.WithContext(ctx).Debug().Select(clause.Associations).Delete(&domain.Interface{Identifier: id}).Error
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error) {
	var ips []struct {
		domain.Cidr
		InterfaceId domain.InterfaceIdentifier `gorm:"column:interface_identifier"`
	}

	err := r.db.WithContext(ctx).
		Table("interface_addresses").
		Joins("LEFT JOIN cidrs ON interface_addresses.cidr_cidr = cidrs.cidr").
		Scan(&ips).Error
	if err != nil {
		return nil, err
	}

	result := make(map[domain.InterfaceIdentifier][]domain.Cidr)
	for _, ip := range ips {
		result[ip.InterfaceId] = append(result[ip.InterfaceId], ip.Cidr)
	}
	return result, nil
}

// endregion interfaces

// region peers

func (r *SqlRepo) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	var peer domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").First(&peer, id).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &peer, nil
}

func (r *SqlRepo) GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	var peers []domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").Where("interface_identifier = ?", id).Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (r *SqlRepo) FindInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier, search string) ([]domain.Peer, error) {
	var peers []domain.Peer

	searchValue := "%" + strings.ToLower(search) + "%"
	err := r.db.WithContext(ctx).Where("interface_identifier = ?", id).
		Where("identifier LIKE ?", searchValue).
		Or("display_name LIKE ?", searchValue).
		Or("iface_address_str_v LIKE ?", searchValue).
		Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (r *SqlRepo) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	var peers []domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").Where("user_identifier = ?", id).Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (r *SqlRepo) FindUserPeers(ctx context.Context, id domain.UserIdentifier, search string) ([]domain.Peer, error) {
	var peers []domain.Peer

	searchValue := "%" + strings.ToLower(search) + "%"
	err := r.db.WithContext(ctx).Where("user_identifier = ?", id).
		Where("identifier LIKE ?", searchValue).
		Or("display_name LIKE ?", searchValue).
		Or("iface_address_str_v LIKE ?", searchValue).
		Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

func (r *SqlRepo) SavePeer(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.Peer) (*domain.Peer, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		peer, err := r.getOrCreatePeer(tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		peer, err = updateFunc(peer)
		if err != nil {
			return err
		}

		err = r.upsertPeer(tx, peer)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreatePeer(tx *gorm.DB, id domain.PeerIdentifier) (*domain.Peer, error) {
	var peer domain.Peer

	// interfaceDefaults will be applied to newly created interface records
	interfaceDefaults := domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Identifier: id,
	}

	err := tx.Attrs(interfaceDefaults).FirstOrCreate(&peer, id).Error
	if err != nil {
		return nil, err
	}

	return &peer, nil
}

func (r *SqlRepo) upsertPeer(tx *gorm.DB, peer *domain.Peer) error {
	err := tx.Save(peer).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := r.db.WithContext(ctx).Delete(&domain.PeerStatus{PeerId: id}).Error
		if err != nil {
			return err
		}

		err = r.db.WithContext(ctx).Select(clause.Associations).Delete(&domain.Peer{Identifier: id}).Error
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) GetPeerIps(ctx context.Context) (map[domain.PeerIdentifier][]domain.Cidr, error) {
	var ips []struct {
		domain.Cidr
		PeerId domain.PeerIdentifier `gorm:"column:peer_identifier"`
	}

	err := r.db.WithContext(ctx).
		Table("peer_addresses").
		Joins("LEFT JOIN cidrs ON peer_addresses.cidr_cidr = cidrs.cidr").
		Scan(&ips).Error
	if err != nil {
		return nil, err
	}

	result := make(map[domain.PeerIdentifier][]domain.Cidr)
	for _, ip := range ips {
		result[ip.PeerId] = append(result[ip.PeerId], ip.Cidr)
	}
	return result, nil
}

func (r *SqlRepo) GetUsedIpsPerSubnet(ctx context.Context) (map[domain.Cidr][]domain.Cidr, error) {
	var peerIps []struct {
		domain.Cidr
		PeerId domain.PeerIdentifier `gorm:"column:peer_identifier"`
	}

	err := r.db.WithContext(ctx).
		Table("peer_addresses").
		Joins("LEFT JOIN cidrs ON peer_addresses.cidr_cidr = cidrs.cidr").
		Scan(&peerIps).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peer IP's: %w", err)
	}

	var interfaceIps []struct {
		domain.Cidr
		InterfaceId domain.InterfaceIdentifier `gorm:"column:interface_identifier"`
	}

	err = r.db.WithContext(ctx).
		Table("interface_addresses").
		Joins("LEFT JOIN cidrs ON interface_addresses.cidr_cidr = cidrs.cidr").
		Scan(&interfaceIps).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch interface IP's: %w", err)
	}

	result := make(map[domain.Cidr][]domain.Cidr)
	for _, ip := range interfaceIps {
		networkAddr := ip.Cidr.NetworkAddr()
		result[networkAddr] = append(result[networkAddr], ip.Cidr)
	}
	for _, ip := range peerIps {
		networkAddr := ip.Cidr.NetworkAddr()
		result[networkAddr] = append(result[networkAddr], ip.Cidr)
	}
	return result, nil
}

// endregion peers

// region users

func (r *SqlRepo) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	var user domain.User

	err := r.db.WithContext(ctx).First(&user, id).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *SqlRepo) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	var users []domain.User

	err := r.db.WithContext(ctx).Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r *SqlRepo) FindUsers(ctx context.Context, search string) ([]domain.User, error) {
	var users []domain.User

	searchValue := "%" + strings.ToLower(search) + "%"
	err := r.db.WithContext(ctx).
		Where("identifier LIKE ?", searchValue).
		Or("firstname LIKE ?", searchValue).
		Or("lastname LIKE ?", searchValue).
		Or("email LIKE ?", searchValue).
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r *SqlRepo) SaveUser(ctx context.Context, id domain.UserIdentifier, updateFunc func(u *domain.User) (*domain.User, error)) error {
	userInfo := domain.GetUserInfo(ctx)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := r.getOrCreateUser(string(userInfo.Id), tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		user, err = updateFunc(user)
		if err != nil {
			return err
		}

		user.UpdatedAt = time.Now()
		user.UpdatedBy = string(userInfo.Id)

		err = r.upsertUser(tx, user)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) DeleteUser(ctx context.Context, id domain.UserIdentifier) error {
	err := r.db.WithContext(ctx).Delete(&domain.User{}, id).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreateUser(creator string, tx *gorm.DB, id domain.UserIdentifier) (*domain.User, error) {
	var user domain.User

	// userDefaults will be applied to newly created user records
	userDefaults := domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: creator,
			UpdatedBy: creator,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Identifier: id,
		Source:     domain.UserSourceDatabase,
		IsAdmin:    false,
	}

	err := tx.Attrs(userDefaults).FirstOrCreate(&user, id).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *SqlRepo) upsertUser(tx *gorm.DB, user *domain.User) error {
	err := tx.Save(user).Error
	if err != nil {
		return err
	}

	return nil
}

// endregion users

// region statistics

func (r *SqlRepo) UpdateInterfaceStatus(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.InterfaceStatus) (*domain.InterfaceStatus, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		in, err := r.getOrCreateInterfaceStatus(tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		in, err = updateFunc(in)
		if err != nil {
			return err
		}

		err = r.upsertInterfaceStatus(tx, in)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreateInterfaceStatus(tx *gorm.DB, id domain.InterfaceIdentifier) (*domain.InterfaceStatus, error) {
	var in domain.InterfaceStatus

	// defaults will be applied to newly created record
	defaults := domain.InterfaceStatus{
		InterfaceId: id,
		UpdatedAt:   time.Now(),
	}

	err := tx.Attrs(defaults).FirstOrCreate(&in, id).Error
	if err != nil {
		return nil, err
	}

	return &in, nil
}

func (r *SqlRepo) upsertInterfaceStatus(tx *gorm.DB, in *domain.InterfaceStatus) error {
	err := tx.Save(in).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) UpdatePeerStatus(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		in, err := r.getOrCreatePeerStatus(tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		in, err = updateFunc(in)
		if err != nil {
			return err
		}

		err = r.upsertPeerStatus(tx, in)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreatePeerStatus(tx *gorm.DB, id domain.PeerIdentifier) (*domain.PeerStatus, error) {
	var in domain.PeerStatus

	// defaults will be applied to newly created record
	defaults := domain.PeerStatus{
		PeerId:    id,
		UpdatedAt: time.Now(),
	}

	err := tx.Attrs(defaults).FirstOrCreate(&in, id).Error
	if err != nil {
		return nil, err
	}

	return &in, nil
}

func (r *SqlRepo) upsertPeerStatus(tx *gorm.DB, in *domain.PeerStatus) error {
	err := tx.Save(in).Error
	if err != nil {
		return err
	}

	return nil
}

// endregion statistics

// region audit

func (r *SqlRepo) SaveAuditEntry(ctx context.Context, entry *domain.AuditEntry) error {
	err := r.db.WithContext(ctx).Save(entry).Error
	if err != nil {
		return err
	}

	return nil
}

// endregion audit
