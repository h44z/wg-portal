package app

import (
	"context"
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
}

type WireGuardManager interface {
	GetImportableInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error)
	ImportNewInterfaces(ctx context.Context, filter ...domain.InterfaceIdentifier) error
	RestoreInterfaceState(ctx context.Context, updateDbOnError bool, filter ...domain.InterfaceIdentifier) error
	CreateDefaultPeer(ctx context.Context, user *domain.User) error
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	PrepareInterface(ctx context.Context) (*domain.Interface, error)
	CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
	UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error)
}

type StatisticsCollector interface {
	StartBackgroundJobs(ctx context.Context)
}
