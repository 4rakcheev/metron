package winagent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// VerifyBinaryVersion runs "<path> -version" and checks the printed version
func VerifyBinaryVersion(ctx context.Context, path, wantVersion string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, path, "-version")
	cmd.Stdout = &out
	cmd.Stderr = &out
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s -version: %w (output: %q)", path, err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != wantVersion {
		return fmt.Errorf("binary reports version %q, want %q", got, wantVersion)
	}
	return nil
}

// fileStamp identifies a file version cheaply
type fileStamp struct {
	size    int64
	modTime time.Time
}

func statStamp(path string) (fileStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{size: info.Size(), modTime: info.ModTime()}, nil
}

// WatchExecutable calls onChange once the file at path is replaced (e.g. by the updater)
// and has stayed unchanged for one extra interval, so a half-written file is never started.
// Blocks until ctx is cancelled or onChange has been called.
func WatchExecutable(ctx context.Context, path string, interval time.Duration, logger *slog.Logger, onChange func()) {
	initial, err := statStamp(path)
	if err != nil {
		logger.Warn("cannot watch executable, self-restart after update disabled", "path", path, "error", err)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pending *fileStamp
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		current, err := statStamp(path)
		if err != nil || current == initial {
			pending = nil
			continue
		}
		if pending == nil || *pending != current {
			pending = &current
			continue
		}

		logger.Info("agent binary replaced on disk, restarting", "path", path)
		onChange()
		return
	}
}

// RestartSelf starts path with the current arguments and returns once the new process is running.
// The caller exits afterwards; the new process performs its first poll immediately.
func RestartSelf(path string) error {
	cmd := exec.Command(path, os.Args[1:]...)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
