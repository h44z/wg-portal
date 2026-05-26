package backend

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/fedor-git/wg-portal-2/internal/app/api/v1/models"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type ProvisioningServiceUserManagerRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
}

type ProvisioningServicePeerManagerRepo interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUserPeers(context.Context, domain.UserIdentifier) ([]domain.Peer, error)
	PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	CreatePeer(ctx context.Context, p *domain.Peer) (*domain.Peer, error)
}

type ProvisioningServiceConfigFileManagerRepo interface {
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier, style string) (io.Reader, error)
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier, style string) (io.Reader, error)
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

	peerCfgReader, err := p.configFiles.GetPeerConfig(ctx, peer.Identifier, domain.ConfigStyleWgQuick)
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

	peerCfgQrReader, err := p.configFiles.GetPeerConfigQrCode(ctx, peer.Identifier, domain.ConfigStyleWgQuick)
	if err != nil {
		return nil, err
	}

	peerCfgQrData, err := io.ReadAll(peerCfgQrReader)
	if err != nil {
		return nil, err
	}

	return peerCfgQrData, nil
}

func (p ProvisioningService) NewPeer(ctx context.Context, req models.ProvisioningRequest) (*domain.Peer, error) {
	if req.UserIdentifier == "" {
		req.UserIdentifier = string(domain.GetUserInfo(ctx).Id) // use authenticated user id if not set
	}

	// check permissions
	if err := domain.ValidateUserAccessRights(ctx, domain.UserIdentifier(req.UserIdentifier)); err != nil {
		return nil, err
	}
	if !p.cfg.Core.SelfProvisioningAllowed {
		// only admins can create new peers if self-provisioning is disabled
		if err := domain.ValidateAdminAccessRights(ctx); err != nil {
			return nil, err
		}
	}

	// prepare new peer
	peer, err := p.peers.PreparePeer(ctx, domain.InterfaceIdentifier(req.InterfaceIdentifier))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare new peer: %w", err)
	}
	peer.UserIdentifier = domain.UserIdentifier(req.UserIdentifier) // overwrite context user id with the one from the request
	if req.PublicKey != "" {
		peer.Identifier = domain.PeerIdentifier(req.PublicKey)
		peer.Interface.PublicKey = req.PublicKey
		peer.Interface.PrivateKey = "" // clear private key if public key is set, WireGuard Portal does not know the private key in that case
	}
	if req.PresharedKey != "" {
		peer.PresharedKey = domain.PreSharedKey(req.PresharedKey)
	}

	if req.DisplayName != "" {
		peer.DisplayName = req.DisplayName
	} else {
		peer.GenerateDisplayName("API")
	}

	// Only set expiration if DoNotExpire is not set
	if !req.DoNotExpire && req.ExpiresAt != "" {
		expiryDate, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid ExpiresAt format, expected RFC3339: %w", err)
		}
		peer.ExpiresAt = &expiryDate
		// Lock TTL only if user explicitly provided the date (not default)
		// Default TTL should allow activity tracking to update the expiration
		if !req.ExpiresAtIsDefault {
			peer.TTLLocked = true
		}
	} else if req.DoNotExpire {
		// If DoNotExpire is true, lock TTL to prevent activity tracking from updating it
		// This ensures ExpiresAt remains nil and peer never expires
		peer.TTLLocked = true
	}

	// save new peer
	peer, err = p.peers.CreatePeer(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("failed to create new peer: %w", err)
	}

	return peer, nil
}

func (p ProvisioningService) FindPeerByDisplayNameAndUserIdentifier(ctx context.Context, displayName, userIdentifier string) (*domain.Peer, error) {
	peers, err := p.peers.GetUserPeers(ctx, domain.UserIdentifier(userIdentifier))
	if err != nil {
		return nil, err
	}

	for _, peer := range peers {
		if peer.DisplayName == displayName {
			return &peer, nil
		}
	}

	return nil, domain.ErrNotFound
}

func (p ProvisioningService) GetConfig() *config.Config {
	return p.cfg
}
