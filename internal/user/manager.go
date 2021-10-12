package user

import (
	"sort"
	"sync"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
)

type Loader interface {
	GetUser(id persistence.UserIdentifier) (*persistence.User, error)
	GetActiveUsers() ([]*persistence.User, error)
	GetAllUsers() ([]*persistence.User, error)
	GetFilteredUsers(filter Filter) ([]*persistence.User, error)
}

type Updater interface {
	CreateUser(user *persistence.User) error
	UpdateUser(user *persistence.User) error
	DeleteUser(identifier persistence.UserIdentifier) error
}

type Authenticator interface {
	PlaintextAuthentication(userId persistence.UserIdentifier, plainPassword string) error
	HashedAuthentication(userId persistence.UserIdentifier, hashedPassword string) error
}

type PasswordHasher interface {
	HashPassword(plain string) (string, error)
}

// Filter can be used to filter users. If this function returns true, the given user is included in the result.
type Filter func(user *persistence.User) bool

type Manager interface {
	Loader
	Updater
	Authenticator
	PasswordHasher
}

type PersistentManager struct {
	mux sync.RWMutex // mutex to synchronize access to maps and external api clients

	store store

	// internal holder of user objects
	users map[persistence.UserIdentifier]*persistence.User
}

func NewPersistentManager(store store) (*PersistentManager, error) {
	if store == nil {
		return nil, errors.New("user manager requires a valid store object")
	}

	mgr := &PersistentManager{
		store: store,

		users: make(map[persistence.UserIdentifier]*persistence.User),
	}

	return mgr, nil
}

func (p *PersistentManager) GetUser(id persistence.UserIdentifier) (*persistence.User, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	if !p.userExists(id) {
		return nil, errors.New("no such user exists")
	}

	if !p.userIsEnabled(id) {
		return nil, errors.New("user is disabled")
	}

	return p.users[id], nil
}

func (p *PersistentManager) GetActiveUsers() ([]*persistence.User, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	users := make([]*persistence.User, 0, len(p.users))
	for _, user := range p.users {
		if !user.DeletedAt.Valid {
			users = append(users, user)
		}
	}

	// Order the users by uid
	sort.Slice(users, func(i, j int) bool {
		return users[i].Uid < users[j].Uid
	})

	return users, nil
}

func (p *PersistentManager) GetAllUsers() ([]*persistence.User, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	users := make([]*persistence.User, 0, len(p.users))
	for _, user := range p.users {
		users = append(users, user)
	}

	// Order the users by uid
	sort.Slice(users, func(i, j int) bool {
		return users[i].Uid < users[j].Uid
	})

	return users, nil
}

func (p *PersistentManager) GetFilteredUsers(filter Filter) ([]*persistence.User, error) {
	p.mux.RLock()
	defer p.mux.RUnlock()

	users := make([]*persistence.User, 0, len(p.users))
	for _, user := range p.users {
		if filter == nil || filter(user) {
			users = append(users, user)
		}
	}

	// Order the users by uid
	sort.Slice(users, func(i, j int) bool {
		return users[i].Uid < users[j].Uid
	})

	return users, nil
}

func (p *PersistentManager) CreateUser(user *persistence.User) error {
	if err := p.checkUser(user); err != nil {
		return errors.WithMessage(err, "user validation failed")
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	if p.userExists(user.Uid) {
		return errors.New("user already exists")
	}

	p.users[user.Uid] = user

	err := p.persistUser(user.Uid, false)
	if err != nil {
		return errors.WithMessage(err, "failed to persist created user")
	}

	return nil
}

func (p *PersistentManager) UpdateUser(user *persistence.User) error {
	if err := p.checkUser(user); err != nil {
		return errors.WithMessage(err, "user validation failed")
	}

	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.userExists(user.Uid) {
		return errors.New("user does not exists")
	}

	p.users[user.Uid] = user

	err := p.persistUser(user.Uid, false)
	if err != nil {
		return errors.WithMessage(err, "failed to persist updated user")
	}

	return nil
}

func (p *PersistentManager) DeleteUser(id persistence.UserIdentifier) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	if !p.userExists(id) {
		return errors.New("user does not exists")
	}

	err := p.persistUser(id, true)
	if err != nil {
		return errors.WithMessage(err, "failed to persist deleted user")
	}

	delete(p.users, id)

	return nil
}

//
// -- Helpers
//

func (p *PersistentManager) initializeFromStore() error {
	if p.store == nil {
		return nil // no store, nothing to do
	}

	users, err := p.store.GetUsersUnscoped()
	if err != nil {
		return errors.WithMessage(err, "failed to get all users")
	}

	for _, tmpUser := range users {
		user := tmpUser
		p.users[user.Uid] = &user
	}

	return nil
}

func (p *PersistentManager) userExists(id persistence.UserIdentifier) bool {
	if _, ok := p.users[id]; ok {
		return true
	}
	return false
}

func (p *PersistentManager) userIsEnabled(id persistence.UserIdentifier) bool {
	if user, ok := p.users[id]; ok && !user.DeletedAt.Valid {
		return true
	}
	return false
}

func (p *PersistentManager) persistUser(id persistence.UserIdentifier, delete bool) error {
	if p.store == nil {
		return nil // nothing to do
	}

	var err error
	if delete {
		err = p.store.DeleteUser(id)
	} else {
		err = p.store.SaveUser(*p.users[id])
	}
	if err != nil {
		return errors.Wrapf(err, "failed to persist user")
	}

	return nil
}

func (p *PersistentManager) checkUser(user *persistence.User) error {
	if user == nil {
		return errors.New("user must not be nil")
	}
	if user.Uid == "" {
		return errors.New("missing user identifier")
	}
	if user.Source == "" {
		return errors.New("missing user source")
	}

	return nil
}
