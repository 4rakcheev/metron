package aqara

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"metron/internal/core"
	"metron/internal/devices"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config contains Aqara Cloud API configuration
type Config struct {
	AppID       string
	AppKey      string
	KeyID       string
	AccessToken string
	BaseURL     string
	PINSceneID  string
	WarnSceneID string
	OffSceneID  string
}

// Driver implements the DeviceDriver interface for Aqara Cloud
type Driver struct {
	config     Config
	httpClient *http.Client
}

// NewDriver creates a new Aqara driver
func NewDriver(config Config) *Driver {
	return &Driver{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the driver name
func (d *Driver) Name() string {
	return "aqara"
}

// StartSession initiates a session by triggering the PIN entry scene
func (d *Driver) StartSession(ctx context.Context, session *core.Session) error {
	if d.config.PINSceneID == "" {
		return fmt.Errorf("PIN scene ID not configured")
	}
	return d.triggerScene(ctx, d.config.PINSceneID)
}

// StopSession ends a session by triggering the power-off scene
func (d *Driver) StopSession(ctx context.Context, session *core.Session) error {
	if d.config.OffSceneID == "" {
		return fmt.Errorf("power-off scene ID not configured")
	}
	return d.triggerScene(ctx, d.config.OffSceneID)
}

// ApplyWarning sends a warning by triggering the warning scene
func (d *Driver) ApplyWarning(ctx context.Context, session *core.Session, minutesRemaining int) error {
	if d.config.WarnSceneID == "" {
		// Warning is optional
		return nil
	}
	return d.triggerScene(ctx, d.config.WarnSceneID)
}

// GetLiveState retrieves the current state of a device (not supported in MVP)
func (d *Driver) GetLiveState(ctx context.Context, deviceID string) (*devices.DeviceState, error) {
	// Not implemented in MVP
	return nil, nil
}

// Capabilities returns the driver capabilities
func (d *Driver) Capabilities() devices.DriverCapabilities {
	return devices.DriverCapabilities{
		SupportsWarnings:   d.config.WarnSceneID != "",
		SupportsLiveState:  false,
		SupportsScheduling: true,
	}
}

// triggerScene triggers an Aqara scene via the Cloud API
func (d *Driver) triggerScene(ctx context.Context, sceneID string) error {
	// Build request
	req := map[string]interface{}{
		"intent": "config.scene.run",
		"data": map[string]interface{}{
			"sceneId": sceneID,
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v3.0/open/api", d.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	timestamp := time.Now().UnixMilli()
	nonce := generateNonce()

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accesstoken", d.config.AccessToken)
	httpReq.Header.Set("Appid", d.config.AppID)
	httpReq.Header.Set("Keyid", d.config.KeyID)
	httpReq.Header.Set("Time", strconv.FormatInt(timestamp, 10))
	httpReq.Header.Set("Nonce", nonce)
	httpReq.Header.Set("Sign", d.generateSignature(timestamp, nonce))

	// Send request
	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var apiResp struct {
		Code          int                    `json:"code"`
		RequestId     string                 `json:"requestId"`
		Message       string                 `json:"message"`
		MessageDetail string                 `json:"messageDetail"`
		Result        interface{}            `json:"result"` // Can be string, object, or null
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 0 {
		return fmt.Errorf("API returned error code %d: %s (%s)", apiResp.Code, apiResp.Message, apiResp.MessageDetail)
	}

	return nil
}

// generateSignature generates the request signature for Aqara Cloud API
// According to official documentation: https://opendoc.aqara.cn/en/docs/developmanual/apiIntroduction/signGenerationRules.html
// Parameters must be sorted by ASCII: Accesstoken, Appid, Keyid, Nonce, Time
// Format: Accesstoken=xxx&Appid=xxx&Keyid=xxx&Nonce=xxx&Time=xxx
// Append Appkey directly (no separator)
// Convert to lowercase, then apply MD5 hash
func (d *Driver) generateSignature(timestamp int64, nonce string) string {
	// Build parameter string in ASCII order: Accesstoken, Appid, Keyid, Nonce, Time
	params := []struct {
		key   string
		value string
	}{
		{"Accesstoken", d.config.AccessToken},
		{"Appid", d.config.AppID},
		{"Keyid", d.config.KeyID},
		{"Nonce", nonce},
		{"Time", strconv.FormatInt(timestamp, 10)},
	}

	// Build signature base string
	var sb strings.Builder
	for i, p := range params {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(p.key)
		sb.WriteString("=")
		sb.WriteString(p.value)
	}

	// Append app key directly (no separator)
	sb.WriteString(d.config.AppKey)

	// Convert to lowercase
	signStr := strings.ToLower(sb.String())

	// Calculate MD5 hash
	hash := md5.Sum([]byte(signStr))

	// Debug: print signature components (uncomment for debugging)
	// fmt.Printf("DEBUG Signature String: %s\n", signStr)
	// fmt.Printf("DEBUG Signature: %s\n", hex.EncodeToString(hash[:]))

	return hex.EncodeToString(hash[:])
}

// generateNonce generates a random nonce for API requests
func generateNonce() string {
	// Use a simpler format to avoid scientific notation issues
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
