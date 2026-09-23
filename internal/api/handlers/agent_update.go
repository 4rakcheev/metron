package handlers

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"metron/internal/agentupdate"
	"metron/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// AgentUpdateHandler serves the Windows agent build published by CI
type AgentUpdateHandler struct {
	dir    string
	logger *slog.Logger
}

// NewAgentUpdateHandler creates a handler serving updates from dir
func NewAgentUpdateHandler(dir string, logger *slog.Logger) *AgentUpdateHandler {
	return &AgentUpdateHandler{
		dir:    dir,
		logger: logger.With("component", "agent-update-api"),
	}
}

// GetManifest returns the currently published agent manifest.
// GET /v1/agent/update
func (h *AgentUpdateHandler) GetManifest(c *gin.Context) {
	manifest, err := h.loadManifest()
	if errors.Is(err, fs.ErrNotExist) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No agent update published",
			"code":  "UPDATE_NOT_AVAILABLE",
		})
		return
	}
	if err != nil {
		h.logger.Error("failed to load agent update manifest", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load update manifest",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	deviceID, _ := c.Get(middleware.AgentDeviceIDKey)
	h.logger.Debug("agent update manifest requested",
		"device_id", deviceID,
		"agent_version", c.Query("version"),
		"published_version", manifest.Version,
	)
	if current := c.Query("version"); current != "" && current != manifest.Version {
		h.logger.Info("agent update available",
			"device_id", deviceID,
			"agent_version", current,
			"published_version", manifest.Version,
		)
	}

	c.JSON(http.StatusOK, manifest)
}

// Download streams the published agent binary.
// GET /v1/agent/update/download
func (h *AgentUpdateHandler) Download(c *gin.Context) {
	path := filepath.Join(h.dir, agentupdate.BinaryFile)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No agent update published",
			"code":  "UPDATE_NOT_AVAILABLE",
		})
		return
	}

	deviceID, _ := c.Get(middleware.AgentDeviceIDKey)
	h.logger.Info("agent update downloaded", "device_id", deviceID, "size", info.Size())

	c.Header("Content-Type", "application/octet-stream")
	c.File(path)
}

func (h *AgentUpdateHandler) loadManifest() (*agentupdate.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(h.dir, agentupdate.ManifestFile))
	if err != nil {
		return nil, err
	}
	var manifest agentupdate.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}
