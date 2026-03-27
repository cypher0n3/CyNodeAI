package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// SessionStore holds persisted gateway access and refresh tokens (never in config.yaml).
type SessionStore struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Keyring hooks for tests.
var (
	sessionKeyringGetFn    = keyring.Get
	sessionKeyringSetFn    = keyring.Set
	sessionKeyringDeleteFn = keyring.Delete
)

const (
	sessionKeyringService = "cynork"
	sessionKeyringUser    = "gateway-session"
	sessionFileName       = "session.json"
)

func sessionStoreDisabled() bool {
	return os.Getenv("CYNORK_DISABLE_OS_CREDSTORE") == "1"
}

func cacheSessionFilePath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionFileName), nil
}

// PersistSession writes tokens to the OS store when possible, otherwise to the XDG cache file.
// When both access and refresh are empty, it removes stored credentials.
func PersistSession(accessToken, refreshToken string) error {
	if accessToken == "" && refreshToken == "" {
		return DeleteSession()
	}
	s := SessionStore{AccessToken: accessToken, RefreshToken: refreshToken}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	payload := string(data)

	if !sessionStoreDisabled() {
		if err := sessionKeyringSetFn(sessionKeyringService, sessionKeyringUser, payload); err == nil {
			_ = removeSessionFile()
			return nil
		}
	}
	return writeSessionFile(payload)
}

// DeleteSession removes credentials from the OS store and the cache file.
func DeleteSession() error {
	if !sessionStoreDisabled() {
		_ = sessionKeyringDeleteFn(sessionKeyringService, sessionKeyringUser)
	}
	return removeSessionFile()
}

// LoadSession reads the session from the OS store or cache file.
func LoadSession() (SessionStore, error) {
	if !sessionStoreDisabled() {
		payload, err := sessionKeyringGetFn(sessionKeyringService, sessionKeyringUser)
		if err == nil && payload != "" {
			var s SessionStore
			if err := json.Unmarshal([]byte(payload), &s); err != nil {
				return SessionStore{}, fmt.Errorf("parse keyring session: %w", err)
			}
			return s, nil
		}
	}
	path, err := cacheSessionFilePath()
	if err != nil {
		return SessionStore{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionStore{}, nil
		}
		return SessionStore{}, fmt.Errorf("read session file: %w", err)
	}
	var s SessionStore
	if err := json.Unmarshal(raw, &s); err != nil {
		return SessionStore{}, fmt.Errorf("parse session file: %w", err)
	}
	return s, nil
}

// ApplySessionStore fills cfg.Token and cfg.RefreshToken from the store when those fields are empty.
func ApplySessionStore(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if cfg.Token != "" && cfg.RefreshToken != "" {
		return nil
	}
	s, err := LoadSession()
	if err != nil {
		return err
	}
	if cfg.Token == "" {
		cfg.Token = s.AccessToken
	}
	if cfg.RefreshToken == "" {
		cfg.RefreshToken = s.RefreshToken
	}
	return nil
}

func writeSessionFile(payload string) error {
	path, err := cacheSessionFilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("session cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".session.*.tmp")
	if err != nil {
		return fmt.Errorf("session temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.WriteString(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write session temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename session file: %w", err)
	}
	return nil
}

func removeSessionFile() error {
	path, err := cacheSessionFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}
