// Package pma provides configuration and runtime for the cynode-pma agent binary.
package pma

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadInstructions reads the role instructions bundle from dir and returns concatenated content.
// It reads all .md and .txt files in dir (and optionally one level of subdirs) in deterministic order.
// Returns empty string if dir does not exist or is not a directory.
func LoadInstructions(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, e := range entries {
		s, err := loadEntry(dir, e)
		if err != nil {
			return "", err
		}
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

func loadEntry(dir string, e os.DirEntry) (string, error) {
	name := e.Name()
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".md" && ext != ".txt" {
		return "", nil
	}
	fpath := filepath.Join(dir, name)
	if e.IsDir() {
		return LoadInstructions(fpath)
	}
	b, err := os.ReadFile(fpath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
