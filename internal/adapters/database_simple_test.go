package adapters

import (
	"context"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func init() {
	schema.RegisterSerializer("encstr", dummySerializer{})
}

type dummySerializer struct{}

func (dummySerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	return nil
}

func (dummySerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue any) (any, error) {
	if fieldValue == nil {
		return nil, nil
	}
	if v, ok := fieldValue.(string); ok {
		return v, nil
	}
	if v, ok := fieldValue.(domain.PreSharedKey); ok {
		return string(v), nil
	}
	return fieldValue, nil
}

func TestSqlRepo_SaveInterface_Simple(t *testing.T) {
	// Initialize in-memory database
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate only what's needed for this test (avoids Peer and its encstr serializer)
	require.NoError(t, db.AutoMigrate(&domain.Interface{}, &domain.Cidr{}))

	repo := &SqlRepo{db: db, cfg: &config.Config{}}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	ifaceId := domain.InterfaceIdentifier("wg0")

	// 1. Create an interface with one address
	addr, _ := domain.CidrFromString("10.0.0.1/24")
	initialIface := &domain.Interface{
		Identifier: ifaceId,
		Addresses:  []domain.Cidr{addr},
	}
	require.NoError(t, db.Create(initialIface).Error)

	// 2. Perform a "partial" update using SaveInterface (this is the buggy path)
	err = repo.SaveInterface(ctx, ifaceId, func(in *domain.Interface) (*domain.Interface, error) {
		in.DisplayName = "New Name"
		return in, nil
	})
	require.NoError(t, err)

	// 3. Verify that the address was NOT deleted
	var finalIface domain.Interface
	require.NoError(t, db.Preload("Addresses").First(&finalIface, "identifier = ?", ifaceId).Error)
	
	require.Equal(t, "New Name", finalIface.DisplayName)
	require.Len(t, finalIface.Addresses, 1, "Address list should still have 1 entry!")
	require.Equal(t, "10.0.0.1/24", finalIface.Addresses[0].Cidr)
}
