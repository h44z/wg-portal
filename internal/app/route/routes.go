package route

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type ControllerManager interface {
	// GetController returns the controller for the given interface.
	GetController(iface domain.Interface) domain.InterfaceController
}

type InterfaceAndPeerDatabaseRepo interface {
	// GetInterface returns the interface with the given identifier.
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
}

type EventBus interface {
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
}

type RoutesController interface {
	// SetRoutes sets the routes for the given interface. If no routes are provided, the function is a no-op.
	SetRoutes(ctx context.Context, info domain.RoutingTableInfo) error
	// RemoveRoutes removes the routes for the given interface. If no routes are provided, the function is a no-op.
	RemoveRoutes(ctx context.Context, info domain.RoutingTableInfo) error
}

// endregion dependencies

type routeRuleInfo struct {
	ifaceId    domain.InterfaceIdentifier
	fwMark     uint32
	table      int
	family     int
	hasDefault bool
}

// Manager is try to mimic wg-quick behaviour (https://git.zx2c4.com/wireguard-tools/tree/src/wg-quick/linux.bash)
// for default routes.
type Manager struct {
	cfg *config.Config

	bus          EventBus
	db           InterfaceAndPeerDatabaseRepo
	wgController ControllerManager

	mux *sync.Mutex
}

// NewRouteManager creates a new route manager instance.
func NewRouteManager(
	cfg *config.Config,
	bus EventBus,
	db InterfaceAndPeerDatabaseRepo,
	wgController ControllerManager,
) (*Manager, error) {
	m := &Manager{
		cfg: cfg,
		bus: bus,

		db:           db,
		wgController: wgController,
		mux:          &sync.Mutex{},
	}

	m.connectToMessageBus()

	return m, nil
}

func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicRouteUpdate, m.handleRouteUpdateEvent)
	_ = m.bus.Subscribe(app.TopicRouteRemove, m.handleRouteRemoveEvent)
}

// StartBackgroundJobs starts background jobs for the route manager.
// This method is non-blocking and returns immediately.
func (m Manager) StartBackgroundJobs(_ context.Context) {
	// this is a no-op for now
}

func (m Manager) handleRouteUpdateEvent(info domain.RoutingTableInfo) {
	m.mux.Lock() // ensure that only one route update is processed at a time
	defer m.mux.Unlock()

	slog.Debug("handling route update event", "info", info.String())

	if !info.ManagementEnabled() {
		return // route management disabled
	}

	err := m.syncRoutes(context.Background(), info)
	if err != nil {
		slog.Error("failed to synchronize routes",
			"info", info.String(), "error", err)
		return
	}

	slog.Debug("routes synchronized", "info", info.String())
}

func (m Manager) handleRouteRemoveEvent(info domain.RoutingTableInfo) {
	m.mux.Lock() // ensure that only one route update is processed at a time
	defer m.mux.Unlock()

	slog.Debug("handling route remove event", "info", info.String())

	if !info.ManagementEnabled() {
		return // route management disabled
	}

	err := m.removeRoutes(context.Background(), info)
	if err != nil {
		slog.Error("failed to synchronize routes",
			"info", info.String(), "error", err)
		return
	}

	slog.Debug("routes removed", "info", info.String())
}

func (m Manager) syncRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	rc, ok := m.wgController.GetController(info.Interface).(RoutesController)
	if !ok {
		slog.Warn("no capable routes-controller found for interface", "interface", info.Interface.Identifier)
		return nil
	}

	if !info.Interface.ManageRoutingTable() {
		slog.Debug("interface does not manage routing table, skipping route update",
			"interface", info.Interface.Identifier)
		return nil
	}

	err := rc.SetRoutes(ctx, info)
	if err != nil {
		return fmt.Errorf("failed to set routes for interface %s: %w", info.Interface.Identifier, err)
	}
	return nil
}

func (m Manager) removeRoutes(ctx context.Context, info domain.RoutingTableInfo) error {
	rc, ok := m.wgController.GetController(info.Interface).(RoutesController)
	if !ok {
		slog.Warn("no capable routes-controller found for interface", "interface", info.Interface.Identifier)
		return nil
	}

	if !info.Interface.ManageRoutingTable() {
		slog.Debug("interface does not manage routing table, skipping route removal",
			"interface", info.Interface.Identifier)
		return nil
	}

	err := rc.RemoveRoutes(ctx, info)
	if err != nil {
		return fmt.Errorf("failed to remove routes for interface %s: %w", info.Interface.Identifier, err)
	}
	return nil
}
