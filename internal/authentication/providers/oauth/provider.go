package oauth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Provider implements a password login method for a database backend.
type Provider struct {
	db *gorm.DB
}

func New(cfg *common.DatabaseConfig) (*Provider, error) {
	p := &Provider{}

	var err error

	p.db, err = common.GetDatabaseForConfig(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup authentication database %s", cfg.Database)
	}

	return p, nil
}

// GetName return provider name
func (Provider) GetName() string {
	return string(users.UserSourceDatabase)
}

// GetType return provider type
func (Provider) GetType() authentication.AuthProviderType {
	return authentication.AuthProviderTypeOauth
}

// GetPriority return provider priority
func (Provider) GetPriority() int {
	return 2
}

func (provider Provider) SetupRoutes(routes *gin.RouterGroup) {
	// nothing todo here
}

func (provider Provider) Login(ctx *authentication.AuthContext) (string, error) {
	username := strings.ToLower(ctx.Username)

	// Validate input
	if strings.Trim(username, " ") == "" {
		return "", errors.New("empty username")
	}

	// Find user with by email (search for the disabled users too)
	user := users.User{}
	provider.db.Unscoped().Where("email = ?", username).First(&user)

	if user.DeletedAt.Valid {
		return "", errors.New("disabled user")
	}

	// the email can be empty here, if the provider can create the user automatically
	return user.Email, nil
}

func (provider Provider) Logout(context *authentication.AuthContext) error {
	return nil // nothing todo here
}

func (provider Provider) GetUserModel(ctx *authentication.AuthContext) (*authentication.User, error) {
	return &authentication.User{}, nil // nothing todo here
}
