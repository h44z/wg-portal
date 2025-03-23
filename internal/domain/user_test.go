package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestUser_IsDisabled(t *testing.T) {
	user := &User{}
	assert.False(t, user.IsDisabled())

	now := time.Now()
	user.Disabled = &now
	assert.True(t, user.IsDisabled())
}

func TestUser_IsLocked(t *testing.T) {
	user := &User{}
	assert.False(t, user.IsLocked())

	now := time.Now()
	user.Locked = &now
	assert.True(t, user.IsLocked())
}

func TestUser_IsApiEnabled(t *testing.T) {
	user := &User{}
	assert.False(t, user.IsApiEnabled())

	user.ApiToken = "token"
	assert.True(t, user.IsApiEnabled())
}

func TestUser_CanChangePassword(t *testing.T) {
	user := &User{Source: UserSourceDatabase}
	assert.NoError(t, user.CanChangePassword())

	user.Source = UserSourceLdap
	assert.Error(t, user.CanChangePassword())

	user.Source = UserSourceOauth
	assert.Error(t, user.CanChangePassword())
}

func TestUser_EditAllowed(t *testing.T) {
	user := &User{Source: UserSourceDatabase}
	newUser := &User{Source: UserSourceDatabase}
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Notes = "notes can be changed"
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Disabled = &time.Time{}
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Lastname = "lastname or other fields can be changed"
	assert.NoError(t, user.EditAllowed(newUser))

	user.Source = UserSourceLdap
	newUser.Source = UserSourceLdap
	newUser.Disabled = nil
	newUser.Lastname = ""
	newUser.Notes = "notes can be changed"
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Disabled = &time.Time{}
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Lastname = "lastname or other fields can not be changed"
	assert.Error(t, user.EditAllowed(newUser))

	user.Source = UserSourceOauth
	newUser.Source = UserSourceOauth
	newUser.Disabled = nil
	newUser.Lastname = ""
	newUser.Notes = "notes can be changed"
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Disabled = &time.Time{}
	assert.NoError(t, user.EditAllowed(newUser))

	newUser.Lastname = "lastname or other fields can not be changed"
	assert.Error(t, user.EditAllowed(newUser))
}

func TestUser_DeleteAllowed(t *testing.T) {
	user := &User{}
	assert.NoError(t, user.DeleteAllowed())
}

func TestUser_CheckPassword(t *testing.T) {
	password := "password"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &User{Source: UserSourceDatabase, Password: PrivateString(hashedPassword)}
	assert.NoError(t, user.CheckPassword(password))

	user.Password = ""
	assert.Error(t, user.CheckPassword(password))

	user.Source = UserSourceLdap
	assert.Error(t, user.CheckPassword(password))
}

func TestUser_CheckApiToken(t *testing.T) {
	user := &User{}
	assert.Error(t, user.CheckApiToken("token"))

	user.ApiToken = "token"
	assert.NoError(t, user.CheckApiToken("token"))

	assert.Error(t, user.CheckApiToken("wrong_token"))
}

func TestUser_HashPassword(t *testing.T) {
	user := &User{Password: "password"}
	assert.NoError(t, user.HashPassword())
	assert.NotEmpty(t, user.Password)

	user.Password = ""
	assert.NoError(t, user.HashPassword())
}
