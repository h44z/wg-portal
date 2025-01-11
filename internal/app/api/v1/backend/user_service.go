package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserManagerRepo interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
}

type UserService struct {
	cfg *config.Config

	users UserManagerRepo
}

func NewUserService(cfg *config.Config, users UserManagerRepo) *UserService {
	return &UserService{
		cfg:   cfg,
		users: users,
	}
}

func (s UserService) GetAll(ctx context.Context) ([]domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	allUsers, err := s.users.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	return allUsers, nil
}

func (s UserService) GetById(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
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

	return user, nil
}

func (s UserService) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	createdUser, err := s.users.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (s UserService) Update(ctx context.Context, id domain.UserIdentifier, user *domain.User) (
	*domain.User,
	error,
) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	if id != user.Identifier {
		return nil, fmt.Errorf("user id mismatch: %s != %s: %w", id, user.Identifier, domain.ErrInvalidData)
	}

	updatedUser, err := s.users.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

func (s UserService) Delete(ctx context.Context, id domain.UserIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	err := s.users.DeleteUser(ctx, id)
	if err != nil {
		return err
	}

	return nil
}
