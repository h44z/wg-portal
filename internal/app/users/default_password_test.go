package users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type mockUserDatabaseRepo struct {
	users map[domain.UserIdentifier]*domain.User
}

func (m *mockUserDatabaseRepo) GetUser(_ context.Context, id domain.UserIdentifier) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (m *mockUserDatabaseRepo) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserDatabaseRepo) GetUserByWebAuthnCredential(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

func (m *mockUserDatabaseRepo) GetAllUsers(_ context.Context) ([]domain.User, error) {
	res := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		res = append(res, *u)
	}
	return res, nil
}

func (m *mockUserDatabaseRepo) FindUsers(_ context.Context, _ string) ([]domain.User, error) {
	return nil, nil
}

func (m *mockUserDatabaseRepo) SaveUser(_ context.Context, id domain.UserIdentifier, updateFunc func(u *domain.User) (*domain.User, error)) error {
	u, ok := m.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	updated, err := updateFunc(u)
	if err != nil {
		return err
	}
	m.users[id] = updated
	return nil
}

func (m *mockUserDatabaseRepo) DeleteUser(_ context.Context, id domain.UserIdentifier) error {
	delete(m.users, id)
	return nil
}

func TestManager_IsDefaultPasswordInUse(t *testing.T) {
	cfg := &config.Config{}
	cfg.Core.AdminUser = config.DefaultAdminUser
	cfg.Core.AdminPassword = config.DefaultAdminPassword

	defaultAdminUser := &domain.User{
		Identifier: domain.UserIdentifier(config.DefaultAdminUser),
		IsAdmin:    true,
		Password:   domain.PrivateString(config.DefaultAdminPassword),
		Authentications: []domain.UserAuthentication{
			{Source: domain.UserSourceDatabase},
		},
	}
	require.NoError(t, defaultAdminUser.HashPassword())

	repo := &mockUserDatabaseRepo{
		users: map[domain.UserIdentifier]*domain.User{
			defaultAdminUser.Identifier: defaultAdminUser,
		},
	}

	mgr, err := NewUserManager(cfg, nil, repo, nil, nil)
	require.NoError(t, err)

	assert.True(t, mgr.IsDefaultPasswordInUse(context.Background()))

	// Change admin password to something else
	defaultAdminUser.Password = "new-secure-password"
	require.NoError(t, defaultAdminUser.HashPassword())

	assert.False(t, mgr.IsDefaultPasswordInUse(context.Background()))
}
