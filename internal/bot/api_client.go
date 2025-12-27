package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// MetronAPI is a client for the Metron REST API
type MetronAPI struct {
	baseURL string
	apiKey  string
	client  *http.Client
	logger  *slog.Logger
}

// NewMetronAPI creates a new Metron API client
func NewMetronAPI(baseURL, apiKey string, logger *slog.Logger) *MetronAPI {
	return &MetronAPI{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// TodayStats represents today's statistics response
type TodayStats struct {
	Date           string       `json:"date"`
	Children       []ChildStats `json:"children"`
	ActiveSessions int          `json:"active_sessions"`
	TotalChildren  int          `json:"total_children"`
}

// ChildStats represents a child's daily statistics
type ChildStats struct {
	ChildID        string `json:"child_id"`
	ChildName      string `json:"child_name"`
	TodayUsed      int    `json:"today_used"`
	TodayRemaining int    `json:"today_remaining"`
	TodayLimit     int    `json:"today_limit"`
	SessionsToday  int    `json:"sessions_today"`
	UsagePercent   int    `json:"usage_percent"`
}

// Child represents a child
type Child struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Emoji           string     `json:"emoji"`
	WeekdayLimit    int        `json:"weekday_limit"`
	WeekendLimit    int        `json:"weekend_limit"`
	BreakRule       *BreakRule `json:"break_rule,omitempty"`
	DowntimeEnabled bool       `json:"downtime_enabled"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
}

// BreakRule represents break rule settings
type BreakRule struct {
	BreakAfterMinutes    int `json:"break_after_minutes"`
	BreakDurationMinutes int `json:"break_duration_minutes"`
}

// Device represents a device
type Device struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	Capabilities DeviceCapabilities `json:"capabilities,omitempty"`
}

// DeviceCapabilities represents device capabilities
type DeviceCapabilities struct {
	SupportsWarnings   bool `json:"supports_warnings"`
	SupportsLiveState  bool `json:"supports_live_state"`
	SupportsScheduling bool `json:"supports_scheduling"`
}

// Session represents a screen-time session
type Session struct {
	ID               string   `json:"id"`
	DeviceType       string   `json:"device_type"`
	DeviceID         string   `json:"device_id"`
	ChildIDs         []string `json:"child_ids"`
	StartTime        string   `json:"start_time"`
	ExpectedDuration int      `json:"expected_duration"`
	RemainingMinutes int      `json:"remaining_minutes"`
	Status           string   `json:"status"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

// CreateSessionRequest represents a request to create a session
type CreateSessionRequest struct {
	DeviceID string   `json:"device_id"`
	ChildIDs []string `json:"child_ids"`
	Minutes  int      `json:"minutes"`
}

// ExtendSessionRequest represents a request to extend a session
type ExtendSessionRequest struct {
	Action            string `json:"action"`
	AdditionalMinutes int    `json:"additional_minutes,omitempty"`
}

// APIError represents an API error response
type APIError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// GetTodayStats retrieves today's statistics
func (a *MetronAPI) GetTodayStats(ctx context.Context) (*TodayStats, error) {
	var stats TodayStats
	if err := a.doRequest(ctx, "GET", "/v1/stats/today", nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// ListChildren retrieves all children
func (a *MetronAPI) ListChildren(ctx context.Context) ([]Child, error) {
	var children []Child
	if err := a.doRequest(ctx, "GET", "/v1/children", nil, &children); err != nil {
		return nil, err
	}
	return children, nil
}

// ListDevices retrieves all available device types
func (a *MetronAPI) ListDevices(ctx context.Context) ([]Device, error) {
	var devices []Device
	if err := a.doRequest(ctx, "GET", "/v1/devices", nil, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// ListSessions retrieves sessions with optional filters
func (a *MetronAPI) ListSessions(ctx context.Context, active bool, childID string) ([]Session, error) {
	url := "/v1/sessions"
	if active {
		url += "?active=true"
	} else if childID != "" {
		url += "?childId=" + childID
	}

	var sessions []Session
	if err := a.doRequest(ctx, "GET", url, nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// CreateSession creates a new session
func (a *MetronAPI) CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	var session Session
	if err := a.doRequest(ctx, "POST", "/v1/sessions", req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ExtendSession extends an existing session
func (a *MetronAPI) ExtendSession(ctx context.Context, sessionID string, additionalMinutes int) (*Session, error) {
	req := ExtendSessionRequest{
		Action:            "extend",
		AdditionalMinutes: additionalMinutes,
	}

	var session Session
	if err := a.doRequest(ctx, "PATCH", "/v1/sessions/"+sessionID, req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// StopSession stops an active session
func (a *MetronAPI) StopSession(ctx context.Context, sessionID string) error {
	req := ExtendSessionRequest{
		Action: "stop",
	}
	return a.doRequest(ctx, "PATCH", "/v1/sessions/"+sessionID, req, nil)
}

// AddChildrenToSession adds one or more children to an active session
func (a *MetronAPI) AddChildrenToSession(ctx context.Context, sessionID string, childIDs []string) (*Session, error) {
	req := struct {
		Action   string   `json:"action"`
		ChildIDs []string `json:"child_ids"`
	}{
		Action:   "add_children",
		ChildIDs: childIDs,
	}

	var session Session
	if err := a.doRequest(ctx, "PATCH", "/v1/sessions/"+sessionID, req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// GrantRewardResponse represents the response from granting a reward
type GrantRewardResponse struct {
	Message            string `json:"message"`
	MinutesGranted     int    `json:"minutes_granted"`
	TodayRewardGranted int    `json:"today_reward_granted"`
	TodayRemaining     int    `json:"today_remaining"`
	TodayLimit         int    `json:"today_limit"`
}

// GrantReward grants reward minutes to a child
func (a *MetronAPI) GrantReward(ctx context.Context, childID string, minutes int) (*GrantRewardResponse, error) {
	req := struct {
		Minutes int `json:"minutes"`
	}{
		Minutes: minutes,
	}

	var response GrantRewardResponse
	if err := a.doRequest(ctx, "POST", "/v1/children/"+childID+"/rewards", req, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateChildDowntime updates the downtime enabled status for a child
func (a *MetronAPI) UpdateChildDowntime(ctx context.Context, childID string, enabled bool) error {
	req := struct {
		DowntimeEnabled bool `json:"downtime_enabled"`
	}{
		DowntimeEnabled: enabled,
	}

	return a.doRequest(ctx, "PATCH", "/v1/children/"+childID, req, nil)
}

// doRequest performs an HTTP request to the Metron API
func (a *MetronAPI) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	url := a.baseURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Metron-Key", a.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	a.logger.Debug("API request",
		"method", method,
		"url", url,
	)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}
		return fmt.Errorf("API error %d: %s (%s)", resp.StatusCode, apiErr.Error, apiErr.Code)
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
