package wireguard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// newBareControllerManager builds a manager without running init(), which would
// try to register the local controller and needs privileges this test lacks.
func newBareControllerManager(cfg *config.Config) *ControllerManager {
	return &ControllerManager{
		cfg:         cfg,
		controllers: make(map[domain.InterfaceBackend]backendInstance),
	}
}

func TestRegisterOpnsenseControllers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Backend.Opnsense = []config.BackendOpnsense{
		{
			BackendBase: config.BackendBase{Id: "opn1", DisplayName: "Edge firewall"},
			ApiUrl:      "https://127.0.0.1",
			ApiKey:      "key",
			ApiSecret:   "secret",
		},
	}

	manager := newBareControllerManager(cfg)
	require.NoError(t, manager.registerOpnsenseControllers())

	instance, ok := manager.controllers["opn1"]
	require.True(t, ok, "the opnsense backend should be registered under its configured id")
	assert.Equal(t, "Edge firewall", instance.Config.GetDisplayName())
	require.NotNil(t, instance.Implementation)
	assert.Equal(t, domain.InterfaceBackend("opn1"), instance.Implementation.GetId())

	// The controller must also satisfy the wg-quick and routing contracts, which
	// the wireguard manager relies on when an interface is brought up or down.
	_, isWgQuick := instance.Implementation.(WgQuickController)
	assert.True(t, isWgQuick, "controller must implement WgQuickController")
}

// A backend that claims the reserved "local" id must be skipped rather than
// shadowing the built-in local controller.
func TestRegisterOpnsenseControllersSkipsReservedId(t *testing.T) {
	cfg := &config.Config{}
	cfg.Backend.Opnsense = []config.BackendOpnsense{
		{
			BackendBase: config.BackendBase{Id: config.LocalBackendName},
			ApiUrl:      "https://127.0.0.1",
			ApiKey:      "key",
			ApiSecret:   "secret",
		},
	}

	manager := newBareControllerManager(cfg)
	require.NoError(t, manager.registerOpnsenseControllers())
	assert.Empty(t, manager.controllers, "the reserved id must not be registered")
}

// Missing credentials must fail loudly at startup rather than producing a
// controller that 401s on every call once the portal is running.
func TestRegisterOpnsenseControllersRequiresCredentials(t *testing.T) {
	tests := map[string]config.BackendOpnsense{
		"no url": {
			BackendBase: config.BackendBase{Id: "opn1"},
			ApiKey:      "key", ApiSecret: "secret",
		},
		"no key": {
			BackendBase: config.BackendBase{Id: "opn1"},
			ApiUrl:      "https://127.0.0.1", ApiSecret: "secret",
		},
		"no secret": {
			BackendBase: config.BackendBase{Id: "opn1"},
			ApiUrl:      "https://127.0.0.1", ApiKey: "key",
		},
	}

	for name, backendConfig := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Backend.Opnsense = []config.BackendOpnsense{backendConfig}

			manager := newBareControllerManager(cfg)
			err := manager.registerOpnsenseControllers()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "opn1")
		})
	}
}
