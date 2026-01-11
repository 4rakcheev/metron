package winagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// SessionStatus represents the response from the agent API
type SessionStatus struct {
	Active     bool       `json:"active"`
	SessionID  *string    `json:"session_id,omitempty"`
	EndsAt     *time.Time `json:"ends_at,omitempty"`
	WarnAt     *time.Time `json:"warn_at,omitempty"`
	ServerTime time.Time  `json:"server_time"`
	BypassMode bool       `json:"bypass_mode"`
}

// MetronClient interface for communicating with the Metron backend
type MetronClient interface {
	// GetSessionStatus retrieves the current session status for the configured device
	GetSessionStatus(ctx context.Context, deviceID string) (*SessionStatus, error)
}

// HTTPMetronClient implements MetronClient using HTTP
type HTTPMetronClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewHTTPMetronClient creates a new HTTP client for the Metron API
func NewHTTPMetronClient(baseURL, token string, logger *slog.Logger) *HTTPMetronClient {
	return &HTTPMetronClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With("component", "metron-client"),
	}
}

// GetSessionStatus retrieves the current session status for the device
func (c *HTTPMetronClient) GetSessionStatus(ctx context.Context, deviceID string) (*SessionStatus, error) {
	// Build URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = "/v1/agent/session"
	q := u.Query()
	q.Set("device_id", deviceID)
	u.RawQuery = q.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set auth header
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	// Execute request
	c.logger.Debug("polling session status", "url", u.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or disabled token")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("forbidden: not authorized for this device")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var status SessionStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	c.logger.Debug("session status received",
		"active", status.Active,
		"bypass_mode", status.BypassMode,
		"session_id", status.SessionID,
	)

	return &status, nil
}

// Ensure HTTPMetronClient implements MetronClient
var _ MetronClient = (*HTTPMetronClient)(nil)
