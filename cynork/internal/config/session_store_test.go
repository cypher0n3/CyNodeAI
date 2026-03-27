package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestPersistSessionLoadDelete_FileFallback(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	if err := PersistSession("acc", "ref"); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	var c Config
	if err := ApplySessionStore(&c); err != nil {
		t.Fatalf("ApplySessionStore: %v", err)
	}
	if c.Token != "acc" || c.RefreshToken != "ref" {
		t.Fatalf("tokens: access=%q refresh=%q", c.Token, c.RefreshToken)
	}

	c2 := Config{Token: "env-only"}
	if err := ApplySessionStore(&c2); err != nil {
		t.Fatalf("ApplySessionStore partial: %v", err)
	}
	if c2.Token != "env-only" || c2.RefreshToken != "ref" {
		t.Fatalf("partial fill: access=%q refresh=%q", c2.Token, c2.RefreshToken)
	}

	if err := DeleteSession(); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	var c3 Config
	if err := ApplySessionStore(&c3); err != nil {
		t.Fatalf("ApplySessionStore after delete: %v", err)
	}
	if c3.Token != "" || c3.RefreshToken != "" {
		t.Fatalf("expected empty after delete: %+v", c3)
	}
}

func TestPersistSessionDelete_EmptyClearsFile(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := PersistSession("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := PersistSession("", ""); err != nil {
		t.Fatalf("PersistSession empty: %v", err)
	}
	path, err := cacheSessionFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file should be removed: stat=%v", err)
	}
}

func TestPersistSession_OnlyAccessToken(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := PersistSession("only-access", ""); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessToken != "only-access" || s.RefreshToken != "" {
		t.Fatalf("session=%+v", s)
	}
}

func TestLoadSession_InvalidSessionFile(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	dir := filepath.Join(base, "cynork")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestApplySessionStore_NilAndFull(t *testing.T) {
	if err := ApplySessionStore(nil); err != nil {
		t.Fatal(err)
	}
	full := Config{Token: "a", RefreshToken: "b"}
	if err := ApplySessionStore(&full); err != nil {
		t.Fatal(err)
	}
	if full.Token != "a" || full.RefreshToken != "b" {
		t.Fatal("mutated")
	}
}

func TestDeleteSession_Idempotent(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := DeleteSession(); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionKeyring_PersistLoadDelete(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "0")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	var stored string
	oldG, oldS, oldD := sessionKeyringGetFn, sessionKeyringSetFn, sessionKeyringDeleteFn
	defer func() { sessionKeyringGetFn, sessionKeyringSetFn, sessionKeyringDeleteFn = oldG, oldS, oldD }()
	sessionKeyringSetFn = func(_, _ string, pass string) error {
		stored = pass
		return nil
	}
	sessionKeyringGetFn = func(_, _ string) (string, error) {
		if stored == "" {
			return "", keyring.ErrNotFound
		}
		return stored, nil
	}
	sessionKeyringDeleteFn = func(_, _ string) error {
		stored = ""
		return nil
	}

	if err := PersistSession("aa", "bb"); err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := ApplySessionStore(&c); err != nil {
		t.Fatal(err)
	}
	if c.Token != "aa" || c.RefreshToken != "bb" {
		t.Fatalf("tokens %q %q", c.Token, c.RefreshToken)
	}
	if err := DeleteSession(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSession_KeyringInvalidJSON(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "0")
	oldG := sessionKeyringGetFn
	defer func() { sessionKeyringGetFn = oldG }()
	sessionKeyringGetFn = func(_, _ string) (string, error) {
		return "not-json", nil
	}
	if _, err := LoadSession(); err == nil {
		t.Fatal("expected error")
	}
}

func TestPersistSession_SessionPathBlockedByDirectory(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "session.json")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := PersistSession("a", "b"); err == nil {
		t.Fatal("expected error when session path is a directory")
	}
}

func persistSessionFailsWhenCacheDirUnwritable(t *testing.T, mode os.FileMode, wantMsg string) {
	t.Helper()
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()
	if err := PersistSession("a", "b"); err == nil {
		t.Fatal(wantMsg)
	}
}

func TestPersistSession_CreateTempFailsReadOnlyCacheDir(t *testing.T) {
	persistSessionFailsWhenCacheDirUnwritable(t, 0o500, "expected error when cache dir is read-only for writes")
}

func TestPersistSession_CacheDirNotWritable(t *testing.T) {
	persistSessionFailsWhenCacheDirUnwritable(t, 0, "expected error when cache dir is not writable")
}

func TestLoadSession_CacheDirUnavailable(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	t.Setenv("XDG_CACHE_HOME", "")
	restore := SetUserHomeDirForTest(func() (string, error) {
		return "", errors.New("no home")
	})
	defer restore()
	if _, err := LoadSession(); err == nil {
		t.Fatal("expected error when cache dir cannot be resolved")
	}
}

func TestDeleteSession_FileRemoveWhenCacheDirUnavailableAfterPersist(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := PersistSession("a", "b"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	restore := SetUserHomeDirForTest(func() (string, error) {
		return "", errors.New("no home")
	})
	defer restore()
	if err := DeleteSession(); err == nil {
		t.Fatal("expected error removing session when cache path cannot be resolved")
	}
}

func TestLoadSession_KeyringEmptyPayloadUsesFile(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "0")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	oldG, oldS := sessionKeyringGetFn, sessionKeyringSetFn
	defer func() { sessionKeyringGetFn, sessionKeyringSetFn = oldG, oldS }()
	sessionKeyringSetFn = func(_, _, _ string) error {
		return errors.New("stub: keyring unavailable")
	}
	if err := PersistSession("ef", "rf"); err != nil {
		t.Fatal(err)
	}
	sessionKeyringGetFn = func(_, _ string) (string, error) {
		return "", nil
	}
	s, err := LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessToken != "ef" || s.RefreshToken != "rf" {
		t.Fatalf("session=%+v", s)
	}
}

func TestLoadSession_KeyringGenericErrorFallsBackToFile(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "0")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	oldG, oldS := sessionKeyringGetFn, sessionKeyringSetFn
	defer func() { sessionKeyringGetFn, sessionKeyringSetFn = oldG, oldS }()
	sessionKeyringSetFn = func(_, _, _ string) error {
		return errors.New("stub: keyring unavailable")
	}
	if err := PersistSession("x", "y"); err != nil {
		t.Fatal(err)
	}
	sessionKeyringGetFn = func(_, _ string) (string, error) {
		return "", errors.New("dbus down")
	}
	s, err := LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessToken != "x" || s.RefreshToken != "y" {
		t.Fatalf("session=%+v", s)
	}
}

func TestDeleteSession_RemoveFailsOnNonEmptyDir(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "1")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", root)
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "session.json")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "nested"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(); err == nil {
		t.Fatal("expected remove error when session path is a non-empty directory")
	}
}

func TestLoadSession_KeyringNotFoundUsesFile(t *testing.T) {
	t.Setenv("CYNORK_DISABLE_OS_CREDSTORE", "0")
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))

	oldG, oldS := sessionKeyringGetFn, sessionKeyringSetFn
	defer func() { sessionKeyringGetFn, sessionKeyringSetFn = oldG, oldS }()
	sessionKeyringGetFn = func(_, _ string) (string, error) {
		return "", keyring.ErrNotFound
	}
	sessionKeyringSetFn = func(_, _, _ string) error {
		return errors.New("stub: keyring unavailable")
	}

	if err := PersistSession("f1", "f2"); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSession()
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessToken != "f1" || s.RefreshToken != "f2" {
		t.Fatalf("session=%+v", s)
	}
}
