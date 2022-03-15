package users

import (
	"time"

	"gorm.io/gorm"
)

type UserSource string

const (
	UserSourceLdap     UserSource = "ldap" // LDAP / ActiveDirectory
	UserSourceDatabase UserSource = "db"   // sqlite / mysql database
	UserSourceOIDC     UserSource = "oidc" // open id connect, TODO: implement
)

type PrivateString string

func (PrivateString) MarshalJSON() ([]byte, error) {
	return []byte(`""`), nil
}

func (PrivateString) String() string {
	return ""
}

// User is the user model that gets linked to peer entries, by default an empty usermodel with only the email address is created
type User struct {
	// required fields
	Email   string `gorm:"primaryKey" form:"email" binding:"required,email"`
	Source  UserSource
	IsAdmin bool `form:"isadmin"`

	// optional fields
	Firstname string `form:"firstname" binding:"required"`
	Lastname  string `form:"lastname" binding:"required"`
	Phone     string `form:"phone" binding:"omitempty"`

	// optional, integrated password authentication
	Password PrivateString `form:"password" binding:"omitempty"`

	// database internal fields
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index" json:",omitempty" swaggertype:"string"`
}
