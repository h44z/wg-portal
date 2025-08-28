package wireguard

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/fedor-git/wg-portal-2/internal/adapters/wgcontroller"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type InterfaceController interface {
	GetId() domain.InterfaceBackend
	GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error)
	GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error)
	GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error)
	SaveInterface(
		_ context.Context,
		id domain.InterfaceIdentifier,
		updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
	) error
	DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error
	SavePeer(
		_ context.Context,
		deviceId domain.InterfaceIdentifier,
		id domain.PeerIdentifier,
		updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
	) error
	DeletePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error
	PingAddresses(
		ctx context.Context,
		addr string,
	) (*domain.PingerResult, error)
}

type backendInstance struct {
	Config         config.BackendBase // Config is the configuration for the backend instance.
	Implementation InterfaceController
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
			Id:          config.LocalBackendName,
			DisplayName: "Local WireGuard Controller",
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

func (c *ControllerManager) logRegisteredControllers() {
	for backend, controller := range c.controllers {
		slog.Debug("backend controller registered",
			"backend", backend, "type", fmt.Sprintf("%T", controller.Implementation))
	}
}

func (c *ControllerManager) GetControllerByName(backend domain.InterfaceBackend) InterfaceController {
	return c.getController(backend, "")
}

func (c *ControllerManager) GetController(iface domain.Interface) InterfaceController {
	return c.getController(iface.Backend, iface.Identifier)
}

func (c *ControllerManager) getController(
	backend domain.InterfaceBackend,
	ifaceId domain.InterfaceIdentifier,
) InterfaceController {
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
	return controller.Implementation
}

func (c *ControllerManager) GetAllControllers() []InterfaceController {
	var backendInstances = make([]InterfaceController, 0, len(c.controllers))
	for instance := range maps.Values(c.controllers) {
		backendInstances = append(backendInstances, instance.Implementation)
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
