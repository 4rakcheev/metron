package winagent

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"metron/internal/agentupdate"
)

type fakeUpdateClient struct {
	manifest    *agentupdate.Manifest
	manifestErr error
	binary      []byte
	downloads   int
}

func (f *fakeUpdateClient) GetUpdateManifest(ctx context.Context, currentVersion string) (*agentupdate.Manifest, error) {
	return f.manifest, f.manifestErr
}

func (f *fakeUpdateClient) DownloadUpdate(ctx context.Context, w io.Writer) error {
	f.downloads++
	_, err := w.Write(f.binary)
	return err
}

func manifestFor(t *testing.T, version string, binary []byte) *agentupdate.Manifest {
	t.Helper()
	sum, size, err := agentupdate.HashReader(strings.NewReader(string(binary)))
	if err != nil {
		t.Fatal(err)
	}
	return &agentupdate.Manifest{Version: version, SHA256: sum, Size: size}
}

func setupInstalled(t *testing.T, content string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "metron-win-agent.exe")
	if err := os.WriteFile(exe, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func newTestUpdater(client UpdateClient, verify BinaryVerifier, exe, publicKey string) *Updater {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewUpdater(client, verify, UpdaterConfig{
		ExePath:        exe,
		CurrentVersion: "v1",
		PublicKey:      publicKey,
	}, logger)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestUpdater_InstallsSignedUpdate(t *testing.T) {
	pub, priv, err := agentupdate.GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	exe := setupInstalled(t, "old binary")
	newBinary := []byte("new binary")
	manifest := manifestFor(t, "v2", newBinary)
	if err := manifest.Sign(priv); err != nil {
		t.Fatal(err)
	}

	var verifiedVersion string
	verify := func(ctx context.Context, path, want string) error {
		verifiedVersion = want
		if got := readFile(t, path); got != string(newBinary) {
			t.Errorf("verifier saw %q", got)
		}
		return nil
	}

	updated, err := newTestUpdater(&fakeUpdateClient{manifest: manifest, binary: newBinary}, verify, exe, pub).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !updated {
		t.Fatal("expected update to be installed")
	}
	if verifiedVersion != "v2" {
		t.Errorf("verifier called with %q, want v2", verifiedVersion)
	}
	if got := readFile(t, exe); got != "new binary" {
		t.Errorf("installed binary = %q", got)
	}
	if got := readFile(t, exe+".old"); got != "old binary" {
		t.Errorf("backup = %q", got)
	}
	if _, err := os.Stat(exe + ".new"); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected .new to be gone")
	}
}

func TestUpdater_UpToDate_NoDownload(t *testing.T) {
	exe := setupInstalled(t, "same binary")
	client := &fakeUpdateClient{manifest: manifestFor(t, "v2", []byte("same binary"))}

	updated, err := newTestUpdater(client, nil, exe, "").RunOnce(context.Background())
	if err != nil || updated {
		t.Fatalf("expected no update, got updated=%v err=%v", updated, err)
	}
	if client.downloads != 0 {
		t.Errorf("expected no download, got %d", client.downloads)
	}
}

func TestUpdater_NothingPublished(t *testing.T) {
	exe := setupInstalled(t, "old binary")

	updated, err := newTestUpdater(&fakeUpdateClient{}, nil, exe, "").RunOnce(context.Background())
	if err != nil || updated {
		t.Fatalf("expected no update, got updated=%v err=%v", updated, err)
	}
}

func TestUpdater_RejectsHashMismatch(t *testing.T) {
	exe := setupInstalled(t, "old binary")
	manifest := manifestFor(t, "v2", []byte("expected binary"))
	client := &fakeUpdateClient{manifest: manifest, binary: []byte("tampered binary")}

	updated, err := newTestUpdater(client, nil, exe, "").RunOnce(context.Background())
	if err == nil || updated {
		t.Fatalf("expected hash mismatch error, got updated=%v err=%v", updated, err)
	}
	assertUntouched(t, exe)
}

func TestUpdater_RejectsBadSignature(t *testing.T) {
	pub, _, _ := agentupdate.GenerateKeyPair(rand.Reader)
	_, otherPriv, _ := agentupdate.GenerateKeyPair(rand.Reader)
	exe := setupInstalled(t, "old binary")
	newBinary := []byte("new binary")
	manifest := manifestFor(t, "v2", newBinary)
	if err := manifest.Sign(otherPriv); err != nil {
		t.Fatal(err)
	}
	client := &fakeUpdateClient{manifest: manifest, binary: newBinary}

	updated, err := newTestUpdater(client, nil, exe, pub).RunOnce(context.Background())
	if !errors.Is(err, agentupdate.ErrInvalidSignature) || updated {
		t.Fatalf("expected signature error, got updated=%v err=%v", updated, err)
	}
	if client.downloads != 0 {
		t.Error("expected no download for an untrusted manifest")
	}
	assertUntouched(t, exe)
}

func TestUpdater_RejectsUnsignedWhenKeyCompiledIn(t *testing.T) {
	pub, _, _ := agentupdate.GenerateKeyPair(rand.Reader)
	exe := setupInstalled(t, "old binary")
	newBinary := []byte("new binary")
	client := &fakeUpdateClient{manifest: manifestFor(t, "v2", newBinary), binary: newBinary}

	updated, err := newTestUpdater(client, nil, exe, pub).RunOnce(context.Background())
	if !errors.Is(err, agentupdate.ErrInvalidSignature) || updated {
		t.Fatalf("expected signature error, got updated=%v err=%v", updated, err)
	}
	assertUntouched(t, exe)
}

func TestUpdater_SelfCheckFailureKeepsCurrentBinary(t *testing.T) {
	exe := setupInstalled(t, "old binary")
	newBinary := []byte("broken binary")
	client := &fakeUpdateClient{manifest: manifestFor(t, "v2", newBinary), binary: newBinary}
	verify := func(ctx context.Context, path, want string) error { return errors.New("crashed") }

	updated, err := newTestUpdater(client, verify, exe, "").RunOnce(context.Background())
	if err == nil || updated {
		t.Fatalf("expected self-check error, got updated=%v err=%v", updated, err)
	}
	assertUntouched(t, exe)
}

func assertUntouched(t *testing.T, exe string) {
	t.Helper()
	if got := readFile(t, exe); got != "old binary" {
		t.Errorf("installed binary changed to %q", got)
	}
	if _, err := os.Stat(exe + ".new"); !errors.Is(err, os.ErrNotExist) {
		t.Error("expected temporary .new file to be removed")
	}
}

func TestWatchExecutable_FiresAfterReplacement(t *testing.T) {
	exe := setupInstalled(t, "old binary")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fired := make(chan struct{})
	go WatchExecutable(ctx, exe, 20*time.Millisecond, logger, func() { close(fired) })

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(exe, []byte("new binary, longer"), 0o755); err != nil {
		t.Fatal(err)
	}

	select {
	case <-fired:
	case <-ctx.Done():
		t.Fatal("expected onChange after the binary was replaced")
	}
}

func TestWatchExecutable_NoChangeNoFire(t *testing.T) {
	exe := setupInstalled(t, "old binary")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	fired := false
	WatchExecutable(ctx, exe, 20*time.Millisecond, logger, func() { fired = true })
	if fired {
		t.Error("onChange fired without a change")
	}
}
