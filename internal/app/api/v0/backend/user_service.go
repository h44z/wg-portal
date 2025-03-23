package backend

import (
	"context"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type UserServiceUserManager interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
	ActivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	DeactivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type UserServiceWireGuardManager interface {
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	GetUserInterfaces(ctx context.Context, _ domain.UserIdentifier) ([]domain.Interface, error)
	GetUserPeerStats(ctx context.Context, id domain.UserIdentifier) ([]domain.PeerStatus, error)
}

// endregion dependencies

type UserService struct {
	cfg *config.Config

	users UserServiceUserManager
	wg    UserServiceWireGuardManager
}

func NewUserService(cfg *config.Config, users UserServiceUserManager, wg UserServiceWireGuardManager) *UserService {
	return &UserService{
		cfg:   cfg,
		users: users,
		wg:    wg,
	}
}

func (u UserService) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	return u.users.GetUser(ctx, id)
}

func (u UserService) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	return u.users.GetAllUsers(ctx)
}

func (u UserService) UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	return u.users.UpdateUser(ctx, user)
}

func (u UserService) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	return u.users.CreateUser(ctx, user)
}

func (u UserService) DeleteUser(ctx context.Context, id domain.UserIdentifier) error {
	return u.users.DeleteUser(ctx, id)
}

func (u UserService) ActivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	return u.users.ActivateApi(ctx, id)
}

func (u UserService) DeactivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	return u.users.DeactivateApi(ctx, id)
}

func (u UserService) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	return u.wg.GetUserPeers(ctx, id)
}

func (u UserService) GetUserPeerStats(ctx context.Context, id domain.UserIdentifier) ([]domain.PeerStatus, error) {
	return u.wg.GetUserPeerStats(ctx, id)
}

func (u UserService) GetUserInterfaces(ctx context.Context, id domain.UserIdentifier) ([]domain.Interface, error) {
	return u.wg.GetUserInterfaces(ctx, id)
}
