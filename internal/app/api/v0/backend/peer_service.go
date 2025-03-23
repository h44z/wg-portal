package backend

import (
	"context"
	"io"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type PeerServicePeerManager interface {
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error)
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	CreateMultiplePeers(
		ctx context.Context,
		interfaceId domain.InterfaceIdentifier,
		r *domain.PeerCreationRequest,
	) ([]domain.Peer, error)
	GetPeerStats(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PeerStatus, error)
}

type PeerServiceConfigFileManager interface {
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
}

type PeerServiceMailManager interface {
	SendPeerEmail(ctx context.Context, linkOnly bool, peers ...domain.PeerIdentifier) error
}

// endregion dependencies

type PeerService struct {
	cfg *config.Config

	peers      PeerServicePeerManager
	configFile PeerServiceConfigFileManager
	mailer     PeerServiceMailManager
}

func NewPeerService(
	cfg *config.Config,
	peers PeerServicePeerManager,
	configFile PeerServiceConfigFileManager,
	mailer PeerServiceMailManager,
) *PeerService {
	return &PeerService{
		cfg:        cfg,
		peers:      peers,
		configFile: configFile,
		mailer:     mailer,
	}
}

func (p PeerService) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	return p.peers.GetInterfaceAndPeers(ctx, id)
}

func (p PeerService) PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	return p.peers.PreparePeer(ctx, id)
}

func (p PeerService) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	return p.peers.GetPeer(ctx, id)
}

func (p PeerService) CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	return p.peers.CreatePeer(ctx, peer)
}

func (p PeerService) CreateMultiplePeers(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	r *domain.PeerCreationRequest,
) ([]domain.Peer, error) {
	return p.peers.CreateMultiplePeers(ctx, interfaceId, r)
}

func (p PeerService) UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	return p.peers.UpdatePeer(ctx, peer)
}

func (p PeerService) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	return p.peers.DeletePeer(ctx, id)
}

func (p PeerService) GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error) {
	return p.configFile.GetPeerConfig(ctx, id)
}

func (p PeerService) GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error) {
	return p.configFile.GetPeerConfigQrCode(ctx, id)
}

func (p PeerService) SendPeerEmail(ctx context.Context, linkOnly bool, peers ...domain.PeerIdentifier) error {
	return p.mailer.SendPeerEmail(ctx, linkOnly, peers...)
}

func (p PeerService) GetPeerStats(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PeerStatus, error) {
	return p.peers.GetPeerStats(ctx, id)
}
