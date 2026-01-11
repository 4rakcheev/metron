package middleware

import (
	"metron/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// AgentDeviceIDKey is the context key for the authenticated device ID
	AgentDeviceIDKey = "agent_device_id"
	// AgentDeviceNameKey is the context key for the authenticated device name
	AgentDeviceNameKey = "agent_device_name"
)

// AgentAuth validates agent tokens from Authorization Bearer header.
// Tokens are looked up from device parameters (agent_token field).
// On success, sets device_id in context for handler use.
func AgentAuth(devices []config.DeviceConfig) gin.HandlerFunc {
	// Build a lookup map from token -> device for O(1) lookup
	tokenToDevice := make(map[string]*config.DeviceConfig)
	for i := range devices {
		device := &devices[i]
		if token := getDeviceAgentToken(device); token != "" {
			tokenToDevice[token] = device
		}
	}

	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
				"code":  "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// Check Bearer scheme
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization scheme. Use Bearer token.",
				"code":  "INVALID_AUTH_SCHEME",
			})
			c.Abort()
			return
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, bearerPrefix)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token required",
				"code":  "TOKEN_REQUIRED",
			})
			c.Abort()
			return
		}

		// Find matching device by token
		device, found := tokenToDevice[token]
		if !found {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		// Check if agent is enabled for this device (defaults to true if not specified)
		if !isAgentEnabled(device) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Agent is disabled for this device",
				"code":  "AGENT_DISABLED",
			})
			c.Abort()
			return
		}

		// Set device ID and name in context for handlers
		c.Set(AgentDeviceIDKey, device.ID)
		c.Set(AgentDeviceNameKey, device.Name)

		c.Next()
	}
}

// getDeviceAgentToken extracts the agent_token from device parameters
func getDeviceAgentToken(device *config.DeviceConfig) string {
	if device.Parameters == nil {
		return ""
	}
	if token, ok := device.Parameters["agent_token"].(string); ok {
		return token
	}
	return ""
}

// isAgentEnabled checks if the agent is enabled for a device
// Defaults to true if agent_enabled is not specified
func isAgentEnabled(device *config.DeviceConfig) bool {
	if device.Parameters == nil {
		return true
	}
	if enabled, ok := device.Parameters["agent_enabled"].(bool); ok {
		return enabled
	}
	return true // Default to enabled
}

// HasAgentDevices returns true if any device has an agent token configured
func HasAgentDevices(devices []config.DeviceConfig) bool {
	for i := range devices {
		if getDeviceAgentToken(&devices[i]) != "" {
			return true
		}
	}
	return false
}
