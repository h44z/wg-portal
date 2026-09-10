package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackendValidate_DefaultsToLocal(t *testing.T) {
	backend := Backend{}
	require.NoError(t, backend.Validate())
	assert.Equal(t, LocalBackendName, backend.Default)
}

func TestBackendValidate_RejectsReservedId(t *testing.T) {
	tests := map[string]Backend{
		"mikrotik": {Mikrotik: []BackendMikrotik{{BackendBase: BackendBase{Id: LocalBackendName}}}},
		"pfsense":  {Pfsense: []BackendPfsense{{BackendBase: BackendBase{Id: LocalBackendName}}}},
		"opnsense": {Opnsense: []BackendOpnsense{{BackendBase: BackendBase{Id: LocalBackendName}}}},
	}

	for name, backend := range tests {
		t.Run(name, func(t *testing.T) {
			err := backend.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "reserved keyword")
		})
	}
}

// IDs must be unique across backend *types*, not just within one list --
// otherwise two different firewalls could claim the same backend id and the
// controller map would silently keep only one of them.
func TestBackendValidate_RejectsDuplicateIdAcrossTypes(t *testing.T) {
	backend := Backend{
		Pfsense:  []BackendPfsense{{BackendBase: BackendBase{Id: "fw1"}}},
		Opnsense: []BackendOpnsense{{BackendBase: BackendBase{Id: "fw1"}}},
	}

	err := backend.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not unique")
}

func TestBackendValidate_RejectsUnknownDefault(t *testing.T) {
	backend := Backend{
		Default:  "does-not-exist",
		Opnsense: []BackendOpnsense{{BackendBase: BackendBase{Id: "fw1"}}},
	}

	err := backend.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not defined")
}

func TestBackendValidate_AcceptsOpnsenseAsDefault(t *testing.T) {
	backend := Backend{
		Default:  "fw1",
		Opnsense: []BackendOpnsense{{BackendBase: BackendBase{Id: "fw1"}}},
	}

	assert.NoError(t, backend.Validate())
}

func TestBackendOpnsenseDefaults(t *testing.T) {
	var nilBackend *BackendOpnsense
	assert.Equal(t, 30*time.Second, nilBackend.GetApiTimeout(),
		"a nil receiver must still yield the default timeout")

	backend := &BackendOpnsense{}
	assert.Equal(t, 30*time.Second, backend.GetApiTimeout())

	backend.ApiTimeout = 5 * time.Second
	assert.Equal(t, 5*time.Second, backend.GetApiTimeout())
}

func TestBackendOpnsenseDisplayName(t *testing.T) {
	backend := BackendOpnsense{BackendBase: BackendBase{Id: "fw1"}}
	assert.Equal(t, "fw1", backend.GetDisplayName(), "display name falls back to the id")

	backend.DisplayName = "Edge firewall"
	assert.Equal(t, "Edge firewall", backend.GetDisplayName())
}
