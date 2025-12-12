package handlers

import (
	"log/slog"
	"metron/internal/devices"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DevicesHandler handles device-related requests
type DevicesHandler struct {
	deviceRegistry *devices.Registry
	driverRegistry DriverRegistry
	logger         *slog.Logger
}

// DriverRegistry interface for accessing device drivers
type DriverRegistry interface {
	List() []string
	Get(name string) (devices.DeviceDriver, error)
}

// NewDevicesHandler creates a new devices handler
func NewDevicesHandler(deviceRegistry *devices.Registry, driverRegistry DriverRegistry, logger *slog.Logger) *DevicesHandler {
	return &DevicesHandler{
		deviceRegistry: deviceRegistry,
		driverRegistry: driverRegistry,
		logger:         logger,
	}
}

// ListDevices returns all available devices
// GET /devices
func (h *DevicesHandler) ListDevices(c *gin.Context) {
	deviceList := h.deviceRegistry.List()

	response := make([]gin.H, 0, len(deviceList))
	for _, device := range deviceList {
		deviceInfo := gin.H{
			"id":   device.ID,
			"name": device.Name,
			"type": device.Type,
		}

		// Get driver capabilities
		driver, err := h.driverRegistry.Get(device.Driver)
		if err != nil {
			h.logger.Error("Failed to get driver for device",
				"component", "api",
				"device_id", device.ID,
				"driver_name", device.Driver,
				"error", err,
			)
			// Include device without capabilities
			response = append(response, deviceInfo)
			continue
		}

		// Add driver capabilities
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
