package config

import (
	"fmt"
)

const LocalBackendName = "local"

type Backend struct {
	Default string `yaml:"default"` // The default backend to use (defaults to the internal backend)

	Mikrotik []BackendMikrotik `yaml:"mikrotik"`
}

// Validate checks the backend configuration for errors.
func (b *Backend) Validate() error {
	if b.Default == "" {
		b.Default = LocalBackendName
	}

	uniqueMap := make(map[string]struct{})
	for _, backend := range b.Mikrotik {
		if backend.Id == LocalBackendName {
			return fmt.Errorf("backend ID %q is a reserved keyword", LocalBackendName)
		}
		if _, exists := uniqueMap[backend.Id]; exists {
			return fmt.Errorf("backend ID %q is not unique", backend.Id)
		}
		uniqueMap[backend.Id] = struct{}{}
	}

	if b.Default != LocalBackendName {
		if _, ok := uniqueMap[b.Default]; !ok {
			return fmt.Errorf("default backend %q is not defined in the configuration", b.Default)
		}
	}

	return nil
}

type BackendMikrotik struct {
	Id          string `yaml:"id"`           // A unique id for the Mikrotik backend
	DisplayName string `yaml:"display_name"` // A display name for the Mikrotik backend

	ApiUrl      string `yaml:"api_url"` // The base URL of the Mikrotik API (e.g., "https://10.10.10.10:8729/rest")
	ApiUser     string `yaml:"api_user"`
	ApiPassword string `yaml:"api_password"`
}
