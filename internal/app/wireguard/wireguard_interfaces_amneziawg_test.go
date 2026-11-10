package wireguard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type amneziaWGExtrasController struct {
	mockController
	physical domain.PhysicalInterface
}

func (c *amneziaWGExtrasController) SaveInterface(_ context.Context, _ domain.InterfaceIdentifier,
	update func(*domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	physical, err := update(&c.physical)
	if err != nil {
		return err
	}
	c.physical = *physical
	return nil
}

func TestSaveInterfacePersistsAmneziaWGExtras(t *testing.T) {
	for name, options := range map[string]domain.AmneziaWGOptions{
		"omitted options": nil,
		"partial update":  {"Jc": "6"},
	} {
		t.Run(name, func(t *testing.T) {
			controller := &amneziaWGExtrasController{physical: domain.PhysicalInterface{
				Identifier: "awg0", ImportSource: domain.ControllerTypeAmneziaWG,
			}}
			controller.physical.SetExtras(domain.AmneziaWGInterfaceExtras{
				Options: domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4"},
			})
			db := &mockDB{iface: &domain.Interface{
				Identifier: "awg0", Backend: config.AmneziaWGBackendName, Type: domain.InterfaceTypeServer,
			}}
			manager := Manager{
				cfg: &config.Config{}, db: db, bus: &mockBus{},
				wg: &ControllerManager{controllers: map[domain.InterfaceBackend]backendInstance{
					config.AmneziaWGBackendName: {Implementation: controller},
				}},
			}
			update := *db.iface
			update.AmneziaWG = options
			ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
			saved, err := manager.saveInterface(ctx, &update)
			require.NoError(t, err)
			expected := domain.AmneziaWGOptions{"H1": "100-200", "Jc": "4"}
			if options != nil {
				expected["Jc"] = "6"
			}
			require.Equal(t, expected, saved.AmneziaWG)
			require.Equal(t, expected, db.iface.AmneziaWG)
			require.Equal(t, domain.AmneziaWGInterfaceExtras{Options: expected}, controller.physical.GetExtras())
		})
	}
}
