package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type testControllerManager struct{}

func (testControllerManager) GetControllerNames() []config.BackendBase {
	return nil
}

func getSettings(t *testing.T, cfg *config.Config, userId domain.UserIdentifier) model.Settings {
	t.Helper()

	ep := ConfigEndpoint{cfg: cfg, controllerMgr: testControllerManager{}}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/config/settings", nil)
	req = req.WithContext(domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: userId}))
	rec := httptest.NewRecorder()

	ep.handleSettingsGet().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var settings model.Settings
	if err := json.NewDecoder(rec.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}
	return settings
}

// TestConfigEndpointSettingsExposesEditableKeys pins the EditableKeys flag that the frontend uses to offer
// browser-side key generation in frontend/src/components/UserPeerEditModal.vue.
func TestConfigEndpointSettingsExposesEditableKeys(t *testing.T) {
	for _, editable := range []bool{true, false} {
		cfg := &config.Config{}
		cfg.Core.EditableKeys = editable

		if got := getSettings(t, cfg, "user@example.com").EditableKeys; got != editable {
			t.Fatalf("expected EditableKeys=%v for logged-in users, got %v", editable, got)
		}
	}
}

func TestConfigEndpointSettingsHidesEditableKeysFromAnonymousUsers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Core.EditableKeys = true

	if getSettings(t, cfg, domain.CtxUnknownUserId).EditableKeys {
		t.Fatalf("expected EditableKeys to be hidden from anonymous users")
	}
}
