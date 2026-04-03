package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

func init() {
	dir, err := os.MkdirTemp("", "ugw-nats-seeds")
	if err != nil {
		panic(err)
	}
	s, err := natsjwt.GenerateRandomDevSeeds()
	if err != nil {
		panic(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "seeds.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		panic(err)
	}
	_ = os.Setenv(natsjwt.EnvDevSeedsFile, path)
	natsjwt.ResetDevSeedsCache()
	// Avoid a host NATS on 4222: its resolver JWTs will not match these test-generated seeds (Authorization Violation).
	if os.Getenv("NATS_CLIENT_URL") == "" {
		_ = os.Setenv("NATS_CLIENT_URL", "nats://127.0.0.1:43999")
	}
	if os.Getenv("NATS_WEBSOCKET_URL") == "" {
		_ = os.Setenv("NATS_WEBSOCKET_URL", "ws://127.0.0.1:43998/nats")
	}
}
