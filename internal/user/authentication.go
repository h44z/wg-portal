package user

import (
	"crypto/subtle"

	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

type PasswordAuthenticator struct {
	store store
}

func NewPasswordAuthenticator(store store) (*PasswordAuthenticator, error) {
	a := &PasswordAuthenticator{
		store: store,
	}

	return a, nil
}

func (p *PasswordAuthenticator) PlaintextAuthentication(userId persistence.UserIdentifier, plainPassword string) error {
	user, err := p.store.GetUser(userId)
	if err != nil {
		return errors.WithMessagef(err, "unable to load user %s", userId)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(plainPassword)); err != nil {
		return errors.WithMessage(err, "invalid password")
	}

	return nil
}

func (p *PasswordAuthenticator) HashedAuthentication(userId persistence.UserIdentifier, hashedPassword string) error {
	user, err := p.store.GetUser(userId)
	if err != nil {
		return errors.WithMessagef(err, "unable to load user %s", userId)
	}

	if subtle.ConstantTimeCompare([]byte(user.Password), []byte(hashedPassword)) != 1 {
		return errors.New("invalid password")
	}

	return nil
}

func (p *PasswordAuthenticator) HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.WithMessage(err, "failed to hash password")
	}

	return string(hash), nil
}
