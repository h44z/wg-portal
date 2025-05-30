package wgcontroller

import (
	"context"

	"github.com/h44z/wg-portal/internal/domain"
)

type MikrotikController struct {
}

func NewMikrotikController() (*MikrotikController, error) {
	return &MikrotikController{}, nil
}

// region wireguard-related

func (c MikrotikController) GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error) {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) (
	[]domain.PhysicalPeer,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) SaveInterface(
	_ context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) SavePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) DeletePeer(
	_ context.Context,
	deviceId domain.InterfaceIdentifier,
	id domain.PeerIdentifier,
) error {
	// TODO implement me
	panic("implement me")
}

// endregion wireguard-related

// region wg-quick-related

func (c MikrotikController) ExecuteInterfaceHook(id domain.InterfaceIdentifier, hookCmd string) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) SetDNS(id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) UnsetDNS(id domain.InterfaceIdentifier) error {
	// TODO implement me
	panic("implement me")
}

// endregion wg-quick-related

// region routing-related

func (c MikrotikController) SyncRouteRules(_ context.Context, rules []domain.RouteRule) error {
	// TODO implement me
	panic("implement me")
}

func (c MikrotikController) DeleteRouteRules(_ context.Context, rules []domain.RouteRule) error {
	// TODO implement me
	panic("implement me")
}

// endregion routing-related
