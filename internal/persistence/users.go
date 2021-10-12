package persistence

import (
	"time"

	"github.com/pkg/errors"
)

func (d *Database) GetUsersUnscoped() ([]User, error) {
	var users []User
	if err := d.db.Unscoped().Find(&users).Error; err != nil {
		return nil, errors.WithMessagef(err, "unable to find unscoped users")
	}
	return users, nil
}

func (d *Database) SaveUser(user User) error {
	create := user.Identifier == ""
	now := time.Now()

	user.UpdatedAt = now

	if create {
		user.CreatedAt = now
		if err := d.db.Create(&user).Error; err != nil {
			return errors.WithMessage(err, "unable to create new user")
		}
	} else {
		if err := d.db.Save(&user).Error; err != nil {
			return errors.WithMessagef(err, "unable to update user %s", user.Identifier)
		}
	}
	return nil
}

func (d *Database) DeleteUser(id UserIdentifier) error {
	if err := d.db.Delete(&User{}, id).Error; err != nil {
		return errors.WithMessagef(err, "unable to delete user %s", id)
	}
	return nil
}

// Extra functions, currently unused...

func (d *Database) GetUser(id UserIdentifier) (User, error) {
	var user User
	if err := d.db.First(&user, id).Error; err != nil {
		return User{}, errors.WithMessagef(err, "unable to find user %s", id)
	}
	return user, nil
}

func (d *Database) GetUsers() ([]User, error) {
	var users []User
	if err := d.db.Find(&users).Error; err != nil {
		return nil, errors.WithMessagef(err, "unable to find users")
	}
	return users, nil
}

func (d *Database) GetUsersFiltered(filters ...DatabaseFilterCondition) ([]User, error) {
	var users []User
	tx := d.db
	for _, filter := range filters {
		tx = filter(tx)
	}
	if err := tx.Find(&users).Error; err != nil {
		return nil, errors.WithMessagef(err, "unable to find filtered users")
	}
	return users, nil
}
