package bdd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

func init() {
	dir, err := os.MkdirTemp("", "bdd-nats-seeds")
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
}
