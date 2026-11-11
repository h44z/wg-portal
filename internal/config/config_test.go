package config

import (
	"os"
	"testing"
)

func TestGetConfig_NormalizesPeerExpiryAction(t *testing.T) {
	// Point to a non-existent config file so only env defaults are used.
	t.Setenv("WG_PORTAL_CONFIG", "nonexistent.yaml")

	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"valid disable", "disable", "disable"},
		{"valid delete", "delete", "delete"},
		{"invalid value", "badvalue", "disable"},
		{"empty string", "", "disable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WG_PORTAL_CORE_PEER_EXPIRY_ACTION", tt.envValue)

			cfg, err := GetConfig()
			if err != nil {
				t.Fatalf("GetConfig() error: %v", err)
			}
			if cfg.Core.Peer.ExpiryAction != tt.want {
				t.Errorf("ExpiryAction = %q, want %q", cfg.Core.Peer.ExpiryAction, tt.want)
			}
		})
	}
}

func TestGetConfig_PeerExpiryAction_DefaultWithoutEnv(t *testing.T) {
	t.Setenv("WG_PORTAL_CONFIG", "nonexistent.yaml")
	os.Unsetenv("WG_PORTAL_CORE_PEER_EXPIRY_ACTION")

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}
	if cfg.Core.Peer.ExpiryAction != "disable" {
		t.Errorf("ExpiryAction = %q, want %q", cfg.Core.Peer.ExpiryAction, "disable")
	}
}
