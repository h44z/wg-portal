package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"

	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// SchemaVersion describes the current database schema version. It must be incremented if a manual migration is needed.
var SchemaVersion uint64 = 1

// SysStat stores the current database schema version and the timestamp when it was applied.
type SysStat struct {
	MigratedAt    time.Time `gorm:"column:migrated_at"`
	SchemaVersion uint64    `gorm:"primaryKey,column:schema_version"`
}

// NodeSyncLock provides distributed locking for peer synchronization across nodes.
// Only one node can hold the lock at a time, preventing concurrent syncs that cause
// database contention and 504 Gateway Timeout errors.
type NodeSyncLock struct {
	LockKey   string    `gorm:"primaryKey;column:lock_key"`
	NodeID    string    `gorm:"column:node_id;index"`
	LockedAt  time.Time `gorm:"column:locked_at"`
	ExpiresAt time.Time `gorm:"column:expires_at;index"` // Auto-release stuck locks after 5 minutes
}

func (NodeSyncLock) TableName() string {
	return "node_sync_locks"
}

// GormLogger is a custom logger for Gorm, making it use slog
type GormLogger struct {
	SlowThreshold           time.Duration
	SourceField             string
	IgnoreErrRecordNotFound bool
	Debug                   bool
	Silent                  bool

	prefix string
}

func NewLogger(slowThreshold time.Duration, debug bool) *GormLogger {
	return &GormLogger{
		SlowThreshold:           slowThreshold,
		Debug:                   debug,
		IgnoreErrRecordNotFound: true,
		Silent:                  false,
		SourceField:             "src",
		prefix:                  "GORM-SQL: ",
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	if level == logger.Silent {
		l.Silent = true
	} else {
		l.Silent = false
	}
	return l
}

func (l *GormLogger) Info(ctx context.Context, s string, args ...any) {
	if l.Silent {
		return
	}
	slog.InfoContext(ctx, l.prefix+s, args...)
}

func (l *GormLogger) Warn(ctx context.Context, s string, args ...any) {
	if l.Silent {
		return
	}
	slog.WarnContext(ctx, l.prefix+s, args...)
}

func (l *GormLogger) Error(ctx context.Context, s string, args ...any) {
	if l.Silent {
		return
	}
	slog.ErrorContext(ctx, l.prefix+s, args...)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		"rows", rows,
		"duration", elapsed,
	}

	if l.SourceField != "" {
		attrs = append(attrs, l.SourceField, utils.FileWithLineNum())
	}

	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && l.IgnoreErrRecordNotFound) {
		attrs = append(attrs, "error", err)
		slog.ErrorContext(ctx, l.prefix+sql, attrs...)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold {
		slog.WarnContext(ctx, l.prefix+sql, attrs...)
		return
	}

	if l.Debug {
		slog.DebugContext(ctx, l.prefix+sql, attrs...)
	}
}

// applyConnectionPoolConfig applies the connection pool configuration from config to the database.
func applyConnectionPoolConfig(db *gorm.DB, cfg config.DatabaseConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Use configured values, or defaults if not specified
	maxOpen := cfg.MaxOpenConnections
	if maxOpen <= 0 {
		maxOpen = 50
	}

	maxIdle := cfg.MaxIdleConnections
	if maxIdle <= 0 {
		maxIdle = 10
	}

	maxLifetime := cfg.ConnectionMaxLifetime
	if maxLifetime <= 0 {
		maxLifetime = 3 * time.Minute
	}

	// For multi-node cluster with 24 nodes, ensure adequate pool size
	// Each node may need 2-3 concurrent connections for:
	// - Interface/peer sync operations
	// - Message bus event processing
	// - Regular API requests
	// Recommended: maxOpen = (nodes * 2-3) + buffer, e.g., 24*2.5 = 60
	if maxOpen < 60 {
		slog.Warn("Connection pool size may be too small for multi-node cluster",
			"configured", maxOpen, "recommended_minimum", 60)
	}
	if maxIdle < 10 {
		maxIdle = 10 // Ensure minimum idle connections for cluster
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(maxLifetime)
	// Set connection max idle time to recycle stale connections faster
	// Prevents "too many connections" by recycling idle connections after 5 minutes
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	slog.Info("Database connection pool configured",
		"max_open_connections", maxOpen,
		"max_idle_connections", maxIdle,
		"connection_max_lifetime", maxLifetime.String(),
		"connection_max_idle_time", "5m")

	return nil
}

// NewDatabase creates a new database connection and returns a Gorm database instance.
func NewDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var gormDb *gorm.DB
	var err error

	// Retry logic for initial database connection
	// With 24 nodes starting simultaneously, initial connection can fail with "Too many connections"
	// Increased to 50 retries to handle all 24 nodes with exponential backoff and jitter
	// This gives us up to ~2 minutes of retry time with proper staggering
	const maxRetries = 50
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with random jitter to prevent thundering herd
			// With 24 nodes starting simultaneously, stagger retries much more aggressively
			// Exponential: 50ms, 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s, 6.4s...
			// Jitter: ±100% randomness so not all nodes retry at same time
			baseWaitTime := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			// Cap base wait at 10 seconds to avoid excessive delays
			if baseWaitTime > 10*time.Second {
				baseWaitTime = 10 * time.Second
			}
			// Full randomness jitter to spread retries across time
			jitterAmount := time.Duration(rand.Intn(int(baseWaitTime))) // ±100% jitter
			waitTime := baseWaitTime + jitterAmount

			slog.Warn("database connection attempt failed, retrying with exponential backoff",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"base_wait", baseWaitTime.String(),
				"actual_wait", waitTime.String(),
				"error", lastErr)
			time.Sleep(waitTime)
		}

		switch cfg.Type {
		case config.DatabaseMySQL:
			gormDb, err = gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
				Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
			})
			if err != nil {
				lastErr = fmt.Errorf("failed to open MySQL database: %w", err)
				if strings.Contains(err.Error(), "Too many connections") {
					continue // Retry on connection pool exhaustion
				}
				return nil, lastErr
			}

			// Apply connection pool configuration
			if err := applyConnectionPoolConfig(gormDb, cfg); err != nil {
				lastErr = fmt.Errorf("failed to configure MySQL connection pool: %w", err)
				continue
			}

			// Skip Ping() during initial connection to avoid connection pool exhaustion
			// during multi-node startup. gorm.Open() already validates the connection.
			// Ping would open another connection which might fail with "Too many connections"
			slog.Info("database connected successfully", "type", "MySQL", "attempt", attempt+1)
			return gormDb, nil

		case config.DatabaseMsSQL:
			gormDb, err = gorm.Open(sqlserver.Open(cfg.DSN), &gorm.Config{
				Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
			})
			if err != nil {
				lastErr = fmt.Errorf("failed to open sqlserver database: %w", err)
				if strings.Contains(err.Error(), "Too many connections") {
					continue
				}
				return nil, lastErr
			}

			// Apply connection pool configuration
			if err := applyConnectionPoolConfig(gormDb, cfg); err != nil {
				lastErr = fmt.Errorf("failed to configure MSSQL connection pool: %w", err)
				continue
			}

			slog.Info("database connected successfully", "type", "MSSQL", "attempt", attempt+1)
			return gormDb, nil

		case config.DatabasePostgres:
			gormDb, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
				Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
			})
			if err != nil {
				lastErr = fmt.Errorf("failed to open Postgres database: %w", err)
				if strings.Contains(err.Error(), "too many connections") {
					continue
				}
				return nil, lastErr
			}

			// Apply connection pool configuration
			if err := applyConnectionPoolConfig(gormDb, cfg); err != nil {
				lastErr = fmt.Errorf("failed to configure Postgres connection pool: %w", err)
				continue
			}

			slog.Info("database connected successfully", "type", "Postgres", "attempt", attempt+1)
			return gormDb, nil

		case config.DatabaseSQLite:
			gormDb, err = gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{
				Logger: NewLogger(cfg.SlowQueryThreshold, cfg.Debug),
			})
			if err != nil {
				lastErr = fmt.Errorf("failed to open SQLite database: %w", err)
				continue
			}

			// SQLite doesn't benefit from connection pooling, set to 1
			if err := applyConnectionPoolConfig(gormDb, config.DatabaseConfig{
				MaxOpenConnections:    1,
				MaxIdleConnections:    1,
				ConnectionMaxLifetime: 0,
			}); err != nil {
				lastErr = fmt.Errorf("failed to configure SQLite connection pool: %w", err)
				continue
			}

			slog.Info("database connected successfully", "type", "SQLite", "attempt", attempt+1)
			return gormDb, nil

		default:
			return nil, fmt.Errorf("unknown database type: %s", cfg.Type)
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, lastErr)
}

// SqlRepo is a SQL database repository implementation.
// Currently, it supports MySQL, SQLite, Microsoft SQL and Postgresql database systems.
type SqlRepo struct {
	db              *gorm.DB
	cfg             *config.Config
	metricsCallback func(peerId string) // Optional callback to remove peer metrics
}

// NewSqlRepository creates a new SqlRepo instance.
func NewSqlRepository(db *gorm.DB, cfg *config.Config) (*SqlRepo, error) {
	repo := &SqlRepo{
		db:              db,
		cfg:             cfg,
		metricsCallback: nil, // Can be set via SetMetricsCallback
	}

	if err := repo.preCheck(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return repo, nil
}

func (r *SqlRepo) preCheck() error {
	// WireGuard Portal v1 database migration table
	type DatabaseMigrationInfo struct {
		Version string `gorm:"primaryKey"`
		Applied time.Time
	}

	// temporarily disable logger as the next request might fail (intentionally)
	r.db.Logger.LogMode(logger.Silent)
	defer func() { r.db.Logger.LogMode(logger.Info) }()

	lastVersion := DatabaseMigrationInfo{}
	err := r.db.Order("applied desc, version desc").FirstOrInit(&lastVersion).Error
	if err != nil {
		return nil // we probably don't have a V1 database =)
	}

	return fmt.Errorf("detected a WireGuard Portal V1 database (version: %s) - please migrate first",
		lastVersion.Version)
}

func (r *SqlRepo) migrate() error {
	slog.Debug("running migration: sys-stat", "result", r.db.AutoMigrate(&SysStat{}))
	slog.Debug("running migration: user", "result", r.db.AutoMigrate(&domain.User{}))
	slog.Debug("running migration: user webauthn credentials", "result",
		r.db.AutoMigrate(&domain.UserWebauthnCredential{}))
	slog.Debug("running migration: interface", "result", r.db.AutoMigrate(&domain.Interface{}))
	slog.Debug("running migration: peer", "result", r.db.AutoMigrate(&domain.Peer{}))
	slog.Debug("running migration: peer status", "result", r.db.AutoMigrate(&domain.PeerStatus{}))
	slog.Debug("running migration: interface status", "result", r.db.AutoMigrate(&domain.InterfaceStatus{}))
	slog.Debug("running migration: audit data", "result", r.db.AutoMigrate(&domain.AuditEntry{}))
	slog.Debug("running migration: node sync lock", "result", r.db.AutoMigrate(&NodeSyncLock{}))

	// Clean up deprecated columns from peer_statuses table (traffic accumulation refactor)
	// Previously we had PreviousSessionBytesReceived/Transmitted columns, now we use a simpler approach
	// These columns are safe to drop if they exist
	if r.db.Migrator().HasColumn("peer_statuses", "previous_session_received") {
		slog.Debug("dropping deprecated column", "table", "peer_statuses", "column", "previous_session_received")
		r.db.Migrator().DropColumn("peer_statuses", "previous_session_received")
	}
	if r.db.Migrator().HasColumn("peer_statuses", "previous_session_transmitted") {
		slog.Debug("dropping deprecated column", "table", "peer_statuses", "column", "previous_session_transmitted")
		r.db.Migrator().DropColumn("peer_statuses", "previous_session_transmitted")
	}

	// Clean up accumulated traffic columns (no longer needed - use current session traffic only)
	// Simplified to keep only current session bytes which are accurate from WireGuard
	if r.db.Migrator().HasColumn("peer_statuses", "accumulated_received") {
		slog.Debug("dropping deprecated column", "table", "peer_statuses", "column", "accumulated_received")
		r.db.Migrator().DropColumn("peer_statuses", "accumulated_received")
	}
	if r.db.Migrator().HasColumn("peer_statuses", "accumulated_transmitted") {
		slog.Debug("dropping deprecated column", "table", "peer_statuses", "column", "accumulated_transmitted")
		r.db.Migrator().DropColumn("peer_statuses", "accumulated_transmitted")
	}

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
		slog.Debug("sys-stat entry written", "schema_version", SchemaVersion)
	}

	return nil
}

// region interfaces

// GetInterface returns the interface with the given id.
// If no interface is found, an error domain.ErrNotFound is returned.
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

// GetInterfaceAndPeers returns the interface with the given id and all peers associated with it.
// If no interface is found, an error domain.ErrNotFound is returned.
func (r *SqlRepo) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
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

// GetPeersStats returns the stats for the given peer ids. The order of the returned stats is not guaranteed.
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

// GetAllInterfaces returns all interfaces.
func (r *SqlRepo) GetAllInterfaces(ctx context.Context) ([]domain.Interface, error) {
	var interfaces []domain.Interface

	err := r.db.WithContext(ctx).Preload("Addresses").Find(&interfaces).Error
	if err != nil {
		return nil, err
	}

	return interfaces, nil
}

// GetInterfaceStats returns the stats for the given interface id.
// If no stats are found, an error domain.ErrNotFound is returned.
func (r *SqlRepo) GetInterfaceStats(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.InterfaceStatus,
	error,
) {
	if id == "" {
		return nil, nil
	}

	var stats []domain.InterfaceStatus

	err := r.db.WithContext(ctx).Where("identifier = ?", id).Find(&stats).Error
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return nil, domain.ErrNotFound
	}

	stat := stats[0]

	return &stat, nil
}

// FindInterfaces returns all interfaces that match the given search string.
// The search string is matched against the interface identifier and display name.
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

// SaveInterface updates the interface with the given id.
func (r *SqlRepo) SaveInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(in *domain.Interface) (*domain.Interface, error),
) error {
	userInfo := domain.GetUserInfo(ctx)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		in, err := r.getOrCreateInterface(userInfo, tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		in, err = updateFunc(in)
		if err != nil {
			return err
		}

		err = r.upsertInterface(userInfo, tx, in)
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

func (r *SqlRepo) getOrCreateInterface(
	ui *domain.ContextUserInfo,
	tx *gorm.DB,
	id domain.InterfaceIdentifier,
) (*domain.Interface, error) {
	var in domain.Interface

	// interfaceDefaults will be applied to newly created interface records
	interfaceDefaults := domain.Interface{
		BaseModel: domain.BaseModel{
			CreatedBy: ui.UserId(),
			UpdatedBy: ui.UserId(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Identifier: id,
	}

	err := tx.Preload("Addresses").Attrs(interfaceDefaults).FirstOrCreate(&in, id).Error
	if err != nil {
		return nil, err
	}

	return &in, nil
}

func (r *SqlRepo) upsertInterface(ui *domain.ContextUserInfo, tx *gorm.DB, in *domain.Interface) error {
	in.UpdatedBy = ui.UserId()
	in.UpdatedAt = time.Now()

	err := tx.Save(in).Error
	if err != nil {
		return err
	}

	// Only update addresses if they were explicitly set (not nil)
	// This prevents accidentally deleting all addresses when loading interface without preload
	if in.Addresses != nil {
		err = tx.Model(in).Association("Addresses").Replace(in.Addresses)
		if err != nil {
			return fmt.Errorf("failed to update interface addresses: %w", err)
		}
	}

	return nil
}

// DeleteInterface deletes the interface with the given id.
func (r *SqlRepo) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("interface_identifier = ?", id).Delete(&domain.Peer{}).Error
		if err != nil {
			return err
		}

		err = tx.Delete(&domain.InterfaceStatus{InterfaceId: id}).Error
		if err != nil {
			return err
		}

		err = tx.Select(clause.Associations).Delete(&domain.Interface{Identifier: id}).Error
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

// GetInterfaceIps returns a map of interface identifiers to their respective IP addresses.
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

// GetPeer returns the peer with the given id.
// If no peer is found, an error domain.ErrNotFound is returned.
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

// GetInterfacePeers returns all peers associated with the given interface id.
func (r *SqlRepo) GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	var peers []domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").Where("interface_identifier = ?", id).Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

// FindInterfacePeers returns all peers associated with the given interface id that match the given search string.
// The search string is matched against the peer identifier, display name and IP address.
func (r *SqlRepo) FindInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier, search string) (
	[]domain.Peer,
	error,
) {
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

// GetUserPeers returns all peers associated with the given user id.
func (r *SqlRepo) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	var peers []domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").Where("user_identifier = ?", id).Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

// GetPeersByDisplayName returns all peers with the given display name.
func (r *SqlRepo) GetPeersByDisplayName(ctx context.Context, displayName string) ([]domain.Peer, error) {
	var peers []domain.Peer

	err := r.db.WithContext(ctx).Preload("Addresses").Where("display_name = ?", displayName).Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

// FindUserPeers returns all peers associated with the given user id that match the given search string.
// The search string is matched against the peer identifier, display name and IP address.
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

// SavePeer updates the peer with the given id.
// If no existing peer is found, a new peer is created.
// IMPORTANT: Also creates peer_status record to avoid deadlock during concurrent updates
func (r *SqlRepo) SavePeer(
	ctx context.Context,
	id domain.PeerIdentifier,
	updateFunc func(in *domain.Peer) (*domain.Peer, error),
) error {
	userInfo := domain.GetUserInfo(ctx)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		peer, err := r.getOrCreatePeer(userInfo, tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		peer, err = updateFunc(peer)
		if err != nil {
			return err
		}

		// CRITICAL: Ensure TTLLocked is set to true if peer has explicit future expiration date
		// This handles cases where:
		// 1. User sets ExpiresAt via API but TTLLocked wasn't set (legacy or update scenario)
		// 2. Peer was created with explicit future date (far in future, beyond 1 hour)
		// We use 1 hour threshold to distinguish explicit dates from "now + DefaultUserTTL"
		if peer.ExpiresAt != nil && peer.ExpiresAt.After(time.Now().Add(1*time.Hour)) && !peer.TTLLocked {
			slog.Debug("automatically locking TTL for peer with explicit future expiration",
				"peer", id,
				"expires_at", peer.ExpiresAt.Format(time.RFC3339))
			peer.TTLLocked = true
		}

		err = r.upsertPeer(userInfo, tx, peer)
		if err != nil {
			return err
		}

		// DEADLOCK FIX: Ensure peer_status record exists WITHOUT FOR UPDATE lock
		// This prevents lock contention when stats collection immediately tries to update it
		// CRITICAL: Reset peer_status if peer was recreated with same ID to avoid inheriting old stats
		// (e.g., if old peer was online, new peer would show as online until stats update)
		// IMPORTANT: Also remove metrics for the peer being recreated to avoid duplicate metric values
		// When a peer is recreated with same ID, old label values remain in Prometheus registry
		// This causes metrics to show x3 values (old labels + new labels + potential dups)
		var existingStatus domain.PeerStatus
		statusExists := tx.Where("identifier = ?", id).First(&existingStatus).Error == nil

		if statusExists {
			// IMPORTANT: Remove old metrics BEFORE resetting status
			// This ensures we don't have duplicate label values in Prometheus
			r.removeMetricsForPeer(string(id))

			// Reset existing status to clean state for new peer
			cleanStatus := domain.PeerStatus{
				PeerId:           id,
				UpdatedAt:        time.Now(),
				IsConnected:      false,
				IsPingable:       false,
				LastHandshake:    nil,
				Endpoint:         "",
				LastSessionStart: nil,
				BytesReceived:    0,
				BytesTransmitted: 0,
				OwnerNodeId:      "",
			}
			err = tx.Where("identifier = ?", id).Save(&cleanStatus).Error
			if err != nil {
				slog.Debug("failed to reset peer status", "peer", id, "error", err)
			}
		} else {
			// Create new status record for new peer
			peerStatus := domain.PeerStatus{
				PeerId:    id,
				UpdatedAt: time.Now(),
			}
			err = tx.Create(&peerStatus).Error
			if err != nil {
				slog.Debug("peer status record creation deferred", "peer", id, "error", err)
			}
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// removeMetricsForPeer removes metrics from Prometheus registry for a peer being recreated
// This prevents duplicate metric values when peer is recreated with same ID
// Calls the registered callback if one exists
func (r *SqlRepo) removeMetricsForPeer(peerId string) {
	if r.metricsCallback != nil {
		r.metricsCallback(peerId)
	}
}

// SetMetricsCallback sets the callback for removing peer metrics
// This is called when a peer is recreated to clean up old metrics in Prometheus
// Should be called during initialization with the metrics server's RemovePeerMetricsByID function
func (r *SqlRepo) SetMetricsCallback(callback func(peerId string)) {
	r.metricsCallback = callback
}

func (r *SqlRepo) getOrCreatePeer(ui *domain.ContextUserInfo, tx *gorm.DB, id domain.PeerIdentifier) (
	*domain.Peer,
	error,
) {
	var peer domain.Peer

	// interfaceDefaults will be applied to newly created interface records
	interfaceDefaults := domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedBy: ui.UserId(),
			UpdatedBy: ui.UserId(),
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

func (r *SqlRepo) upsertPeer(ui *domain.ContextUserInfo, tx *gorm.DB, peer *domain.Peer) error {
	peer.UpdatedBy = ui.UserId()
	peer.UpdatedAt = time.Now()

	err := tx.Save(peer).Error
	if err != nil {
		return err
	}

	err = tx.Model(peer).Association("Addresses").Replace(peer.Interface.Addresses)
	if err != nil {
		return fmt.Errorf("failed to update peer addresses: %w", err)
	}

	return nil
}

// DeletePeer deletes the peer with the given id.
// This also deletes the peer_addresses associations (many-to-many relationships with Cidr)
// NOTE: We do NOT delete peer_status here.
// The peer_status will be cleaned up by CleanOrphanedStatuses on all cluster nodes.
// This ensures that other nodes can detect orphaned statuses and clean up their metrics.
func (r *SqlRepo) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First: delete peer_addresses many2many associations to avoid foreign key issues
		// This is critical because we use raw SQL and explicit deletion like in DeletePeersByIDs
		if err := tx.Table("peer_addresses").Where("peer_identifier = ?", string(id)).Delete(nil).Error; err != nil {
			return err
		}

		// Second: delete the peer itself
		err := tx.Where("identifier = ?", string(id)).Delete(&domain.Peer{}).Error
		if err != nil {
			return err
		}

		// Third: delete peer_status to avoid orphaned records
		if err := tx.Where("identifier = ?", string(id)).Delete(&domain.PeerStatus{}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Remove Prometheus metrics AFTER database transaction succeeds
	// This prevents orphaned metrics for deleted peers
	if r.metricsCallback != nil {
		r.metricsCallback(string(id))
	}

	return nil
}

// GetPeerIps returns a map of peer identifiers to their respective IP addresses.
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

// GetUsedIpsPerSubnet returns a map of subnets to their respective used IP addresses.
// Does NOT use locks - relies on retry logic when unique constraint violations occur.
func (r *SqlRepo) GetUsedIpsPerSubnet(ctx context.Context, subnets []domain.Cidr) (
	map[domain.Cidr][]domain.Cidr,
	error,
) {
	var peerIps []struct {
		domain.Cidr
		PeerId domain.PeerIdentifier `gorm:"column:peer_identifier"`
	}

	// Read peer addresses without lock - allocation uses sequence-based approach with retry on conflict
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

	// Read interface addresses without lock
	err = r.db.WithContext(ctx).
		Table("interface_addresses").
		Joins("LEFT JOIN cidrs ON interface_addresses.cidr_cidr = cidrs.cidr").
		Scan(&interfaceIps).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch interface IP's: %w", err)
	}

	result := make(map[domain.Cidr][]domain.Cidr, len(subnets))
	for _, ip := range interfaceIps {
		var subnet domain.Cidr // default empty subnet (if no subnet matches, we will add the IP to the empty subnet group)
		for _, s := range subnets {
			if s.Contains(ip.Cidr) {
				subnet = s
				break
			}
		}
		result[subnet] = append(result[subnet], ip.Cidr)
	}
	for _, ip := range peerIps {
		var subnet domain.Cidr // default empty subnet (if no subnet matches, we will add the IP to the empty subnet group)
		for _, s := range subnets {
			if s.Contains(ip.Cidr) {
				subnet = s
				break
			}
		}
		result[subnet] = append(result[subnet], ip.Cidr)
	}
	return result, nil
}

// GetNextPeerIPForSubnet returns the next available IP address for a peer in the given subnet.
// Uses sequence-based allocation: reads the last allocated IP and returns next.
// No locks - relies on INSERT unique constraint to prevent duplicates.
// On conflict, caller retries with next IP.
func (r *SqlRepo) GetNextPeerIPForSubnet(ctx context.Context, subnet domain.Cidr) (domain.Cidr, error) {
	var lastIP struct {
		Addr string `gorm:"column:addr"`
	}

	// Find highest IP currently allocated in this subnet
	err := r.db.WithContext(ctx).
		Table("peer_addresses").
		Joins("LEFT JOIN cidrs ON peer_addresses.cidr_cidr = cidrs.cidr").
		Where("cidrs.cidr = ?", subnet.Cidr).
		Order("cidrs.addr DESC").
		Limit(1).
		Scan(&lastIP).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return domain.Cidr{}, fmt.Errorf("failed to get last allocated IP: %w", err)
	}

	// If no IP found, start from network base + 1
	if lastIP.Addr == "" {
		start := subnet.NextAddr()
		return start.HostAddr(), nil
	}

	// Parse last IP and return next
	lastCidr, err := domain.CidrFromString(lastIP.Addr)
	if err != nil {
		return domain.Cidr{}, fmt.Errorf("failed to parse last IP: %w", err)
	}

	nextIP := lastCidr.NextAddr()
	if !nextIP.IsValid() {
		return domain.Cidr{}, fmt.Errorf("ip space on subnet %s is exhausted", subnet.String())
	}

	return nextIP.HostAddr(), nil
}

// endregion peers

// region users

// GetUser returns the user with the given id.
// If no user is found, an error domain.ErrNotFound is returned.
func (r *SqlRepo) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	var user domain.User

	err := r.db.WithContext(ctx).Preload("WebAuthnCredentialList").First(&user, id).Error

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByEmail returns the user with the given email.
// If no user is found, an error domain.ErrNotFound is returned.
// If multiple users are found, an error domain.ErrNotUnique is returned.
func (r *SqlRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var users []domain.User

	err := r.db.WithContext(ctx).Where("email = ?", email).Preload("WebAuthnCredentialList").Find(&users).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, domain.ErrNotFound
	}

	if len(users) > 1 {
		return nil, fmt.Errorf("found multiple users with email %s: %w", email, domain.ErrNotUnique)
	}

	user := users[0]

	return &user, nil
}

// GetUserByWebAuthnCredential returns the user with the given webauthn credential id.
func (r *SqlRepo) GetUserByWebAuthnCredential(ctx context.Context, credentialIdBase64 string) (*domain.User, error) {
	var credential domain.UserWebauthnCredential

	err := r.db.WithContext(ctx).Where("credential_identifier = ?", credentialIdBase64).First(&credential).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return r.GetUser(ctx, domain.UserIdentifier(credential.UserIdentifier))
}

// GetAllUsers returns all users.
func (r *SqlRepo) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	var users []domain.User

	err := r.db.WithContext(ctx).Preload("WebAuthnCredentialList").Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

// FindUsers returns all users that match the given search string.
// The search string is matched against the user identifier, firstname, lastname and email.
func (r *SqlRepo) FindUsers(ctx context.Context, search string) ([]domain.User, error) {
	var users []domain.User

	searchValue := "%" + strings.ToLower(search) + "%"
	err := r.db.WithContext(ctx).
		Where("identifier LIKE ?", searchValue).
		Or("firstname LIKE ?", searchValue).
		Or("lastname LIKE ?", searchValue).
		Or("email LIKE ?", searchValue).
		Preload("WebAuthnCredentialList").
		Find(&users).Error
	if err != nil {
		return nil, err
	}

	return users, nil
}

// SaveUser updates the user with the given id.
// If no user is found, a new user is created.
func (r *SqlRepo) SaveUser(
	ctx context.Context,
	id domain.UserIdentifier,
	updateFunc func(u *domain.User) (*domain.User, error),
) error {
	userInfo := domain.GetUserInfo(ctx)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := r.getOrCreateUser(userInfo, tx, id)
		if err != nil {
			return err // return any error will roll back
		}

		user, err = updateFunc(user)
		if err != nil {
			return err
		}

		err = r.upsertUser(userInfo, tx, user)
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

// DeleteUser deletes the user with the given id.
func (r *SqlRepo) DeleteUser(ctx context.Context, id domain.UserIdentifier) error {
	err := r.db.WithContext(ctx).Unscoped().Select(clause.Associations).Delete(&domain.User{Identifier: id}).Error
	if err != nil {
		return err
	}

	return nil
}

func (r *SqlRepo) getOrCreateUser(ui *domain.ContextUserInfo, tx *gorm.DB, id domain.UserIdentifier) (
	*domain.User,
	error,
) {
	var user domain.User

	// userDefaults will be applied to newly created user records
	userDefaults := domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: ui.UserId(),
			UpdatedBy: ui.UserId(),
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

func (r *SqlRepo) upsertUser(ui *domain.ContextUserInfo, tx *gorm.DB, user *domain.User) error {
	user.UpdatedBy = ui.UserId()
	user.UpdatedAt = time.Now()

	err := tx.Save(user).Error
	if err != nil {
		return err
	}

	err = tx.Session(&gorm.Session{FullSaveAssociations: true}).Unscoped().Model(user).Association("WebAuthnCredentialList").Unscoped().Replace(user.WebAuthnCredentialList)
	if err != nil {
		return fmt.Errorf("failed to update users webauthn credentials: %w", err)
	}

	return nil
}

// endregion users

// region statistics

// UpdateInterfaceStatus updates the interface status with the given id.
// If no interface status is found, a new one is created.
func (r *SqlRepo) UpdateInterfaceStatus(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(in *domain.InterfaceStatus) (*domain.InterfaceStatus, error),
) error {
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

func (r *SqlRepo) getOrCreateInterfaceStatus(tx *gorm.DB, id domain.InterfaceIdentifier) (
	*domain.InterfaceStatus,
	error,
) {
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

// UpdatePeerStatus updates the peer status with the given id.
// If no peer status is found, a new one is created.
// OWNERSHIP CHECK: If peer has an OwnerNodeId set, skip update to prevent conflicts with owner node.
// Includes automatic retry logic with exponential backoff for deadlock/conflict recovery.
// Uses row-level FOR UPDATE locking to prevent concurrent modification conflicts.
func (r *SqlRepo) UpdatePeerStatus(
	ctx context.Context,
	id domain.PeerIdentifier,
	updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error),
) error {
	// Retry logic with exponential backoff for deadlocks and conflicts
	// Increased to 10 retries (50ms, 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s, 6.4s, 12.8s, 25.6s)
	// This handles high-concurrency scenarios with 24 cluster nodes and 100+ peers
	// Total max wait time: ~50 seconds for all retries
	const maxRetries = 10
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter to reduce synchronized retries
			// Base: 50ms * 2^(attempt-1) + random 0-50ms jitter
			baseWait := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			jitter := time.Duration(rand.Intn(50)) * time.Millisecond
			waitTime := baseWait + jitter
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			in, err := r.getOrCreatePeerStatus(tx, id)
			if err != nil {
				return err // return any error will roll back
			}

			// Save original state to restore conflicting fields if peer is owned by another node
			oldIsConnected := in.IsConnected
			oldIsPingable := in.IsPingable
			oldOwnerNodeId := in.OwnerNodeId

			// IMPORTANT: Always allow updateFunc to proceed!
			// This ensures non-conflicting data (BytesReceived, BytesTransmitted, LastHandshake, Endpoint)
			// is captured for ALL nodes, not just the owner.
			// We'll selectively restore conflicting fields below if owned by another node.
			in, err = updateFunc(in)
			if err != nil {
				return err
			}

			// OWNERSHIP CHECK: If peer is CONNECTED and owned by another node,
			// restore the conflicting state fields to prevent this node from overwriting owner's state
			// BUT keep non-conflicting data like bytes and endpoint
			if oldIsConnected && oldOwnerNodeId != "" && oldOwnerNodeId != r.cfg.Core.ClusterNodeId {
				slog.Info("peer status owned by other node, restoring conflicting fields but KEEPING bytes",
					"peer", id,
					"owner", oldOwnerNodeId,
					"our_node", r.cfg.Core.ClusterNodeId,
					"restored_is_connected", oldIsConnected,
					"bytes_received", in.BytesReceived,
					"bytes_transmitted", in.BytesTransmitted,
					"last_handshake", in.LastHandshake,
					"endpoint", in.Endpoint)
				// Restore conflicting fields to prevent state conflicts
				in.IsConnected = oldIsConnected
				in.IsPingable = oldIsPingable
				in.OwnerNodeId = oldOwnerNodeId
				// Keep the non-conflicting data:
				// - BytesReceived, BytesTransmitted (traffic data)
				// - LastHandshake, Endpoint (connection details)
				// - LastSessionStart, LastPing (timestamps)
			} else {
				// Peer is either offline or not owned by another node - safe to save all data
				slog.Debug("peer status update - saving all data",
					"peer", id,
					"is_connected", in.IsConnected,
					"owned_by", in.OwnerNodeId,
					"bytes_received", in.BytesReceived,
					"bytes_transmitted", in.BytesTransmitted)
			}

			// CRITICAL: When peer transitions to OFFLINE, clear its ownership
			// This allows any node to update it in future cycles
			if !in.IsConnected && in.OwnerNodeId != "" {
				slog.Debug("clearing peer ownership on offline transition",
					"peer", id, "old_owner", in.OwnerNodeId)
				in.OwnerNodeId = ""
			}

			err = r.upsertPeerStatus(tx, in)
			if err != nil {
				return err
			}

			// return nil will commit the whole transaction
			return nil
		})
		if err == nil {
			return nil // success
		}

		lastErr = err
		// Check if this is a retryable error (deadlock or record changed)
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Deadlock") && !strings.Contains(errMsg, "Record has changed") {
			return err // not a retryable error
		}
		// Continue to retry
	}

	return fmt.Errorf("UpdatePeerStatus failed after %d retries: %w", maxRetries, lastErr)
}

// ClaimPeerStatus claims ownership of a peer status for this node.
// Only the owner node should update peer status to avoid conflicts.
// Once claimed, only this node (and nodes with same owner_node_id) can update it.
// Uses row-level FOR UPDATE locking to serialize concurrent claims.
func (r *SqlRepo) ClaimPeerStatus(
	ctx context.Context,
	id domain.PeerIdentifier,
	ownerNodeId string,
	updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error),
) error {
	if ownerNodeId == "" {
		return fmt.Errorf("ownerNodeId cannot be empty")
	}

	// Retry logic with exponential backoff for deadlocks and conflicts
	// Increased to 10 retries for high-concurrency multi-node ownership claiming
	// Total max wait time: ~50 seconds for all retries
	const maxRetries = 10
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter to reduce synchronized retries
			// Base: 50ms * 2^(attempt-1) + random 0-50ms jitter
			baseWait := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			jitter := time.Duration(rand.Intn(50)) * time.Millisecond
			waitTime := baseWait + jitter
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			in, err := r.getOrCreatePeerStatus(tx, id)
			if err != nil {
				return err
			}

			// Claim ownership: set before calling updateFunc
			in.OwnerNodeId = ownerNodeId
			in.UpdatedAt = time.Now()

			// Apply the custom update function
			in, err = updateFunc(in)
			if err != nil {
				return err
			}

			// Ensure ownership is maintained
			in.OwnerNodeId = ownerNodeId

			err = r.upsertPeerStatus(tx, in)
			if err != nil {
				return err
			}

			return nil
		})
		if err == nil {
			return nil // success
		}

		lastErr = err
		// Check if this is a retryable error (deadlock or record changed)
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Deadlock") && !strings.Contains(errMsg, "Record has changed") {
			return err // not a retryable error
		}
		// Continue to retry
	}

	return fmt.Errorf("ClaimPeerStatus failed after %d retries: %w", maxRetries, lastErr)
}

// BatchUpdatePeerStatuses updates multiple peer statuses in a single optimized transaction.
// This is more efficient than calling UpdatePeerStatus individually and reduces deadlock risk.
// OWNERSHIP CHECK: Skips peers owned by other nodes to prevent conflicts.
// Uses ON CONFLICT DO UPDATE for bulk upsert with automatic conflict resolution.
func (r *SqlRepo) BatchUpdatePeerStatuses(
	ctx context.Context,
	updates map[domain.PeerIdentifier]func(in *domain.PeerStatus) (*domain.PeerStatus, error),
) error {
	if len(updates) == 0 {
		return nil
	}

	// Retry logic for batch operations - more retries for bulk operations
	const maxRetries = 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 50ms, 100ms, 200ms, 400ms, 800ms
			waitTime := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var allStatuses []*domain.PeerStatus

			// Load all peer statuses we need to update
			for id := range updates {
				in, err := r.getOrCreatePeerStatus(tx, id)
				if err != nil {
					return err
				}

				// OWNERSHIP CHECK: Skip peers owned by other nodes
				if in.OwnerNodeId != "" {
					// Peer is owned by another node - skip update
					slog.Debug("peer status owned by other node, skipping batch update",
						"peer", id, "owner", in.OwnerNodeId)
					continue
				}

				// Apply the update function
				updateFunc := updates[id]
				in, err = updateFunc(in)
				if err != nil {
					return err
				}

				allStatuses = append(allStatuses, in)
			}

			// Batch insert/update all statuses in one operation
			if len(allStatuses) > 0 {
				err := tx.Clauses(clause.OnConflict{
					UpdateAll: true,
				}).CreateInBatches(allStatuses, 50).Error // batch insert in groups of 50
				if err != nil {
					return err
				}
			}

			return nil
		})
		if err == nil {
			return nil // success
		}

		lastErr = err
		// Check if this is a retryable error
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Deadlock") && !strings.Contains(errMsg, "Record has changed") {
			return err // not a retryable error
		}
		// Continue to retry
	}

	return fmt.Errorf("BatchUpdatePeerStatuses failed after %d retries: %w", maxRetries, lastErr)
}

func (r *SqlRepo) getOrCreatePeerStatus(tx *gorm.DB, id domain.PeerIdentifier) (*domain.PeerStatus, error) {
	var in domain.PeerStatus

	// defaults will be applied to newly created record
	defaults := domain.PeerStatus{
		PeerId:    id,
		UpdatedAt: time.Now(),
	}

	// DEADLOCK FIX: Two-phase approach to reduce lock contention
	// 1. First try to get existing record with FOR UPDATE lock
	//    (This avoids gap locks on non-existent rows)
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identifier = ?", id).First(&in).Error
	if err == nil {
		// Record found and locked - proceed with update
		return &in, nil
	}

	if err != gorm.ErrRecordNotFound {
		// Unexpected error
		return nil, err
	}

	// Record doesn't exist - create it WITHOUT FOR UPDATE lock
	// This is safe because SavePeer pre-creates peer_status, so this is rare
	// Explicit WHERE clause using correct column name
	err = tx.Where("identifier = ?", id).Attrs(defaults).FirstOrCreate(&in).Error
	if err != nil {
		return nil, err
	}

	// Now that record exists, acquire FOR UPDATE lock for modifications
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identifier = ?", id).First(&in).Error
	if err != nil {
		return nil, fmt.Errorf("failed to lock peer status after creation: %w", err)
	}

	return &in, nil
}

func (r *SqlRepo) getOrCreatePeerStatusForRead(tx *gorm.DB, id domain.PeerIdentifier) (*domain.PeerStatus, error) {
	var in domain.PeerStatus

	// defaults will be applied to newly created record
	defaults := domain.PeerStatus{
		PeerId:    id,
		UpdatedAt: time.Now(),
	}

	// For read-only access without modifications (used in conditionals)
	// No FOR UPDATE lock - allows concurrent reads
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

// DeletePeerStatus deletes the peer status with the given id.
func (r *SqlRepo) DeletePeerStatus(ctx context.Context, id domain.PeerIdentifier) error {
	err := r.db.WithContext(ctx).Delete(&domain.PeerStatus{}, id).Error
	if err != nil {
		return err
	}

	// Remove Prometheus metrics when peer status is deleted
	// This prevents orphaned metrics for peers that no longer exist
	if r.metricsCallback != nil {
		r.metricsCallback(string(id))
	}

	return nil
}

// GetAllPeerStatuses returns all peer statuses from the database.
func (r *SqlRepo) GetAllPeerStatuses(ctx context.Context) ([]domain.PeerStatus, error) {
	var statuses []domain.PeerStatus

	err := r.db.WithContext(ctx).Find(&statuses).Error
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// endregion statistics

// region audit

// SaveAuditEntry saves the given audit entry.
func (r *SqlRepo) SaveAuditEntry(ctx context.Context, entry *domain.AuditEntry) error {
	err := r.db.WithContext(ctx).Save(entry).Error
	if err != nil {
		return err
	}

	return nil
}

// GetAllAuditEntries retrieves all audit entries from the database.
// The entries are ordered by timestamp, with the newest entries first.
func (r *SqlRepo) GetAllAuditEntries(ctx context.Context) ([]domain.AuditEntry, error) {
	var entries []domain.AuditEntry
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&entries).Error
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// endregion audit

func (r *SqlRepo) GetAllPeers(ctx context.Context) ([]domain.Peer, error) {
	var peers []domain.Peer
	// OPTIMIZED: Select only peer identifiers and basic fields to avoid N+1 queries
	// from Preload("Addresses") and Preload("Interface") which would create 600+ queries
	// This method is used mainly for cleanup validation, not for full peer data display
	err := r.db.WithContext(ctx).
		Select("id, identifier, display_name, interface_identifier").
		Find(&peers).Error
	if err != nil {
		return nil, err
	}

	return peers, nil
}

// region node sync lock

const (
	syncLockKey      = "peer:sync"
	syncLockDuration = 5 * time.Minute // Auto-release stuck locks after 5 minutes
	syncLockTimeout  = 2 * time.Minute // Total wait time for acquiring lock
)

// AcquireSyncLock attempts to acquire the global peer sync lock.
// Returns nodeID that holds the lock, or empty string if we acquired it.
// Uses exponential backoff to retry up to syncLockTimeout.
func (r *SqlRepo) AcquireSyncLock(ctx context.Context, nodeID string) (acquiredBy string, err error) {
	// Use generic lock mechanism
	return r.AcquireLock(ctx, syncLockKey, nodeID, syncLockDuration)
}

// ReleaseSyncLock releases the global peer sync lock.
func (r *SqlRepo) ReleaseSyncLock(ctx context.Context, nodeID string) error {
	return r.ReleaseLock(ctx, syncLockKey, nodeID)
}

// SyncAllPeersFromDBWithLock wraps sync to ensure only one node syncs at a time.
// This prevents database contention and 504 Gateway Timeout cascades.
func (r *SqlRepo) SyncAllPeersFromDBWithLock(ctx context.Context, nodeID string) (int, error) {
	// Try to acquire lock with timeout
	lockCtx, cancel := context.WithTimeout(ctx, syncLockTimeout)
	defer cancel()

	heldBy, err := r.AcquireSyncLock(lockCtx, nodeID)
	if err != nil {
		slog.Warn("[SYNC_LOCK] cannot acquire sync lock, another node is syncing",
			"held_by", heldBy, "self", nodeID, "error", err)
		// Return 0,0 to indicate no sync performed (not an error condition)
		return 0, nil
	}

	// Ensure lock is released after we're done (even if sync fails)
	defer func() {
		if relErr := r.ReleaseSyncLock(context.Background(), nodeID); relErr != nil {
			slog.Error("failed to release sync lock", "error", relErr)
		}
		// Clean up expired locks in background to prevent table bloat
		go func() {
			goCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if cleanErr := r.CleanupExpiredLocks(goCtx); cleanErr != nil {
				slog.Debug("cleanup expired locks failed", "error", cleanErr)
			}
		}()
	}()

	// Now perform the actual sync with the lock held
	count, err := r.SyncAllPeersFromDB(ctx)
	if err != nil {
		slog.Error("[SYNC_LOCK] sync failed while holding lock", "node_id", nodeID, "error", err)
		return count, err
	}

	slog.Info("[SYNC_LOCK] sync completed", "node_id", nodeID, "synced_count", count)
	return count, nil
}

// endregion node sync lock

// SyncAllPeersFromDB synchronizes all peers from the database.
func (r *SqlRepo) SyncAllPeersFromDB(ctx context.Context) (int, error) {
	slog.Debug("SyncAllPeersFromDB called, but no operation performed")
	return 0, nil
}

// region expired peers cleanup

const (
	expireCleanupLockKey      = "expire:cleanup"
	expireCleanupLockDuration = 10 * time.Minute // Lock to ensure only one node performs cleanup
	expireCleanupTimeout      = 30 * time.Second // Timeout for acquiring lock
)

// GetExpiredPeers finds all peers with expiredAt in the past
func (r *SqlRepo) GetExpiredPeers(ctx context.Context) ([]domain.Peer, error) {
	var peers []domain.Peer
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Select("identifier, interface_identifier").
		Find(&peers).Error

	if err != nil {
		return nil, err
	}

	return peers, nil
}

// DeletePeersByIDs deletes peers by their Identifier (public key) and their associated addresses
// Used when cleaning up expired peers
// IMPORTANT: Deletes peer_addresses associations first to avoid foreign key constraint violations
// Also deletes peer statuses to avoid orphaned status records
func (r *SqlRepo) DeletePeersByIDs(ctx context.Context, peerIDs []string) (int64, error) {
	if len(peerIDs) == 0 {
		return 0, nil
	}

	// Start transaction to ensure atomic deletion (associations first, then peers, then statuses)
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}

	// First: delete peer_addresses many2many associations via raw SQL
	if err := tx.Table("peer_addresses").Where("peer_identifier IN ?", peerIDs).Delete(nil).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Second: delete peer statuses to avoid orphaned status records
	if err := tx.Where("identifier IN ?", peerIDs).Delete(&domain.PeerStatus{}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// Third: delete peers
	result := tx.Where("identifier IN ?", peerIDs).Delete(&domain.Peer{})
	if result.Error != nil {
		tx.Rollback()
		return 0, result.Error
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	// Remove Prometheus metrics for all deleted peers
	// This prevents orphaned metrics for bulk-deleted peers
	if r.metricsCallback != nil {
		for _, peerID := range peerIDs {
			r.metricsCallback(peerID)
		}
	}

	return result.RowsAffected, nil
}

// FindAndDeleteExpiredPeersWithLock ensures only MASTER node deletes expired peers
// Other nodes skip cleanup to avoid conflicts across multiple nodes
// Under high load, returns error to defer deletion to next cycle instead of blocking
func (r *SqlRepo) FindAndDeleteExpiredPeersWithLock(ctx context.Context, nodeID string) (expiredPeerIDs []string, err error) {
	// Only MASTER node can delete expired peers
	if !r.cfg.Core.Master {
		slog.Debug("[EXPIRE_CLEANUP] this node is not master, skipping cleanup", "node_id", nodeID)
		return nil, nil
	}

	// Create a child context with 30-second timeout for the entire cleanup operation
	// If deletion takes too long (high load), we defer to next cycle to avoid blocking
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Find expired peers
	expiredPeers, err := r.GetExpiredPeers(cleanupCtx)
	if err != nil {
		slog.ErrorContext(cleanupCtx, "[EXPIRE_CLEANUP] failed to get expired peers", "error", err)
		if cleanupCtx.Err() != nil {
			return nil, fmt.Errorf("expired peer lookup timeout (high load condition): %w", cleanupCtx.Err())
		}
		return nil, err
	}

	if len(expiredPeers) == 0 {
		slog.Debug("[EXPIRE_CLEANUP] no expired peers found")
		return nil, nil
	}

	// Delete them from DB
	peerIDs := make([]string, len(expiredPeers))
	for i, p := range expiredPeers {
		peerIDs[i] = string(p.Identifier)
	}

	deletedCount, err := r.DeletePeersByIDs(cleanupCtx, peerIDs)
	if err != nil {
		slog.ErrorContext(cleanupCtx, "[EXPIRE_CLEANUP] failed to delete expired peers", "error", err, "count", len(peerIDs))
		// Return timeout error to signal high load condition
		if cleanupCtx.Err() != nil {
			return nil, fmt.Errorf("expired peer deletion timeout (high load condition): %w", cleanupCtx.Err())
		}
		return nil, err
	}

	slog.Info("[EXPIRE_CLEANUP] deleted expired peers",
		"node_id", nodeID, "count", deletedCount, "peer_ids_count", len(peerIDs))

	return peerIDs, nil
}

// endregion expired peers cleanup

// region generic distributed locking (for reuse)

// AcquireLock is a generic lock mechanism for various operations.
// IMPORTANT: Uses simple INSERT without ON DUPLICATE KEY UPDATE to avoid deadlocks.
// If lock already exists and is not expired, returns who holds it (no retry loop).
func (r *SqlRepo) AcquireLock(ctx context.Context, lockKey string, nodeID string, duration time.Duration) (acquiredBy string, err error) {
	now := time.Now()

	// Clean up expired locks (but don't retry on failure)
	if delErr := r.db.WithContext(ctx).
		Where("lock_key = ? AND expires_at < ?", lockKey, now).
		Delete(&NodeSyncLock{}).Error; delErr != nil {
		slog.Debug("failed to clean expired locks", "lock_key", lockKey, "error", delErr)
	}

	// Try simple INSERT (no ON DUPLICATE KEY UPDATE to avoid deadlock)
	result := r.db.WithContext(ctx).
		Create(&NodeSyncLock{
			LockKey:   lockKey,
			NodeID:    nodeID,
			LockedAt:  now,
			ExpiresAt: now.Add(duration),
		})

	if result.Error == nil {
		slog.Debug("[LOCK] acquired", "lock_key", lockKey, "node_id", nodeID)
		return "", nil
	}

	// INSERT failed - check who holds the lock (no retry, just check)
	var lock NodeSyncLock
	if err := r.db.WithContext(ctx).
		Where("lock_key = ?", lockKey).
		First(&lock).Error; err == nil {
		if lock.ExpiresAt.Before(now) {
			// Lock expired, clean it up (best effort)
			r.db.WithContext(ctx).Delete(&lock)
			return "", fmt.Errorf("lock expired, please retry")
		}
		// Lock is held by another node
		acquiredBy = lock.NodeID
		return acquiredBy, fmt.Errorf("lock held by %s", acquiredBy)
	}

	// Couldn't determine lock holder
	return "", fmt.Errorf("failed to acquire lock %s", lockKey)
}

// ReleaseLock releases a distributed lock
func (r *SqlRepo) ReleaseLock(ctx context.Context, lockKey string, nodeID string) error {
	// Only delete lock if it belongs to us AND it hasn't expired yet
	// This prevents deadlocks from competing DELETE operations on expired locks
	result := r.db.WithContext(ctx).
		Where("lock_key = ? AND node_id = ? AND expires_at > NOW()", lockKey, nodeID).
		Delete(&NodeSyncLock{})

	if result.Error != nil {
		// Log but don't fail - lock might already be released or expired
		slog.Debug("failed to release lock", "lockKey", lockKey, "nodeID", nodeID, "error", result.Error)
		return nil
	}

	if result.RowsAffected == 0 {
		// Lock was already released or expired - not an error
		slog.Debug("lock already released or expired", "lockKey", lockKey, "nodeID", nodeID)
		return nil
	}

	return nil
}

// CleanupExpiredLocks removes all expired locks from the database
// This prevents deadlocks from piling up expired lock rows
func (r *SqlRepo) CleanupExpiredLocks(ctx context.Context) error {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&NodeSyncLock{})

	if result.Error != nil {
		slog.Warn("failed to cleanup expired locks", "error", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		slog.Debug("cleaned up expired locks", "rows_deleted", result.RowsAffected)
	}

	return nil
}

// endregion generic distributed locking
