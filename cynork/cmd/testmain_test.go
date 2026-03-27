package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cynork-cmd-test-*")
	if err != nil {
		panic(err)
	}
	cacheRoot := filepath.Join(dir, "cache")
	_ = os.MkdirAll(cacheRoot, 0o700)
	_ = os.Setenv("XDG_CACHE_HOME", cacheRoot)
	_ = os.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
