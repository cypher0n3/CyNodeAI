package natsjwt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "natsjwt-seeds")
	if err != nil {
		panic(err)
	}
	s, err := GenerateRandomDevSeeds()
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
	_ = os.Setenv(EnvDevSeedsFile, path)
	ResetDevSeedsCache()
	code := m.Run()
	_ = os.RemoveAll(dir)
	_ = os.Unsetenv(EnvDevSeedsFile)
	ResetDevSeedsCache()
	os.Exit(code)
}
