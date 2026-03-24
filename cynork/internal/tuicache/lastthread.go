// Package tuicache stores TUI-only cache data (not config) under the XDG cache dir.
package tuicache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/config"
)

// lastThreadsFileName is a single flat JSON document in the cache root (no subdirs).
const lastThreadsFileName = "last_threads.json"

// lastThreadsDoc is the on-disk JSON shape: one object mapping composite keys to thread ids.
type lastThreadsDoc struct {
	Threads map[string]string `json:"threads"`
}

// Root returns the base cache directory for TUI cache files.
// If CYNORK_CACHE_DIR is set, it is used instead of config.CacheDir().
func Root() (string, error) {
	if v := os.Getenv("CYNORK_CACHE_DIR"); v != "" {
		return filepath.Clean(v), nil
	}
	return config.CacheDir()
}

// ReadLastThread returns the cached thread id for the given gateway URL, user id, and project id.
// The second return is false if there is no entry.
func ReadLastThread(cacheRoot, gatewayURL, userID, projectID string) (threadID string, found bool, err error) {
	if cacheRoot == "" || userID == "" {
		return "", false, nil
	}
	path := filepath.Join(cacheRoot, lastThreadsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	var doc lastThreadsDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.Threads == nil {
		return "", false, nil
	}
	key := compositeKey(gatewayURL, userID, projectID)
	tid, ok := doc.Threads[key]
	if !ok || tid == "" {
		return "", false, nil
	}
	return tid, true, nil
}

// WriteLastThread writes the last active thread id for gateway + user + project. Best-effort; ignores empty threadID.
func WriteLastThread(cacheRoot, gatewayURL, userID, projectID, threadID string) error {
	if cacheRoot == "" || userID == "" || threadID == "" {
		return nil
	}
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	path := filepath.Join(cacheRoot, lastThreadsFileName)
	key := compositeKey(gatewayURL, userID, projectID)

	doc := lastThreadsDoc{Threads: map[string]string{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &doc)
		if doc.Threads == nil {
			doc.Threads = map[string]string{}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read cache: %w", err)
	}
	doc.Threads[key] = threadID

	payload, err := lastThreadMarshalIndent(&doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal last threads: %w", err)
	}
	dir := cacheRoot
	tmp, err := lastThreadCreateTemp(dir, ".last_threads.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := lastThreadOsChmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := lastThreadOsRename(tmpPath, path); err != nil {
		return fmt.Errorf("rename cache file: %w", err)
	}
	return nil
}

// Swapped in tests to exercise error paths (same package only).
var (
	lastThreadCreateTemp    = os.CreateTemp
	lastThreadMarshalIndent = json.MarshalIndent
	lastThreadOsChmod       = os.Chmod
	lastThreadOsRename      = os.Rename
)

func compositeKey(gatewayURL, userID, projectID string) string {
	s := normalizeGatewayURL(gatewayURL) + "\n" + userID + "\n" + projectID
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func normalizeGatewayURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, "/")
	return u
}
