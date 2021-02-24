package users

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func GetDatabaseForConfig(cfg *Config) (db *gorm.DB, err error) {
	switch cfg.Typ {
	case SupportedDatabaseSQLite:
		if _, err = os.Stat(filepath.Dir(cfg.Database)); os.IsNotExist(err) {
			if err = os.MkdirAll(filepath.Dir(cfg.Database), 0700); err != nil {
				return
			}
		}
		db, err = gorm.Open(sqlite.Open(cfg.Database), &gorm.Config{})
		if err != nil {
			return
		}
	case SupportedDatabaseMySQL:
		connectionString := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		db, err = gorm.Open(mysql.Open(connectionString), &gorm.Config{})
		if err != nil {
			return
		}

		sqlDB, _ := db.DB()
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(10)
		err = sqlDB.Ping() // This DOES open a connection if necessary. This makes sure the database is accessible
		if err != nil {
			return nil, errors.Wrap(err, "failed to ping mysql authentication database")
		}
	}

	// Enable Logger (logrus)
	logCfg := logger.Config{
		SlowThreshold: time.Second, // all slower than one second
		Colorful:      false,
		LogLevel:      logger.Silent, // default: log nothing
	}

	if logrus.StandardLogger().GetLevel() == logrus.TraceLevel {
		logCfg.LogLevel = logger.Info
		logCfg.SlowThreshold = 500 * time.Millisecond // all slower than half a second
	}

	db.Config.Logger = logger.New(logrus.StandardLogger(), logCfg)
	return
}

type Manager struct {
	db *gorm.DB
}

func NewManager(cfg *Config) (*Manager, error) {
	m := &Manager{}

	var err error
	m.db, err = GetDatabaseForConfig(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup user database %s", cfg.Database)
	}

	return m, m.MigrateUserDB()
}

func (m Manager) MigrateUserDB() error {
	if err := m.db.AutoMigrate(&User{}); err != nil {
		return errors.Wrap(err, "failed to migrate user database")
	}
	return nil
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
