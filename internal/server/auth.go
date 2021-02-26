package server

import (
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
)

// AuthManager keeps track of available authentication providers.
type AuthManager struct {
	Server      *Server
	Group       *gin.RouterGroup // basic group for all providers (/auth)
	providers   []authentication.AuthProvider
	UserManager *users.Manager
}

// RegisterProvider register auth provider
func (auth *AuthManager) RegisterProvider(provider authentication.AuthProvider) {
	name := provider.GetName()
	if auth.GetProvider(name) != nil {
		logrus.Warnf("auth provider %v already registered", name)
	}

	provider.SetupRoutes(auth.Group)
	auth.providers = append(auth.providers, provider)
}

// RegisterProviderWithoutError register auth provider if err is nil
func (auth *AuthManager) RegisterProviderWithoutError(provider authentication.AuthProvider, err error) {
	if err != nil {
		logrus.Errorf("skipping provider registration: %v", err)
		return
	}
	auth.RegisterProvider(provider)
}

// GetProvider get provider by name
func (auth *AuthManager) GetProvider(name string) authentication.AuthProvider {
	for _, provider := range auth.providers {
		if provider.GetName() == name {
			return provider
		}
	}
	return nil
}

// GetProviders return registered providers.
// Returned providers are ordered by provider priority.
func (auth *AuthManager) GetProviders() (providers []authentication.AuthProvider) {
	for _, provider := range auth.providers {
		providers = append(providers, provider)
	}

	// order by priority
	sort.SliceStable(providers, func(i, j int) bool {
		return providers[i].GetPriority() < providers[j].GetPriority()
	})

	return
}

// GetProvidersForType return registered providers for the given type.
// Returned providers are ordered by provider priority.
func (auth *AuthManager) GetProvidersForType(typ authentication.AuthProviderType) (providers []authentication.AuthProvider) {
	for _, provider := range auth.providers {
		if provider.GetType() == typ {
			providers = append(providers, provider)
		}
	}

	// order by priority
	sort.SliceStable(providers, func(i, j int) bool {
		return providers[i].GetPriority() < providers[j].GetPriority()
	})

	return
}

func NewAuthManager(server *Server) *AuthManager {
	m := &AuthManager{
		Server: server,
	}

	m.Group = m.Server.server.Group("/auth")

	return m
}
