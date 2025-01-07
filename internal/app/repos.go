package app

import (
	"context"
	"io"

	"github.com/h44z/wg-portal/internal/domain"
)

type Authenticator interface {
	GetExternalLoginProviders(_ context.Context) []domain.LoginProviderInfo
	IsUserValid(ctx context.Context, id domain.UserIdentifier) bool
	PlainLogin(ctx context.Context, username, password string) (*domain.User, error)
	OauthLoginStep1(_ context.Context, providerId string) (authCodeUrl, state, nonce string, err error)
	OauthLoginStep2(ctx context.Context, providerId, nonce, code string) (*domain.User, error)
}

type UserManager interface {
	RegisterUser(ctx context.Context, user *domain.User) error
	NewUser(ctx context.Context, user *domain.User) error
	StartBackgroundJobs(ctx context.Context)
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
	ActivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	DeactivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type WireGuardManager interface {
	StartBackgroundJobs(ctx context.Context)
	GetImportableInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error)
	ImportNewInterfaces(ctx context.Context, filter ...domain.InterfaceIdentifier) (int, error)
	RestoreInterfaceState(ctx context.Context, updateDbOnError bool, filter ...domain.InterfaceIdentifier) error
	CreateDefaultPeer(ctx context.Context, userId domain.UserIdentifier) error
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeerStats(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PeerStatus, error)
	GetUserPeerStats(ctx context.Context, id domain.UserIdentifier) ([]domain.PeerStatus, error)
	GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	PrepareInterface(ctx context.Context) (*domain.Interface, error)
	CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
	UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, []domain.Peer, error)
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error)
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	CreatePeer(ctx context.Context, p *domain.Peer) (*domain.Peer, error)
	CreateMultiplePeers(
		ctx context.Context,
		id domain.InterfaceIdentifier,
		r *domain.PeerCreationRequest,
	) ([]domain.Peer, error)
	UpdatePeer(ctx context.Context, p *domain.Peer) (*domain.Peer, error)
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	ApplyPeerDefaults(ctx context.Context, in *domain.Interface) error
}

type StatisticsCollector interface {
	StartBackgroundJobs(ctx context.Context)
}

type ConfigFileManager interface {
	GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error)
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	PersistInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) error
}

type MailManager interface {
	SendPeerEmail(ctx context.Context, linkOnly bool, peers ...domain.PeerIdentifier) error
}

type ApiV1Manager interface {
	ApiV1GetUsers(ctx context.Context) ([]domain.User, error)
}
