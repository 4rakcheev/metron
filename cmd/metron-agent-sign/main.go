// Command metron-agent-sign generates update signing keys and writes signed agent update manifests.
//
//	metron-agent-sign -genkey
//	AGENT_SIGNING_KEY=... metron-agent-sign -file build/metron-win-agent.exe -version 20260923.1642-abc1234 -out build/manifest.json
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"metron/internal/agentupdate"
)

func main() {
	genKey := flag.Bool("genkey", false, "Generate a new ed25519 key pair and exit")
	file := flag.String("file", "", "Agent binary to describe")
	version := flag.String("version", "", "Version compiled into the binary")
	out := flag.String("out", "", "Manifest output path (stdout if empty)")
	keyEnv := flag.String("key-env", "AGENT_SIGNING_KEY", "Environment variable holding the base64 private key (unsigned if unset)")
	requireSignature := flag.Bool("require-signature", false, "Fail when the private key is not available")
	flag.Parse()

	if *genKey {
		pub, priv, err := agentupdate.GenerateKeyPair(rand.Reader)
		if err != nil {
			fail("generate key: %v", err)
		}
		fmt.Printf("Public key (commit to deploy/win-agent/update-public-key.txt):\n%s\n\n", pub)
		fmt.Printf("Private key (GitHub secret AGENT_SIGNING_KEY, never commit):\n%s\n", priv)
		return
	}

	if *file == "" || *version == "" {
		fail("-file and -version are required")
	}

	f, err := os.Open(*file)
	if err != nil {
		fail("open binary: %v", err)
	}
	sum, size, err := agentupdate.HashReader(f)
	_ = f.Close()
	if err != nil {
		fail("hash binary: %v", err)
	}

	manifest := agentupdate.Manifest{
		Version: *version,
		SHA256:  sum,
		Size:    size,
		BuiltAt: time.Now().UTC().Format(time.RFC3339),
	}

	if key := os.Getenv(*keyEnv); key != "" {
		if err := manifest.Sign(key); err != nil {
			fail("sign manifest: %v", err)
		}
	} else if *requireSignature {
		fail("%s is not set but signature is required", *keyEnv)
	} else {
		fmt.Fprintf(os.Stderr, "warning: %s not set, manifest is unsigned\n", *keyEnv)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail("encode manifest: %v", err)
	}
	data = append(data, '\n')

	if *out == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fail("write manifest: %v", err)
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
