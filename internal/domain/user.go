package domain

import (
	"crypto/subtle"
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
	Disabled       *time.Time    `gorm:"index;column:disabled"` // if this field is set, the user is disabled (WireGuard peers are disabled as well)
	DisabledReason string        // the reason why the user has been disabled
	Locked         *time.Time    `gorm:"index;column:locked"` // if this field is set, the user is locked and can no longer login (WireGuard peers still can connect)
	LockedReason   string        // the reason why the user has been locked

	// API token for REST API access
	ApiToken        string `form:"api_token" binding:"omitempty"`
	ApiTokenCreated *time.Time

	LinkedPeerCount int `gorm:"-"`
}

// IsDisabled returns true if the user is disabled. In such a case,
// no login is possible and WireGuard peers associated with the user are disabled.
func (u *User) IsDisabled() bool {
	return u.Disabled != nil
}

// IsLocked returns true if the user is locked. In such a case, no login is possible, WireGuard connections still work.
func (u *User) IsLocked() bool {
	return u.Locked != nil
}

func (u *User) IsApiEnabled() bool {
	if u.ApiToken != "" {
		return true
	}

	return false
}

func (u *User) CanChangePassword() error {
	if u.Source == UserSourceDatabase {
		return nil
	}

	return errors.New("password change only allowed for database source")
}

func (u *User) EditAllowed(new *User) error {
	if u.Source == UserSourceDatabase {
		return nil
	}

	// for users which are not database users, only the notes field and the disabled flag can be updated
	updateOk := true
	updateOk = updateOk && u.Identifier == new.Identifier
	updateOk = updateOk && u.Source == new.Source
	updateOk = updateOk && u.IsAdmin == new.IsAdmin
	updateOk = updateOk && u.Email == new.Email
	updateOk = updateOk && u.Firstname == new.Firstname
	updateOk = updateOk && u.Lastname == new.Lastname
	updateOk = updateOk && u.Phone == new.Phone
	updateOk = updateOk && u.Department == new.Department

	if !updateOk {
		return errors.New("edit only allowed for database source")
	}

	return nil
}

func (u *User) DeleteAllowed() error {
	return nil // all users can be deleted, OAuth and LDAP users might still be recreated
}

func (u *User) CheckPassword(password string) error {
	if u.Source != UserSourceDatabase {
		return errors.New("invalid user source")
	}

	if u.IsDisabled() {
		return errors.New("user disabled")
	}

	if u.Password == "" {
		return errors.New("empty user password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return errors.New("wrong password")
	}

	return nil
}

func (u *User) CheckApiToken(token string) error {
	if !u.IsApiEnabled() {
		return errors.New("api access disabled")
	}

	if res := subtle.ConstantTimeCompare([]byte(u.ApiToken), []byte(token)); res != 1 {
		return errors.New("wrong token")
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
