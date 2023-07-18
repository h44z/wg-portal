package wireguard

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
)

type InterfaceAndPeerDatabaseRepo interface {
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error)
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	FindInterfaces(ctx context.Context, search string) ([]domain.Interface, error)
	GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error)
	SaveInterface(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.Interface) (*domain.Interface, error)) error
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
	FindInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier, search string) ([]domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	FindUserPeers(ctx context.Context, id domain.UserIdentifier, search string) ([]domain.Peer, error)
	SavePeer(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.Peer) (*domain.Peer, error)) error
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUsedIpsPerSubnet(ctx context.Context) (map[domain.Cidr][]domain.Cidr, error)
}

type StatisticsDatabaseRepo interface {
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)

	UpdatePeerStatus(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error)) error
	UpdateInterfaceStatus(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.InterfaceStatus) (*domain.InterfaceStatus, error)) error
}

type InterfaceController interface {
	GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error)
	GetInterface(_ context.Context, id domain.InterfaceIdentifier) (*domain.PhysicalInterface, error)
	GetPeers(_ context.Context, deviceId domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error)
	GetPeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) (*domain.PhysicalPeer, error)
	SaveInterface(_ context.Context, id domain.InterfaceIdentifier, updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error)) error
	DeleteInterface(_ context.Context, id domain.InterfaceIdentifier) error
	SavePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier, updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error)) error
	DeletePeer(_ context.Context, deviceId domain.InterfaceIdentifier, id domain.PeerIdentifier) error
}
