package backend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/h44z/wg-portal/internal/config"
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

type UserService struct {
	cfg *config.Config

	users UserDatabaseRepo
	peers PeerDatabaseRepo
}

func NewUserService(cfg *config.Config, users UserDatabaseRepo, peers PeerDatabaseRepo) *UserService {
	return &UserService{
		cfg:   cfg,
		users: users,
		peers: peers,
	}
}

func (s UserService) GetUsers(ctx context.Context) ([]domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	users, err := s.users.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load users: %w", err)
	}

	ch := make(chan *domain.User)
	wg := sync.WaitGroup{}
	workers := int(math.Min(float64(len(users)), 10))
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for user := range ch {
				peers, _ := s.peers.GetUserPeers(ctx, user.Identifier) // ignore error, list will be empty in error case
				user.LinkedPeerCount = len(peers)
			}
		}()
	}
	for i := range users {
		ch <- &users[i]
	}
	close(ch)
	wg.Wait()

	return users, nil
}

func (s UserService) GetUserById(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	user, err := s.users.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to load user %s: %w", id, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, errors.Join(err, domain.ErrNoPermission)
	}

	peers, _ := s.peers.GetUserPeers(ctx, user.Identifier) // ignore error, list will be empty in error case
	user.LinkedPeerCount = len(peers)

	return user, nil
}
