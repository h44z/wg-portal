package user

import "github.com/h44z/wg-portal/tmp/persistence"

type Manager interface {
	persistence.UsersLoader
}
