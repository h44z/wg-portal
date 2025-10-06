package domain

import "context"

type InterfaceController interface {
	GetId() InterfaceBackend
	GetInterfaces(_ context.Context) ([]PhysicalInterface, error)
	GetInterface(_ context.Context, id InterfaceIdentifier) (*PhysicalInterface, error)
	GetPeers(_ context.Context, deviceId InterfaceIdentifier) ([]PhysicalPeer, error)
	SaveInterface(
		_ context.Context,
		id InterfaceIdentifier,
		updateFunc func(pi *PhysicalInterface) (*PhysicalInterface, error),
	) error
	DeleteInterface(_ context.Context, id InterfaceIdentifier) error
	SavePeer(
		_ context.Context,
		deviceId InterfaceIdentifier,
		id PeerIdentifier,
		updateFunc func(pp *PhysicalPeer) (*PhysicalPeer, error),
	) error
	DeletePeer(_ context.Context, deviceId InterfaceIdentifier, id PeerIdentifier) error
	PingAddresses(
		ctx context.Context,
		addr string,
	) (*PingerResult, error)
}
