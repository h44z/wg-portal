package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestUpsertUser_SetsCreatedAtWhenZero(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&domain.User{}, &domain.UserAuthentication{}, &domain.UserWebauthnCredential{}))

	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	ui := domain.SystemAdminContextUserInfo()

	user := &domain.User{
		Identifier: "test-user",
		Email:      "test@example.com",
		// CreatedAt is zero
	}

	err := repo.upsertUser(ui, db, user)
	require.NoError(t, err)

	assert.False(t, user.CreatedAt.IsZero(), "CreatedAt should be set when it was zero")
	assert.Equal(t, ui.UserId(), user.CreatedBy, "CreatedBy should be set when it was empty")
	assert.WithinDuration(t, user.UpdatedAt, user.CreatedAt, time.Second, "CreatedAt should be close to UpdatedAt for new user")
}

func TestUpsertUser_PreservesExistingCreatedAt(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&domain.User{}, &domain.UserAuthentication{}, &domain.UserWebauthnCredential{}))

	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	ui := domain.SystemAdminContextUserInfo()

	originalTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	user := &domain.User{
		Identifier: "test-user",
		Email:      "test@example.com",
		BaseModel: domain.BaseModel{
			CreatedAt: originalTime,
			CreatedBy: "original-creator",
		},
	}

	err := repo.upsertUser(ui, db, user)
	require.NoError(t, err)

	assert.Equal(t, originalTime, user.CreatedAt, "CreatedAt should not be overwritten")
	assert.Equal(t, "original-creator", user.CreatedBy, "CreatedBy should not be overwritten")
}

func TestSaveUser_NewUserGetsCreatedAt(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&domain.User{}, &domain.UserAuthentication{}, &domain.UserWebauthnCredential{}))

	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	before := time.Now().Add(-time.Second)

	err := repo.SaveUser(ctx, "new-user", func(u *domain.User) (*domain.User, error) {
		u.Email = "new@example.com"
		return u, nil
	})
	require.NoError(t, err)

	var saved domain.User
	require.NoError(t, db.First(&saved, "identifier = ?", "new-user").Error)

	assert.False(t, saved.CreatedAt.IsZero(), "CreatedAt should not be zero")
	assert.True(t, saved.CreatedAt.After(before), "CreatedAt should be recent")
	assert.NotEmpty(t, saved.CreatedBy, "CreatedBy should be set")
}

func TestMigration_FixesZeroCreatedAt(t *testing.T) {
	db := newTestDB(t)

	// Manually create tables and seed schema version 3
	require.NoError(t, db.AutoMigrate(
		&SysStat{},
		&domain.User{},
		&domain.UserAuthentication{},
		&domain.Interface{},
		&domain.Cidr{},
		&domain.Peer{},
		&domain.AuditEntry{},
		&domain.UserWebauthnCredential{},
	))

	// Insert schema versions 1, 2, 3 so migration starts at 3
	for v := uint64(1); v <= 3; v++ {
		require.NoError(t, db.Create(&SysStat{SchemaVersion: v, MigratedAt: time.Now()}).Error)
	}

	updatedAt := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// Insert a user with zero created_at but valid updated_at
	require.NoError(t, db.Exec(
		"INSERT INTO users (identifier, email, created_at, updated_at) VALUES (?, ?, ?, ?)",
		"zero-user", "zero@example.com", time.Time{}, updatedAt,
	).Error)

	// Run migration
	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	require.NoError(t, repo.migrate())

	// Verify created_at was backfilled from updated_at
	var user domain.User
	require.NoError(t, db.First(&user, "identifier = ?", "zero-user").Error)
	assert.Equal(t, updatedAt, user.CreatedAt, "created_at should be backfilled from updated_at")

	// Verify schema version advanced to 4
	var latest SysStat
	require.NoError(t, db.Order("schema_version DESC").First(&latest).Error)
	assert.Equal(t, uint64(4), latest.SchemaVersion)
}

func TestMigration_DoesNotTouchValidCreatedAt(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.AutoMigrate(
		&SysStat{},
		&domain.User{},
		&domain.UserAuthentication{},
		&domain.Interface{},
		&domain.Cidr{},
		&domain.Peer{},
		&domain.AuditEntry{},
		&domain.UserWebauthnCredential{},
	))

	for v := uint64(1); v <= 3; v++ {
		require.NoError(t, db.Create(&SysStat{SchemaVersion: v, MigratedAt: time.Now()}).Error)
	}

	createdAt := time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	require.NoError(t, db.Exec(
		"INSERT INTO users (identifier, email, created_at, updated_at) VALUES (?, ?, ?, ?)",
		"valid-user", "valid@example.com", createdAt, updatedAt,
	).Error)

	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	require.NoError(t, repo.migrate())

	var user domain.User
	require.NoError(t, db.First(&user, "identifier = ?", "valid-user").Error)
	assert.Equal(t, createdAt, user.CreatedAt, "valid created_at should not be modified")
}
