package persistence

import "gorm.io/gorm"

type UserFilterCondition func(tx *gorm.DB)

type UsersLoader interface {
	GetUser(id UserIdentifier) (User, error)
	GetUsers() ([]User, error)
	GetUsersUnscoped() ([]User, error)
	GetUsersFiltered(filter ...UserFilterCondition) ([]User, error)
}

type Users interface {
	UsersLoader

	SaveUser(user User) error
	DeleteUser(identifier UserIdentifier) error
}
