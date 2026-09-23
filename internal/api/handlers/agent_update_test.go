package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metron/internal/agentupdate"

	"github.com/gin-gonic/gin"
)

func newUpdateTestRouter(t *testing.T, dir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewAgentUpdateHandler(dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	r.GET("/v1/agent/update", h.GetManifest)
	r.GET("/v1/agent/update/download", h.Download)
	return r
}

func serve(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestAgentUpdate_NothingPublished(t *testing.T) {
	r := newUpdateTestRouter(t, t.TempDir())

	if w := serve(r, "/v1/agent/update"); w.Code != http.StatusNotFound {
		t.Errorf("manifest: expected 404, got %d", w.Code)
	}
	if w := serve(r, "/v1/agent/update/download"); w.Code != http.StatusNotFound {
		t.Errorf("download: expected 404, got %d", w.Code)
	}
}

func TestAgentUpdate_ServesManifestAndBinary(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("fake agent binary")
	sum, size, err := agentupdate.HashReader(strings.NewReader(string(binary)))
	if err != nil {
		t.Fatal(err)
	}
	manifest := agentupdate.Manifest{Version: "v2", SHA256: sum, Size: size}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, agentupdate.ManifestFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, agentupdate.BinaryFile), binary, 0o644); err != nil {
		t.Fatal(err)
	}

	r := newUpdateTestRouter(t, dir)

	w := serve(r, "/v1/agent/update?version=v1")
	if w.Code != http.StatusOK {
		t.Fatalf("manifest: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got agentupdate.Manifest
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid manifest JSON: %v", err)
	}
	if got.Version != "v2" || got.SHA256 != sum {
		t.Errorf("unexpected manifest: %+v", got)
	}

	w = serve(r, "/v1/agent/update/download")
	if w.Code != http.StatusOK {
		t.Fatalf("download: expected 200, got %d", w.Code)
	}
	if w.Body.String() != string(binary) {
		t.Errorf("download body mismatch")
	}
}

func TestAgentUpdate_InvalidManifestIs500(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, agentupdate.ManifestFile), []byte(`{"version":""}`), 0o644); err != nil {
		t.Fatal(err)
	}

	w := serve(newUpdateTestRouter(t, dir), "/v1/agent/update")
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid manifest, got %d", w.Code)
	}
}
