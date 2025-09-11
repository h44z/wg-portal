package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type PeerServicePeerManagerRepo interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
}

type PeerServiceUserManagerRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type PeerService struct {
	cfg *config.Config

	peers PeerServicePeerManagerRepo
	users PeerServiceUserManagerRepo
}

func NewPeerService(
	cfg *config.Config,
	peers PeerServicePeerManagerRepo,
	users PeerServiceUserManagerRepo,
) *PeerService {
	return &PeerService{
		cfg:   cfg,
		peers: peers,
		users: users,
	}
}

func (s PeerService) GetForInterface(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	_, interfacePeers, err := s.peers.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return nil, err
	}

	return interfacePeers, nil
}

func (s PeerService) GetForUser(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	if s.cfg.Advanced.ApiAdminOnly && !domain.GetUserInfo(ctx).IsAdmin {
		return nil, errors.Join(errors.New("only admins can access this endpoint"), domain.ErrNoPermission)
	}

	user, err := s.users.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	userPeers, err := s.peers.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		return nil, err
	}

	return userPeers, nil
}

func (s PeerService) GetById(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	if s.cfg.Advanced.ApiAdminOnly && !domain.GetUserInfo(ctx).IsAdmin {
		return nil, errors.Join(errors.New("only admins can access this endpoint"), domain.ErrNoPermission)
	}

	peer, err := s.peers.GetPeer(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check if the user has access rights to the requested peer.
	// If the peer is not linked to any user, access is granted only for admins.
	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	return peer, nil
}

func (s PeerService) Prepare(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	peer, err := s.peers.PreparePeer(ctx, id)
	if err != nil {
		return nil, err
	}

	return peer, nil
}

func (s PeerService) Create(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	if peer.Identifier != domain.PeerIdentifier(peer.Interface.PublicKey) {
		return nil, fmt.Errorf("peer id mismatch: %s != %s: %w",
			peer.Identifier, peer.Interface.PublicKey, domain.ErrInvalidData)
	}

	createdPeer, err := s.peers.CreatePeer(ctx, peer)
	if err != nil {
		return nil, err
	}

	return createdPeer, nil
}

func (s PeerService) Update(ctx context.Context, _ domain.PeerIdentifier, peer *domain.Peer) (
	*domain.Peer,
	error,
) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	updatedPeer, err := s.peers.UpdatePeer(ctx, peer)
	if err != nil {
		return nil, err
	}

	return updatedPeer, nil
}

func (s PeerService) Delete(ctx context.Context, id domain.PeerIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	err := s.peers.DeletePeer(ctx, id)
	if err != nil {
		return err
	}

	return nil
}

func (s *PeerService) SyncAllPeersFromDB(ctx context.Context) (int, error) {
    type syncer interface {
        SyncAllPeersFromDB(context.Context) (int, error)
    }
    if v, ok := any(s.peers).(syncer); ok {
        return v.SyncAllPeersFromDB(ctx)
    }

    type syncerErrOnly interface {
        SyncAllPeersFromDB(context.Context) error
    }
    if v, ok := any(s.peers).(syncerErrOnly); ok {
        if err := v.SyncAllPeersFromDB(ctx); err != nil {
            return 0, err
        }
        return 0, nil
    }

    return 0, fmt.Errorf("sync not supported by current peers backend")
}