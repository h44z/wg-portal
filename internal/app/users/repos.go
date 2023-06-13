package users

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserDatabaseRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	FindUsers(ctx context.Context, search string) ([]domain.User, error)
	SaveUser(ctx context.Context, id domain.UserIdentifier, updateFunc func(u *domain.User) (*domain.User, error)) error
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
}

type PeerDatabaseRepo interface {
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
}
