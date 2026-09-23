// Package agentupdate defines the update manifest shared by the Metron server,
// the Windows agent updater and the CI signing tool.
package agentupdate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// ManifestFile is the manifest file name inside the server's update directory
	ManifestFile = "manifest.json"
	// BinaryFile is the agent binary file name inside the server's update directory
	BinaryFile = "metron-win-agent.exe"
)

// ErrInvalidSignature is returned when a manifest signature does not match
var ErrInvalidSignature = errors.New("invalid update signature")

// Manifest describes the agent build currently published by the server
type Manifest struct {
	Version   string `json:"version"`             // Build version, e.g. "20260923.1642-0176dec"
	SHA256    string `json:"sha256"`              // Hex SHA256 of the binary
	Size      int64  `json:"size"`                // Binary size in bytes
	Signature string `json:"signature,omitempty"` // Base64 ed25519 signature of SigningMessage()
	BuiltAt   string `json:"built_at,omitempty"`  // RFC3339 build time (informational)
}

// Validate checks that required manifest fields are present and well-formed
func (m *Manifest) Validate() error {
	if m.Version == "" {
		return errors.New("manifest: version is required")
	}
	if len(m.SHA256) != sha256.Size*2 {
		return fmt.Errorf("manifest: sha256 must be %d hex chars", sha256.Size*2)
	}
	if _, err := hex.DecodeString(m.SHA256); err != nil {
		return fmt.Errorf("manifest: invalid sha256: %w", err)
	}
	if m.Size <= 0 {
		return errors.New("manifest: size must be positive")
	}
	return nil
}

// SigningMessage returns the bytes covered by the signature.
// It binds the version to the binary hash so a signed binary cannot be replayed under another version.
func (m *Manifest) SigningMessage() []byte {
	return []byte("metron-win-agent:" + m.Version + ":" + strings.ToLower(m.SHA256) + ":" + fmt.Sprint(m.Size))
}

// Sign sets the manifest signature using a base64-encoded ed25519 private key
func (m *Manifest) Sign(privateKeyB64 string) error {
	key, err := decodeKey(privateKeyB64, ed25519.PrivateKeySize)
	if err != nil {
		return fmt.Errorf("private key: %w", err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), m.SigningMessage()))
	return nil
}

// VerifySignature checks the manifest signature against a base64-encoded ed25519 public key
func (m *Manifest) VerifySignature(publicKeyB64 string) error {
	key, err := decodeKey(publicKeyB64, ed25519.PublicKeySize)
	if err != nil {
		return fmt.Errorf("public key: %w", err)
	}
	if m.Signature == "" {
		return fmt.Errorf("%w: manifest is not signed", ErrInvalidSignature)
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	if !ed25519.Verify(ed25519.PublicKey(key), m.SigningMessage(), sig) {
		return ErrInvalidSignature
	}
	return nil
}

// GenerateKeyPair returns a new base64-encoded ed25519 key pair
func GenerateKeyPair(rand io.Reader) (publicKeyB64, privateKeyB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), nil
}

// HashReader returns the hex SHA256 and size of everything read from r
func HashReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func decodeKey(b64 string, size int) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	if len(key) != size {
		return nil, fmt.Errorf("expected %d bytes, got %d", size, len(key))
	}
	return key, nil
}
