package authentication

import (
	"github.com/gin-gonic/gin"
)

// AuthContext contains all information that the AuthProvider needs to perform the authentication.
type AuthContext struct {
	Username string // email or username
	Password string
	Callback string // callback for OIDC
}

type AuthProviderType string

const (
	AuthProviderTypePassword AuthProviderType = "password"
	AuthProviderTypeOauth    AuthProviderType = "oauth"
)

// AuthProvider is a interface that can be implemented by different authentication providers like LDAP, OAUTH, ...
type AuthProvider interface {
	GetName() string
	GetType() AuthProviderType
	GetPriority() int // lower number = higher priority

	Login(*AuthContext) (string, error)
	Logout(*AuthContext) error
	GetUserModel(*AuthContext) (*User, error)

	SetupRoutes(routes *gin.RouterGroup)
}
