//go:build windows

package winagent

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow keeps helper processes from flashing a console window
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// EnsureAgentRunning starts the user-session agent task when no other agent process is running.
// Runs from the SYSTEM updater task, so a child who kills the agent gets it back within minutes.
func EnsureAgentRunning(exePath, taskName string, logger *slog.Logger) error {
	running, err := countOtherProcesses(filepath.Base(exePath))
	if err != nil {
		return fmt.Errorf("list processes: %w", err)
	}
	if running > 0 {
		logger.Debug("agent process is running", "count", running)
		return nil
	}

	logger.Warn("agent process not running, starting scheduled task", "task", taskName)
	cmd := exec.Command("schtasks", "/Run", "/TN", taskName)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Expected when nobody is logged on: the agent task only runs in a user session
		return fmt.Errorf("schtasks /Run %s: %w (%s)", taskName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// countOtherProcesses counts processes with the given image name, excluding the current one
func countOtherProcesses(imageName string) (int, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+imageName, "/FO", "CSV", "/NH")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		// "INFO: No tasks are running..." is not CSV
		return 0, nil
	}

	self := os.Getpid()
	count := 0
	for _, rec := range records {
		if len(rec) < 2 || !strings.EqualFold(rec[0], imageName) {
			continue
		}
		if pid, err := strconv.Atoi(rec[1]); err == nil && pid != self {
			count++
		}
	}
	return count, nil
}
