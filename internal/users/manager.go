package users

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Manager struct {
	db *gorm.DB
}

func NewManager(db *gorm.DB) (*Manager, error) {
	m := &Manager{db: db}

	// check if old user table exists (from version <= 1.0.2), if so rename it to peers.
	if m.db.Migrator().HasTable("users") && !m.db.Migrator().HasTable("peers") {
		if err := m.db.Migrator().RenameTable("users", "peers"); err != nil {
			return nil, errors.Wrapf(err, "failed to migrate old database structure")
		} else {
			logrus.Infof("upgraded database format from version v1.0.2")
		}
	}

	if err := m.db.AutoMigrate(&User{}); err != nil {
		return nil, errors.Wrap(err, "failed to migrate user database")
	}

	return m, nil
}

func (m Manager) GetUsers() []User {
	users := make([]User, 0)
	m.db.Find(&users)
	return users
}

func (m Manager) GetUsersUnscoped() []User {
	users := make([]User, 0)
	m.db.Unscoped().Find(&users)
	return users
}

func (m Manager) UserExists(email string) bool {
	return m.GetUser(email) != nil
}

func (m Manager) GetUser(email string) *User {
	user := User{}
	m.db.Where("email = ?", email).First(&user)

	if user.Email != email {
		return nil
	}

	return &user
}

func (m Manager) GetUserUnscoped(email string) *User {
	user := User{}
	m.db.Unscoped().Where("email = ?", email).First(&user)

	if user.Email != email {
		return nil
	}

	return &user
}

func (m Manager) GetFilteredAndSortedUsers(sortKey, sortDirection, search string) []User {
	users := make([]User, 0)
	m.db.Find(&users)

	filteredUsers := filterUsers(users, search)
	sortUsers(filteredUsers, sortKey, sortDirection)

	return filteredUsers
}

func (m Manager) GetFilteredAndSortedUsersUnscoped(sortKey, sortDirection, search string) []User {
	users := make([]User, 0)
	m.db.Unscoped().Find(&users)

	filteredUsers := filterUsers(users, search)
	sortUsers(filteredUsers, sortKey, sortDirection)

	return filteredUsers
}

func (m Manager) GetOrCreateUser(email string) (*User, error) {
	user := User{}
	m.db.Where("email = ?", email).FirstOrInit(&user)

	if user.Email != email {
		user.Email = email
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()
		user.IsAdmin = false
		user.Source = UserSourceDatabase

		res := m.db.Create(&user)
		if res.Error != nil {
			return nil, errors.Wrapf(res.Error, "failed to create user %s", email)
		}
	}

	return &user, nil
}

func (m Manager) GetOrCreateUserUnscoped(email string) (*User, error) {
	user := User{}
	m.db.Unscoped().Where("email = ?", email).FirstOrInit(&user)

	if user.Email != email {
		user.Email = email
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()
		user.IsAdmin = false
		user.Source = UserSourceDatabase

		res := m.db.Create(&user)
		if res.Error != nil {
			return nil, errors.Wrapf(res.Error, "failed to create user %s", email)
		}
	}

	return &user, nil
}

func (m Manager) CreateUser(user *User) error {
	res := m.db.Create(user)
	if res.Error != nil {
		return errors.Wrapf(res.Error, "failed to create user %s", user.Email)
	}

	return nil
}

func (m Manager) UpdateUser(user *User) error {
	res := m.db.Save(user)
	if res.Error != nil {
		return errors.Wrapf(res.Error, "failed to update user %s", user.Email)
	}

	return nil
}

func (m Manager) DeleteUser(user *User) error {
	res := m.db.Delete(user)
	if res.Error != nil {
		return errors.Wrapf(res.Error, "failed to update user %s", user.Email)
	}

	return nil
}

func sortUsers(users []User, key, direction string) {
	sort.Slice(users, func(i, j int) bool {
		var sortValueLeft string
		var sortValueRight string

		switch key {
		case "email":
			sortValueLeft = users[i].Email
			sortValueRight = users[j].Email
		case "firstname":
			sortValueLeft = users[i].Firstname
			sortValueRight = users[j].Firstname
		case "lastname":
			sortValueLeft = users[i].Lastname
			sortValueRight = users[j].Lastname
		case "phone":
			sortValueLeft = users[i].Phone
			sortValueRight = users[j].Phone
		case "source":
			sortValueLeft = string(users[i].Source)
			sortValueRight = string(users[j].Source)
		case "admin":
			sortValueLeft = strconv.FormatBool(users[i].IsAdmin)
			sortValueRight = strconv.FormatBool(users[j].IsAdmin)
		}

		if direction == "asc" {
			return sortValueLeft < sortValueRight
		} else {
			return sortValueLeft > sortValueRight
		}
	})
}

func filterUsers(users []User, search string) []User {
	if search == "" {
		return users
	}

	filteredUsers := make([]User, 0, len(users))
	for i := range users {
		if strings.Contains(users[i].Email, search) ||
			strings.Contains(users[i].Firstname, search) ||
			strings.Contains(users[i].Lastname, search) ||
			strings.Contains(string(users[i].Source), search) ||
			strings.Contains(users[i].Phone, search) {
			filteredUsers = append(filteredUsers, users[i])
		}
	}
	return filteredUsers
}
