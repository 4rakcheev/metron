package winagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"metron/internal/agentupdate"
)

// UpdateClient fetches published agent builds from the Metron server
type UpdateClient interface {
	GetUpdateManifest(ctx context.Context, currentVersion string) (*agentupdate.Manifest, error)
	DownloadUpdate(ctx context.Context, w io.Writer) error
}

// BinaryVerifier runs a downloaded binary and checks it reports the expected version
type BinaryVerifier func(ctx context.Context, path, wantVersion string) error

// UpdaterConfig configures the self-updater
type UpdaterConfig struct {
	ExePath        string // Installed agent binary (replaced in place)
	CurrentVersion string // Version compiled into the running binary
	// PublicKey is the base64 ed25519 key compiled into the binary.
	// When set, unsigned or wrongly signed updates are rejected.
	PublicKey string
}

// Updater replaces the installed agent binary with the build published on the server
type Updater struct {
	client UpdateClient
	verify BinaryVerifier
	config UpdaterConfig
	logger *slog.Logger
}

// NewUpdater creates an updater
func NewUpdater(client UpdateClient, verify BinaryVerifier, config UpdaterConfig, logger *slog.Logger) *Updater {
	return &Updater{
		client: client,
		verify: verify,
		config: config,
		logger: logger.With("component", "updater"),
	}
}

// RunOnce checks for an update and installs it. Returns true when the binary was replaced.
func (u *Updater) RunOnce(ctx context.Context) (bool, error) {
	manifest, err := u.client.GetUpdateManifest(ctx, u.config.CurrentVersion)
	if err != nil {
		return false, fmt.Errorf("check for update: %w", err)
	}
	if manifest == nil {
		u.logger.Debug("no update published")
		return false, nil
	}

	installedHash, err := fileSHA256(u.config.ExePath)
	if err != nil {
		return false, fmt.Errorf("hash installed binary: %w", err)
	}
	if strings.EqualFold(installedHash, manifest.SHA256) {
		u.logger.Debug("agent is up to date", "version", manifest.Version)
		return false, nil
	}

	u.logger.Info("update available",
		"current_version", u.config.CurrentVersion,
		"new_version", manifest.Version,
	)

	if u.config.PublicKey != "" {
		if err := manifest.VerifySignature(u.config.PublicKey); err != nil {
			return false, fmt.Errorf("reject update %s: %w", manifest.Version, err)
		}
	} else {
		u.logger.Warn("no update public key compiled in, accepting update by hash only")
	}

	newPath := u.config.ExePath + ".new"
	defer os.Remove(newPath) // no-op after a successful rename

	if err := u.download(ctx, newPath, manifest); err != nil {
		return false, err
	}

	if u.verify != nil {
		if err := u.verify(ctx, newPath, manifest.Version); err != nil {
			return false, fmt.Errorf("new binary failed self-check: %w", err)
		}
	}

	if err := swapBinary(u.config.ExePath, newPath); err != nil {
		return false, fmt.Errorf("install update: %w", err)
	}

	u.logger.Info("update installed", "version", manifest.Version)
	return true, nil
}

// download writes the published binary to path and checks its size and hash against the manifest
func (u *Updater) download(ctx context.Context, path string, manifest *agentupdate.Manifest) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	hash := sha256.New()
	counter := &countingWriter{}
	err = u.client.DownloadUpdate(ctx, io.MultiWriter(file, hash, counter))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}

	if counter.n != manifest.Size {
		return fmt.Errorf("downloaded %d bytes, manifest says %d", counter.n, manifest.Size)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, manifest.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, manifest.SHA256)
	}
	return nil
}

// swapBinary moves newPath over exePath, keeping the previous binary as exePath.old.
// Windows allows renaming a running executable, so the agent keeps running on the old image
// until it notices the change and restarts itself.
func swapBinary(exePath, newPath string) error {
	oldPath := exePath + ".old"
	if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		// A previous .old may still be mapped by a process that has not restarted yet
		return fmt.Errorf("remove previous backup: %w", err)
	}
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("back up current binary: %w", err)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		if rbErr := os.Rename(oldPath, exePath); rbErr != nil {
			return fmt.Errorf("activate new binary: %w (rollback failed: %v)", err, rbErr)
		}
		return fmt.Errorf("activate new binary: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum, _, err := agentupdate.HashReader(file)
	return sum, err
}

type countingWriter struct{ n int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}
