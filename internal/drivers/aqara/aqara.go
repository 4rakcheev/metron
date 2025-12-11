package aqara

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"metron/internal/core"
	"metron/internal/devices"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrRefreshTokenExpired = errors.New("refresh token has expired - please update it manually")
	ErrNoRefreshToken      = errors.New("no refresh token configured - please add one using the admin API")
)

// Config contains Aqara Cloud API configuration
type Config struct {
	AppID       string
	AppKey      string
	KeyID       string
	BaseURL     string
	PINSceneID  string
	WarnSceneID string
	OffSceneID  string
}

// Driver implements the DeviceDriver interface for Aqara Cloud
type Driver struct {
	config       Config
	storage      AqaraTokenStorage
	httpClient   *http.Client
	accessToken  string        // In-memory cached access token
	tokenExpiry  time.Time     // When the access token expires
	tokenMutex   sync.RWMutex  // Protects access token cache
}

// NewDriver creates a new Aqara driver
func NewDriver(config Config, storage AqaraTokenStorage) *Driver {
	return &Driver{
		config:  config,
		storage: storage,
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

// getAccessToken retrieves a valid access token, refreshing if necessary
func (d *Driver) getAccessToken(ctx context.Context) (string, error) {
	d.tokenMutex.RLock()
	// Check if we have a valid cached token
	if d.accessToken != "" && time.Now().Before(d.tokenExpiry) {
		token := d.accessToken
		d.tokenMutex.RUnlock()
		return token, nil
	}
	d.tokenMutex.RUnlock()

	// Need to refresh the token
	d.tokenMutex.Lock()
	defer d.tokenMutex.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if d.accessToken != "" && time.Now().Before(d.tokenExpiry) {
		return d.accessToken, nil
	}

	// Get tokens from storage
	tokens, err := d.storage.GetAqaraTokens(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get tokens from storage: %w", err)
	}
	if tokens == nil || tokens.RefreshToken == "" {
		return "", ErrNoRefreshToken
	}

	// Check if stored access token is still valid
	if tokens.AccessToken != "" && tokens.AccessTokenExpiresAt != nil && time.Now().Before(*tokens.AccessTokenExpiresAt) {
		// Use the stored token
		d.accessToken = tokens.AccessToken
		d.tokenExpiry = *tokens.AccessTokenExpiresAt
		return tokens.AccessToken, nil
	}

	// Need to refresh the access token
	newAccessToken, newRefreshToken, expiresIn, err := d.refreshAccessToken(ctx, tokens.RefreshToken)
	if err != nil {
		return "", err
	}

	// Calculate expiry time (use expiresIn minus 5 minutes as buffer)
	expiryTime := time.Now().Add(time.Duration(expiresIn-300) * time.Second)

	// Update cache
	d.accessToken = newAccessToken
	d.tokenExpiry = expiryTime

	// Save new tokens to storage
	tokens.AccessToken = newAccessToken
	tokens.RefreshToken = newRefreshToken
	tokens.AccessTokenExpiresAt = &expiryTime

	if err := d.storage.SaveAqaraTokens(ctx, tokens); err != nil {
		// Log error but don't fail - we have the token in memory
		fmt.Printf("Warning: failed to save refreshed tokens to storage: %v\n", err)
	}

	return newAccessToken, nil
}

// refreshAccessToken calls the Aqara API to refresh the access token
func (d *Driver) refreshAccessToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, expiresIn int, err error) {
	// Build request according to Aqara documentation
	// https://opendoc.aqara.com/en/docs/developmanual/authManagement/aqaraauthMode.html
	reqBody := map[string]interface{}{
		"intent": "config.auth.refreshToken",
		"data": map[string]interface{}{
			"refreshToken": refreshToken,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v3.0/open/api", d.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create refresh request: %w", err)
	}

	// Add headers for refresh request
	timestamp := time.Now().UnixMilli()
	nonce := generateNonce()

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Appid", d.config.AppID)
	httpReq.Header.Set("Keyid", d.config.KeyID)
	httpReq.Header.Set("Time", strconv.FormatInt(timestamp, 10))
	httpReq.Header.Set("Nonce", nonce)
	httpReq.Header.Set("Sign", d.generateRefreshSignature(timestamp, nonce))

	// Send request
	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to read refresh response: %w", err)
	}

	// Parse response
	var apiResp struct {
		Code          int    `json:"code"`
		RequestId     string `json:"requestId"`
		Message       string `json:"message"`
		MessageDetail string `json:"messageDetail"`
		Result        struct {
			AccessToken  string `json:"accessToken"`
			ExpiresIn    string `json:"expiresIn"` // Aqara returns this as a string
			RefreshToken string `json:"refreshToken"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	if apiResp.Code != 0 {
		// Check if it's a refresh token expiration error
		if apiResp.Code == 106 || apiResp.Code == 107 || strings.Contains(apiResp.Message, "expired") {
			return "", "", 0, ErrRefreshTokenExpired
		}
		return "", "", 0, fmt.Errorf("refresh token API returned error code %d: %s (%s)", apiResp.Code, apiResp.Message, apiResp.MessageDetail)
	}

	// Convert expiresIn from string to int
	expiresIn, parseErr := strconv.Atoi(apiResp.Result.ExpiresIn)
	if parseErr != nil {
		return "", "", 0, fmt.Errorf("failed to parse expiresIn value '%s': %w", apiResp.Result.ExpiresIn, parseErr)
	}

	return apiResp.Result.AccessToken, apiResp.Result.RefreshToken, expiresIn, nil
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
	// Get valid access token (will refresh if necessary)
	accessToken, err := d.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

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
	httpReq.Header.Set("Accesstoken", accessToken)
	httpReq.Header.Set("Appid", d.config.AppID)
	httpReq.Header.Set("Keyid", d.config.KeyID)
	httpReq.Header.Set("Time", strconv.FormatInt(timestamp, 10))
	httpReq.Header.Set("Nonce", nonce)
	httpReq.Header.Set("Sign", d.generateSignature(accessToken, timestamp, nonce))

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
func (d *Driver) generateSignature(accessToken string, timestamp int64, nonce string) string {
	// Build parameter string in ASCII order: Accesstoken, Appid, Keyid, Nonce, Time
	params := []struct {
		key   string
		value string
	}{
		{"Accesstoken", accessToken},
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

	return hex.EncodeToString(hash[:])
}

// generateRefreshSignature generates the signature for token refresh requests
// According to Aqara documentation, refresh requests don't include Accesstoken
// Parameters: Appid, Keyid, Nonce, Time (sorted by ASCII)
func (d *Driver) generateRefreshSignature(timestamp int64, nonce string) string {
	// Build parameter string in ASCII order: Appid, Keyid, Nonce, Time
	params := []struct {
		key   string
		value string
	}{
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

	return hex.EncodeToString(hash[:])
}

// generateNonce generates a random nonce for API requests
func generateNonce() string {
	// Use a simpler format to avoid scientific notation issues
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
