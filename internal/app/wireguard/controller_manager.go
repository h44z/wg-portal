package wireguard

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/h44z/wg-portal/internal/adapters/wgcontroller"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type backendInstance struct {
	Config         config.BackendBase // Config is the configuration for the backend instance.
	Implementation domain.InterfaceController
}

type ControllerManager struct {
	cfg         *config.Config
	controllers map[domain.InterfaceBackend]backendInstance
}

func NewControllerManager(cfg *config.Config) (*ControllerManager, error) {
	c := &ControllerManager{
		cfg:         cfg,
		controllers: make(map[domain.InterfaceBackend]backendInstance),
	}

	err := c.init()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ControllerManager) init() error {
	if err := c.registerLocalController(); err != nil {
		return err
	}

	if err := c.registerMikrotikControllers(); err != nil {
		return err
	}

	if err := c.registerPfsenseControllers(); err != nil {
		return err
	}

	c.logRegisteredControllers()

	return nil
}

func (c *ControllerManager) registerLocalController() error {
	localController, err := wgcontroller.NewLocalController(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to create local WireGuard controller: %w", err)
	}

	c.controllers[config.LocalBackendName] = backendInstance{
		Config: config.BackendBase{
			Id:                config.LocalBackendName,
			DisplayName:       "Local WireGuard Controller",
			IgnoredInterfaces: c.cfg.Backend.IgnoredLocalInterfaces,
		},
		Implementation: localController,
	}
	return nil
}

func (c *ControllerManager) registerMikrotikControllers() error {
	for _, backendConfig := range c.cfg.Backend.Mikrotik {
		if backendConfig.Id == config.LocalBackendName {
			slog.Warn("skipping registration of Mikrotik controller with reserved ID", "id", config.LocalBackendName)
			continue
		}

		controller, err := wgcontroller.NewMikrotikController(c.cfg, &backendConfig)
		if err != nil {
			return fmt.Errorf("failed to create Mikrotik controller for backend %s: %w", backendConfig.Id, err)
		}

		c.controllers[domain.InterfaceBackend(backendConfig.Id)] = backendInstance{
			Config:         backendConfig.BackendBase,
			Implementation: controller,
		}
	}
	return nil
}

func (c *ControllerManager) registerPfsenseControllers() error {
	for _, backendConfig := range c.cfg.Backend.Pfsense {
		if backendConfig.Id == config.LocalBackendName {
			slog.Warn("skipping registration of pfSense controller with reserved ID", "id", config.LocalBackendName)
			continue
		}

		controller, err := wgcontroller.NewPfsenseController(c.cfg, &backendConfig)
		if err != nil {
			return fmt.Errorf("failed to create pfSense controller for backend %s: %w", backendConfig.Id, err)
		}

		c.controllers[domain.InterfaceBackend(backendConfig.Id)] = backendInstance{
			Config:         backendConfig.BackendBase,
			Implementation: controller,
		}
	}
	return nil
}

func (c *ControllerManager) logRegisteredControllers() {
	for backend, controller := range c.controllers {
		slog.Debug("backend controller registered",
			"backend", backend, "type", fmt.Sprintf("%T", controller.Implementation))
	}
}

func (c *ControllerManager) GetControllerByName(backend domain.InterfaceBackend) domain.InterfaceController {
	return c.getController(backend, "").Implementation
}

func (c *ControllerManager) GetController(iface domain.Interface) domain.InterfaceController {
	return c.getController(iface.Backend, iface.Identifier).Implementation
}

func (c *ControllerManager) getController(
	backend domain.InterfaceBackend,
	ifaceId domain.InterfaceIdentifier,
) backendInstance {
	if backend == "" {
		// If no backend is specified, use the local controller.
		// This might be the case for interfaces created in previous WireGuard Portal versions.
		backend = config.LocalBackendName
	}

	controller, exists := c.controllers[backend]
	if !exists {
		controller, exists = c.controllers[config.LocalBackendName] // Fallback to local controller
		if !exists {
			// If the local controller is also not found, panic
			panic(fmt.Sprintf("%s interface controller for backend %s not found", ifaceId, backend))
		}
		slog.Warn("controller for backend not found, using local controller",
			"backend", backend, "interface", ifaceId)
	}
	return controller
}

func (c *ControllerManager) GetAllControllers() []backendInstance {
	var backendInstances = make([]backendInstance, 0, len(c.controllers))
	for instance := range maps.Values(c.controllers) {
		backendInstances = append(backendInstances, instance)
	}
	return backendInstances
}

func (c *ControllerManager) GetControllerNames() []config.BackendBase {
	var names []config.BackendBase
	for _, id := range slices.Sorted(maps.Keys(c.controllers)) {
		names = append(names, c.controllers[id].Config)
	}

	return names
}
