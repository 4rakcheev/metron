package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"metron/internal/core"
	"metron/internal/storage/sqlite"

	"github.com/gin-gonic/gin"
)

func newBypassTestRouter(t *testing.T) (*gin.Engine, *sqlite.SQLiteStorage) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewAgentHandler(store, nil, logger)

	r := gin.New()
	r.GET("/v1/devices/:id/bypass", h.GetDeviceBypass)
	return r, store
}

func getBypass(r *gin.Engine, deviceID string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/devices/"+deviceID+"/bypass", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestGetDeviceBypass_NotSet(t *testing.T) {
	r, _ := newBypassTestRouter(t)

	w := getBypass(r, "win-pc1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDeviceBypass_Active(t *testing.T) {
	r, store := newBypassTestRouter(t)
	ctx := context.Background()

	expires := time.Now().Add(time.Hour)
	if err := store.SetDeviceBypass(ctx, &core.DeviceBypass{
		DeviceID:  "win-pc1",
		Enabled:   true,
		Reason:    "Telegram Bot",
		EnabledAt: time.Now(),
		EnabledBy: "api",
		ExpiresAt: &expires,
	}); err != nil {
		t.Fatalf("failed to set bypass: %v", err)
	}

	w := getBypass(r, "win-pc1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", body["enabled"])
	}
	if body["device_id"] != "win-pc1" {
		t.Errorf("expected device_id=win-pc1, got %v", body["device_id"])
	}
	if _, ok := body["expires_at"]; !ok {
		t.Error("expected expires_at in response")
	}
}

func TestGetDeviceBypass_Indefinite(t *testing.T) {
	r, store := newBypassTestRouter(t)

	if err := store.SetDeviceBypass(context.Background(), &core.DeviceBypass{
		DeviceID:  "win-pc1",
		Enabled:   true,
		EnabledAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to set bypass: %v", err)
	}

	w := getBypass(r, "win-pc1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["expires_at"]; ok {
		t.Error("expected no expires_at for indefinite bypass")
	}
}

func TestGetDeviceBypass_ExpiredIsClearedAndNotFound(t *testing.T) {
	r, store := newBypassTestRouter(t)
	ctx := context.Background()

	expired := time.Now().Add(-time.Minute)
	if err := store.SetDeviceBypass(ctx, &core.DeviceBypass{
		DeviceID:  "win-pc1",
		Enabled:   true,
		EnabledAt: time.Now().Add(-time.Hour),
		ExpiresAt: &expired,
	}); err != nil {
		t.Fatalf("failed to set bypass: %v", err)
	}

	w := getBypass(r, "win-pc1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	stored, err := store.GetDeviceBypass(ctx, "win-pc1")
	if err != nil {
		t.Fatalf("failed to read bypass: %v", err)
	}
	if stored != nil {
		t.Error("expected expired bypass to be cleared from storage")
	}
}
