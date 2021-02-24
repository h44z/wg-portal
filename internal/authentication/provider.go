package authentication

import (
	"github.com/gin-gonic/gin"
)

type AuthContext struct {
	Provider AuthProvider
	Username string // email or username
	Password string // optional for OIDC
	Callback string // callback for OIDC
}

type AuthProviderType string

const (
	AuthProviderTypePassword AuthProviderType = "password"
	AuthProviderTypeOauth    AuthProviderType = "oauth"
)

type AuthProvider interface {
	GetName() string
	GetType() AuthProviderType
	GetPriority() int // lower number = higher priority

	Login(*AuthContext) (string, error)
	Logout(*AuthContext) error
	GetUserModel(*AuthContext) (*User, error)

	SetupRoutes(routes *gin.RouterGroup)
}
