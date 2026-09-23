package agentupdate

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

func testManifest() *Manifest {
	return &Manifest{
		Version: "20260923.1642-abc1234",
		SHA256:  strings.Repeat("ab", 32),
		Size:    1024,
	}
}

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	m := testManifest()
	if err := m.Sign(priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := m.VerifySignature(pub); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestVerify_RejectsTamperedFields(t *testing.T) {
	pub, priv, err := GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	tests := map[string]func(m *Manifest){
		"version": func(m *Manifest) { m.Version = "other" },
		"sha256":  func(m *Manifest) { m.SHA256 = strings.Repeat("cd", 32) },
		"size":    func(m *Manifest) { m.Size = 2048 },
	}
	for name, tamper := range tests {
		t.Run(name, func(t *testing.T) {
			m := testManifest()
			if err := m.Sign(priv); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			tamper(m)
			if err := m.VerifySignature(pub); !errors.Is(err, ErrInvalidSignature) {
				t.Errorf("expected ErrInvalidSignature, got %v", err)
			}
		})
	}
}

func TestVerify_RejectsUnsignedAndWrongKey(t *testing.T) {
	pub, _, _ := GenerateKeyPair(rand.Reader)
	_, otherPriv, _ := GenerateKeyPair(rand.Reader)

	m := testManifest()
	if err := m.VerifySignature(pub); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("unsigned: expected ErrInvalidSignature, got %v", err)
	}

	if err := m.Sign(otherPriv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := m.VerifySignature(pub); !errors.Is(err, ErrInvalidSignature) {
		t.Errorf("wrong key: expected ErrInvalidSignature, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	if err := testManifest().Validate(); err != nil {
		t.Errorf("valid manifest rejected: %v", err)
	}

	bad := testManifest()
	bad.SHA256 = "xyz"
	if err := bad.Validate(); err == nil {
		t.Error("expected error for bad sha256")
	}

	bad = testManifest()
	bad.Version = ""
	if err := bad.Validate(); err == nil {
		t.Error("expected error for empty version")
	}
}

func TestHashReader(t *testing.T) {
	sum, n, err := HashReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}
	if n != 5 {
		t.Errorf("size = %d, want 5", n)
	}
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if sum != want {
		t.Errorf("sha256 = %s, want %s", sum, want)
	}
}
