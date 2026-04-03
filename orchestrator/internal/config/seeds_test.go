package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "config-nats-seeds")
	if err != nil {
		panic(err)
	}
	s, err := natsjwt.GenerateRandomDevSeeds()
	if err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "seeds.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		panic(err)
	}
	_ = os.Setenv(natsjwt.EnvDevSeedsFile, path)
	natsjwt.ResetDevSeedsCache()
	code := m.Run()
	_ = os.RemoveAll(dir)
	_ = os.Unsetenv(natsjwt.EnvDevSeedsFile)
	natsjwt.ResetDevSeedsCache()
	os.Exit(code)
}
