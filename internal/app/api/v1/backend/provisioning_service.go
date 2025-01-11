package backend

import (
	"context"
	"fmt"
	"io"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type ProvisioningServiceUserManagerRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
}

type ProvisioningServicePeerManagerRepo interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUserPeers(context.Context, domain.UserIdentifier) ([]domain.Peer, error)
}

type ProvisioningServiceConfigFileManagerRepo interface {
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
}

type ProvisioningService struct {
	cfg *config.Config

	users       ProvisioningServiceUserManagerRepo
	peers       ProvisioningServicePeerManagerRepo
	configFiles ProvisioningServiceConfigFileManagerRepo
}

func NewProvisioningService(
	cfg *config.Config,
	users ProvisioningServiceUserManagerRepo,
	peers ProvisioningServicePeerManagerRepo,
	configFiles ProvisioningServiceConfigFileManagerRepo,
) *ProvisioningService {
	return &ProvisioningService{
		cfg: cfg,

		users:       users,
		peers:       peers,
		configFiles: configFiles,
	}
}

func (p ProvisioningService) GetUserAndPeers(
	ctx context.Context,
	userId domain.UserIdentifier,
	email string,
) (*domain.User, []domain.Peer, error) {
	// first fetch user
	var user *domain.User
	switch {
	case userId != "":
		u, err := p.users.GetUser(ctx, userId)
		if err != nil {
			return nil, nil, err
		}
		user = u
	case email != "":
		u, err := p.users.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, nil, err
		}
		user = u
	default:
		return nil, nil, fmt.Errorf("either UserId or Email must be set: %w", domain.ErrInvalidData)
	}

	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, nil, err
	}

	peers, err := p.peers.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		return nil, nil, err
	}

	return user, peers, nil
}

func (p ProvisioningService) GetPeerConfig(ctx context.Context, peerId domain.PeerIdentifier) ([]byte, error) {
	peer, err := p.peers.GetPeer(ctx, peerId)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	peerCfgReader, err := p.configFiles.GetPeerConfig(ctx, peer.Identifier)
	if err != nil {
		return nil, err
	}

	peerCfgData, err := io.ReadAll(peerCfgReader)
	if err != nil {
		return nil, err
	}

	return peerCfgData, nil
}

func (p ProvisioningService) GetPeerQrPng(ctx context.Context, peerId domain.PeerIdentifier) ([]byte, error) {
	peer, err := p.peers.GetPeer(ctx, peerId)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	peerCfgQrReader, err := p.configFiles.GetPeerConfigQrCode(ctx, peer.Identifier)
	if err != nil {
		return nil, err
	}

	peerCfgQrData, err := io.ReadAll(peerCfgQrReader)
	if err != nil {
		return nil, err
	}

	return peerCfgQrData, nil
}
