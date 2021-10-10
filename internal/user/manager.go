package user

import (
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
)

type Loader interface {
	GetUser(id persistence.UserIdentifier) (persistence.User, error)
	GetActiveUsers() ([]persistence.User, error)
	GetAllUsers() ([]persistence.User, error)
	GetFilteredUsers(filter ...persistence.DatabaseFilterCondition) ([]persistence.User, error)
}

type Updater interface {
	CreateUser(user persistence.User) error
	UpdateUser(user persistence.User) error
	DeleteUser(identifier persistence.UserIdentifier) error
}

type Authenticator interface {
	PlaintextAuthentication(userId persistence.UserIdentifier, plainPassword string) error
	HashedAuthentication(userId persistence.UserIdentifier, hashedPassword string) error
}

type PasswordHasher interface {
	HashPassword(plain string) (string, error)
}

type Manager interface {
	Loader
	Updater
	Authenticator
	PasswordHasher
}

type PersistentManager struct {
	store store

	authenticator Authenticator
	hasher        PasswordHasher
}

func NewPersistentManager(store store) (*PersistentManager, error) {
	if store == nil {
		return nil, errors.New("user manager requires a valid store object")
	}

	pwa, err := NewPasswordAuthenticator(store)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize authenticator")
	}

	mgr := &PersistentManager{
		store:         store,
		authenticator: pwa,
		hasher:        pwa,
	}

	return mgr, nil
}

func (p *PersistentManager) GetUser(id persistence.UserIdentifier) (persistence.User, error) {
	return p.store.GetUser(id)
}

func (p *PersistentManager) GetActiveUsers() ([]persistence.User, error) {
	return p.store.GetUsers()
}

func (p *PersistentManager) GetAllUsers() ([]persistence.User, error) {
	return p.store.GetUsersUnscoped()
}

func (p *PersistentManager) GetFilteredUsers(filter ...persistence.DatabaseFilterCondition) ([]persistence.User, error) {
	return p.store.GetUsersFiltered(filter...)
}

func (p *PersistentManager) CreateUser(user persistence.User) error {
	return p.store.SaveUser(user)
}

func (p *PersistentManager) UpdateUser(user persistence.User) error {
	return p.store.SaveUser(user)
}

func (p *PersistentManager) DeleteUser(identifier persistence.UserIdentifier) error {
	return p.store.DeleteUser(identifier)
}
