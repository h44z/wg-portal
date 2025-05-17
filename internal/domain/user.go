package domain

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
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

	// Passwordless authentication
	WebAuthnId             string                   `gorm:"column:webauthn_id"`         // the webauthn id of the user, used for webauthn authentication
	WebAuthnCredentialList []UserWebauthnCredential `gorm:"foreignKey:user_identifier"` // the webauthn credentials of the user, used for webauthn authentication

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

func (u *User) HasWeakPassword(minLength int) error {
	if u.Source != UserSourceDatabase {
		return nil // password is not required for non-database users, so no check needed
	}

	if u.Password == "" {
		return nil // password is not set, so no check needed
	}

	if len(u.Password) < minLength {
		return fmt.Errorf("password is too short, minimum length is %d", minLength)
	}

	return nil // password is strong enough
}

func (u *User) EditAllowed(new *User) error {
	if u.Source == UserSourceDatabase {
		return nil
	}

	// for users which are not database users, only the notes field and the disabled flag can be updated
	updateOk := u.Identifier == new.Identifier
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

// region webauthn

func (u *User) WebAuthnID() []byte {
	decodeString, err := base64.StdEncoding.DecodeString(u.WebAuthnId)
	if err != nil {
		return nil
	}

	return decodeString
}

func (u *User) GenerateWebAuthnId() {
	randomUid1 := uuid.New().String()                                                              // 32 hex digits + 4 dashes
	randomUid2 := uuid.New().String()                                                              // 32 hex digits + 4 dashes
	webAuthnId := []byte(strings.ReplaceAll(fmt.Sprintf("%s%s", randomUid1, randomUid2), "-", "")) // 64 hex digits

	u.WebAuthnId = base64.StdEncoding.EncodeToString(webAuthnId)
}

func (u *User) WebAuthnName() string {
	return string(u.Identifier)
}

func (u *User) WebAuthnDisplayName() string {
	var userName string
	switch {
	case u.Firstname != "" && u.Lastname != "":
		userName = fmt.Sprintf("%s %s", u.Firstname, u.Lastname)
	case u.Firstname != "":
		userName = u.Firstname
	case u.Lastname != "":
		userName = u.Lastname
	default:
		userName = string(u.Identifier)
	}

	return userName
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, len(u.WebAuthnCredentialList))
	for i, cred := range u.WebAuthnCredentialList {
		credential, err := cred.GetCredential()
		if err != nil {
			continue
		}
		credentials[i] = credential
	}
	return credentials
}

func (u *User) AddCredential(userId UserIdentifier, name string, credential webauthn.Credential) error {
	cred, err := NewUserWebauthnCredential(userId, name, credential)
	if err != nil {
		return err
	}

	// Check if the credential already exists
	for _, c := range u.WebAuthnCredentialList {
		if c.GetCredentialId() == string(credential.ID) {
			return errors.New("credential already exists")
		}
	}

	u.WebAuthnCredentialList = append(u.WebAuthnCredentialList, cred)
	return nil
}

func (u *User) UpdateCredential(credentialIdBase64, name string) error {
	for i, c := range u.WebAuthnCredentialList {
		if c.CredentialIdentifier == credentialIdBase64 {
			u.WebAuthnCredentialList[i].DisplayName = name
			return nil
		}
	}

	return errors.New("credential not found")
}

func (u *User) RemoveCredential(credentialIdBase64 string) {
	u.WebAuthnCredentialList = slices.DeleteFunc(u.WebAuthnCredentialList, func(e UserWebauthnCredential) bool {
		return e.CredentialIdentifier == credentialIdBase64
	})
}

type UserWebauthnCredential struct {
	UserIdentifier       string    `gorm:"primaryKey;column:user_identifier"`                   // the user identifier
	CredentialIdentifier string    `gorm:"primaryKey;uniqueIndex;column:credential_identifier"` // base64 encoded credential id
	CreatedAt            time.Time `gorm:"column:created_at"`                                   // the time when the credential was created
	DisplayName          string    `gorm:"column:display_name"`                                 // the display name of the credential
	SerializedCredential string    `gorm:"column:serialized_credential"`                        // JSON and base64 encoded credential
}

func NewUserWebauthnCredential(userIdentifier UserIdentifier, name string, credential webauthn.Credential) (
	UserWebauthnCredential,
	error,
) {
	c := UserWebauthnCredential{
		UserIdentifier:       string(userIdentifier),
		CreatedAt:            time.Now(),
		DisplayName:          name,
		CredentialIdentifier: base64.StdEncoding.EncodeToString(credential.ID),
	}

	err := c.SetCredential(credential)
	if err != nil {
		return c, err
	}

	return c, nil
}

func (c *UserWebauthnCredential) SetCredential(credential webauthn.Credential) error {
	jsonData, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("failed to marshal credential: %w", err)
	}

	c.SerializedCredential = base64.StdEncoding.EncodeToString(jsonData)

	return nil
}

func (c *UserWebauthnCredential) GetCredential() (webauthn.Credential, error) {
	jsonData, err := base64.StdEncoding.DecodeString(c.SerializedCredential)
	if err != nil {
		return webauthn.Credential{}, fmt.Errorf("failed to decode base64 credential: %w", err)
	}

	var credential webauthn.Credential
	if err := json.Unmarshal(jsonData, &credential); err != nil {
		return webauthn.Credential{}, fmt.Errorf("failed to unmarshal credential: %w", err)
	}

	return credential, nil
}

func (c *UserWebauthnCredential) GetCredentialId() string {
	decodeString, _ := base64.StdEncoding.DecodeString(c.CredentialIdentifier)

	return string(decodeString)
}

// endregion webauthn
