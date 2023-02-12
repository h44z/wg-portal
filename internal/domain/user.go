package domain

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	UserSourceLdap     UserSource = "ldap"  // LDAP / ActiveDirectory
	UserSourceDatabase UserSource = "db"    // sqlite / mysql database
	UserSourceOauth    UserSource = "oauth" // oauth / open id connect
)

type UserIdentifier string

type UserSource string

// User is the user model that gets linked to peer entries, by default an empty user model with only the email address is created
type User struct {
	BaseModel

	// required fields
	Identifier   UserIdentifier `gorm:"primaryKey;column:identifier"`
	Email        string         `form:"email" binding:"required,email"`
	Source       UserSource
	ProviderName string
	IsAdmin      bool

	// optional fields
	Firstname  string `form:"firstname" binding:"omitempty"`
	Lastname   string `form:"lastname" binding:"omitempty"`
	Phone      string `form:"phone" binding:"omitempty"`
	Department string `form:"department" binding:"omitempty"`
	Notes      string `form:"notes" binding:"omitempty"`

	// optional, integrated password authentication
	Password       PrivateString `form:"password" binding:"omitempty"`
	Disabled       *time.Time    `gorm:"index;column:disabled"` // if this field is set, the user is disabled
	DisabledReason string        // the reason why the user has been disabled
	Locked         *time.Time    `gorm:"index;column:locked"` // if this field is set, the user is locked and can no longer login
	LockedReason   string        // the reason why the user has been locked

	LinkedPeerCount int `gorm:"-"`
}

func (u *User) IsDisabled() bool {
	return u.Disabled != nil
}

func (u *User) CanChangePassword() error {
	if u.Source == UserSourceDatabase {
		return nil
	}

	return errors.New("password change only allowed for database source")
}

func (u *User) EditAllowed() error {
	if u.Source == UserSourceDatabase {
		return nil
	}

	return errors.New("edit only allowed for database source")
}

func (u *User) CheckPassword(password string) error {
	if u.Source != UserSourceDatabase {
		return errors.New("invalid user source")
	}

	if u.Password == "" {
		return errors.New("empty user password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return errors.New("wrong password")
	}

	return nil
}

func (u *User) HashPassword() error {
	if u.Password == "" {
		return nil // nothing to hash
	}

	if _, err := bcrypt.Cost([]byte(u.Password)); err == nil {
		return nil // password already hashed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = PrivateString(hash)

	return nil
}

func (u *User) CopyCalculatedAttributes(src *User) {
	u.BaseModel = src.BaseModel
	u.LinkedPeerCount = src.LinkedPeerCount
}
