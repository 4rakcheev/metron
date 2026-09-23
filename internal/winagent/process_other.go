//go:build !windows

package winagent

import (
	"log/slog"
	"os/exec"
)

func hideWindow(*exec.Cmd) {}

// EnsureAgentRunning is a no-op outside Windows (development builds)
func EnsureAgentRunning(exePath, taskName string, logger *slog.Logger) error {
	logger.Debug("agent watchdog is only supported on Windows")
	return nil
}
