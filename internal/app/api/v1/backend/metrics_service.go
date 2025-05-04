package backend

import (
	"context"
	"fmt"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type MetricsServiceDatabaseRepo interface {
	GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error)
	GetInterfaceStats(ctx context.Context, id domain.InterfaceIdentifier) (
		*domain.InterfaceStatus,
		error,
	)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
}

type MetricsServiceUserManagerRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type MetricsServicePeerManagerRepo interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
}

type MetricsService struct {
	cfg *config.Config

	db    MetricsServiceDatabaseRepo
	users MetricsServiceUserManagerRepo
	peers MetricsServicePeerManagerRepo
}

func NewMetricsService(
	cfg *config.Config,
	db MetricsServiceDatabaseRepo,
	users MetricsServiceUserManagerRepo,
	peers MetricsServicePeerManagerRepo,
) *MetricsService {
	return &MetricsService{
		cfg:   cfg,
		db:    db,
		users: users,
		peers: peers,
	}
}

func (m MetricsService) GetForInterface(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.InterfaceStatus,
	error,
) {
	if !m.cfg.Statistics.CollectInterfaceData {
		return nil, fmt.Errorf("interface statistics collection is disabled")
	}

	// validate admin rights
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	interfaceStats, err := m.db.GetInterfaceStats(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats for interface %s: %w", id, err)
	}

	return interfaceStats, nil
}

func (m MetricsService) GetForUser(ctx context.Context, id domain.UserIdentifier) (
	*domain.User,
	[]domain.PeerStatus,
	error,
) {
	if !m.cfg.Statistics.CollectPeerData {
		return nil, nil, fmt.Errorf("statistics collection is disabled")
	}

	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, nil, err
	}

	user, err := m.users.GetUser(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	peers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch peers for user %s: %w", user.Identifier, err)
	}

	peerIds := make([]domain.PeerIdentifier, len(peers))
	for i, peer := range peers {
		peerIds[i] = peer.Identifier
	}

	peerStats, err := m.db.GetPeersStats(ctx, peerIds...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch peer stats for user %s: %w", user.Identifier, err)
	}

	return user, peerStats, nil
}

func (m MetricsService) GetForPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.PeerStatus, error) {
	if !m.cfg.Statistics.CollectPeerData {
		return nil, fmt.Errorf("peer statistics collection is disabled")
	}

	peer, err := m.peers.GetPeer(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	peerStats, err := m.db.GetPeersStats(ctx, peer.Identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats for peer %s: %w", peer.Identifier, err)
	}

	if len(peerStats) == 0 {
		return nil, fmt.Errorf("no stats found for peer %s: %w", peer.Identifier, domain.ErrNotFound)
	}

	return &peerStats[0], nil
}
