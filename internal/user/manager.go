package user

import (
	"github.com/h44z/wg-portal/internal/persistence"
)

type Manager interface {
	persistence.UsersLoader
}
