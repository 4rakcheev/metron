package handlers

import (
	"log/slog"
	"metron/internal/devices"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DevicesHandler handles device-related requests
type DevicesHandler struct {
	registry DriverRegistry
	logger   *slog.Logger
}

// DriverRegistry interface for accessing device drivers
type DriverRegistry interface {
	List() []string
	Get(name string) (devices.DeviceDriver, error)
}

// NewDevicesHandler creates a new devices handler
func NewDevicesHandler(registry DriverRegistry, logger *slog.Logger) *DevicesHandler {
	return &DevicesHandler{
		registry: registry,
		logger:   logger,
	}
}

// ListDevices returns all available device types
// GET /devices
func (h *DevicesHandler) ListDevices(c *gin.Context) {
	driverNames := h.registry.List()

	response := make([]gin.H, 0, len(driverNames))
	for _, name := range driverNames {
		driver, err := h.registry.Get(name)
		if err != nil {
			h.logger.Error("Failed to get driver",
				"component", "api",
				"driver_name", name,
				"error", err,
			)
			continue
		}

		deviceInfo := gin.H{
			"type": name,
			"name": name,
		}

		// Check if driver supports capabilities
		if capableDriver, ok := driver.(devices.CapableDriver); ok {
			caps := capableDriver.Capabilities()
			deviceInfo["capabilities"] = gin.H{
				"supports_warnings":   caps.SupportsWarnings,
				"supports_live_state": caps.SupportsLiveState,
				"supports_scheduling": caps.SupportsScheduling,
			}
		}

		response = append(response, deviceInfo)
	}

	c.JSON(http.StatusOK, response)
}
