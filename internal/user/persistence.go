package user

import (
	"github.com/h44z/wg-portal/internal/persistence"
)

type store interface {
	GetUsersUnscoped() ([]persistence.User, error)
	SaveUser(user persistence.User) error
	DeleteUser(identifier persistence.UserIdentifier) error
}
