package user

import (
	"github.com/h44z/wg-portal/internal/persistence"
)

type store interface {
	GetUser(id persistence.UserIdentifier) (persistence.User, error)
	GetUsers() ([]persistence.User, error)
	GetUsersUnscoped() ([]persistence.User, error)
	GetUsersFiltered(filters ...persistence.DatabaseFilterCondition) ([]persistence.User, error)
	SaveUser(user persistence.User) error
	DeleteUser(identifier persistence.UserIdentifier) error
}
