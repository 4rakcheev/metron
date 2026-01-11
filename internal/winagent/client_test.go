package winagent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHTTPMetronClient_GetSessionStatus_Success(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	sessionID := "session-123"
	endsAt := now.Add(30 * time.Minute)
	warnAt := now.Add(25 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/v1/agent/session" {
			t.Errorf("Expected path /v1/agent/session, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("device_id") != "test-device" {
			t.Errorf("Expected device_id=test-device, got %s", r.URL.Query().Get("device_id"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got %s", r.Header.Get("Authorization"))
		}

		// Return response
		response := map[string]interface{}{
			"active":      true,
			"session_id":  sessionID,
			"ends_at":     endsAt.Format(time.RFC3339),
			"warn_at":     warnAt.Format(time.RFC3339),
			"server_time": now.Format(time.RFC3339),
			"bypass_mode": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !status.Active {
		t.Error("Expected Active to be true")
	}
	if status.SessionID == nil || *status.SessionID != sessionID {
		t.Errorf("Expected SessionID '%s', got %v", sessionID, status.SessionID)
	}
	if !status.EndsAt.Equal(endsAt) {
		t.Errorf("Expected EndsAt %v, got %v", endsAt, status.EndsAt)
	}
	if !status.WarnAt.Equal(warnAt) {
		t.Errorf("Expected WarnAt %v, got %v", warnAt, status.WarnAt)
	}
	if status.BypassMode {
		t.Error("Expected BypassMode to be false")
	}
}

func TestHTTPMetronClient_GetSessionStatus_NoSession(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"active":      false,
			"server_time": now.Format(time.RFC3339),
			"bypass_mode": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if status.Active {
		t.Error("Expected Active to be false")
	}
	if status.SessionID != nil {
		t.Errorf("Expected SessionID to be nil, got %v", status.SessionID)
	}
}

func TestHTTPMetronClient_GetSessionStatus_BypassMode(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"active":      false,
			"server_time": now.Format(time.RFC3339),
			"bypass_mode": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !status.BypassMode {
		t.Error("Expected BypassMode to be true")
	}
}

func TestHTTPMetronClient_GetSessionStatus_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "wrong-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err == nil {
		t.Fatal("Expected error for unauthorized request")
	}
	if status != nil {
		t.Errorf("Expected nil status on error, got %v", status)
	}
}

func TestHTTPMetronClient_GetSessionStatus_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err == nil {
		t.Fatal("Expected error for server error response")
	}
	if status != nil {
		t.Errorf("Expected nil status on error, got %v", status)
	}
}

func TestHTTPMetronClient_GetSessionStatus_NetworkError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	// Use a URL that won't connect
	client := NewHTTPMetronClient("http://localhost:1", "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err == nil {
		t.Fatal("Expected error for network failure")
	}
	if status != nil {
		t.Errorf("Expected nil status on error, got %v", status)
	}
}

func TestHTTPMetronClient_GetSessionStatus_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx := context.Background()
	status, err := client.GetSessionStatus(ctx, "test-device")

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if status != nil {
		t.Errorf("Expected nil status on error, got %v", status)
	}
}

func TestHTTPMetronClient_GetSessionStatus_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		response := map[string]interface{}{
			"active":      true,
			"server_time": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	client := NewHTTPMetronClient(server.URL, "test-token", logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	status, err := client.GetSessionStatus(ctx, "test-device")

	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
	if status != nil {
		t.Errorf("Expected nil status on error, got %v", status)
	}
}
