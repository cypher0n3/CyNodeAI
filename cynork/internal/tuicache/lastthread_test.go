package tuicache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLastThread_renameHookError(t *testing.T) {
	old := lastThreadOsRename
	lastThreadOsRename = func(_, _ string) error {
		return errors.New("injected rename")
	}
	defer func() { lastThreadOsRename = old }()
	err := WriteLastThread(t.TempDir(), "http://gw", "u", "", "tid")
	if err == nil || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteLastThread_marshalHookError(t *testing.T) {
	old := lastThreadMarshalIndent
	lastThreadMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("injected marshal")
	}
	defer func() { lastThreadMarshalIndent = old }()
	err := WriteLastThread(t.TempDir(), "http://gw", "u", "", "tid")
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteLastThread_createTempHookError(t *testing.T) {
	old := lastThreadCreateTemp
	lastThreadCreateTemp = func(_, _ string) (*os.File, error) {
		return nil, errors.New("injected create temp")
	}
	defer func() { lastThreadCreateTemp = old }()
	err := WriteLastThread(t.TempDir(), "http://gw", "u", "", "tid")
	if err == nil || !strings.Contains(err.Error(), "create temp") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteLastThread_chmodHookError(t *testing.T) {
	old := lastThreadOsChmod
	lastThreadOsChmod = func(_ string, _ os.FileMode) error {
		return errors.New("injected chmod")
	}
	defer func() { lastThreadOsChmod = old }()
	err := WriteLastThread(t.TempDir(), "http://gw", "u", "", "tid")
	if err == nil || !strings.Contains(err.Error(), "chmod") {
		t.Fatalf("got %v", err)
	}
}

func TestReadLastThread_invalidOrEmptyThreadsJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"invalid_json", "{"},
		{"threads_null", `{"threads":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, lastThreadsFileName)
			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			got, ok, err := ReadLastThread(root, "http://gw", "u", "")
			if err != nil {
				t.Fatal(err)
			}
			if ok || got != "" {
				t.Fatalf("got %q ok=%v", got, ok)
			}
		})
	}
}

func TestWriteLastThread_CachePathIsDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, lastThreadsFileName)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := WriteLastThread(root, "http://gw", "u", "", "tid")
	if err == nil {
		t.Fatal("expected error when cache path is a directory")
	}
}

func TestRoot_WithoutEnvUsesConfigCacheDir(t *testing.T) {
	t.Setenv("CYNORK_CACHE_DIR", "")
	_, err := Root()
	if err != nil {
		t.Skip(err)
	}
}

func TestReadLastThread_emptyCacheRoot(t *testing.T) {
	_, ok, err := ReadLastThread("", "http://gw", "u", "p")
	if err != nil || ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
}

func TestReadLastThread_emptyUserID(t *testing.T) {
	_, ok, err := ReadLastThread(t.TempDir(), "http://gw", "", "p")
	if err != nil || ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
}

func TestWriteLastThread_CacheRootNotADirectory(t *testing.T) {
	tmp := t.TempDir()
	badRoot := filepath.Join(tmp, "file-as-root")
	if err := os.WriteFile(badRoot, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteLastThread(badRoot, "http://gw", "u", "", "tid")
	if err == nil {
		t.Fatal("expected error when cache root path is a file")
	}
}

func TestReadLastThread_cachePathIsDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, lastThreadsFileName)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadLastThread(root, "http://gw", "u", "")
	if err == nil {
		t.Fatal("expected error when cache file path is a directory")
	}
}

func TestWriteLastThread_readExistingForbidden(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, lastThreadsFileName)
	if err := os.WriteFile(path, []byte(`{"threads":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	err := WriteLastThread(root, "http://gw", "u", "", "tid")
	if err == nil {
		t.Fatal("expected error when existing cache file is unreadable")
	}
}

func TestWriteLastThread_cacheDirNotWritable(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0o700) }()
	err := WriteLastThread(root, "http://gw", "u", "", "tid")
	if err == nil {
		t.Fatal("expected error when cache root is not writable")
	}
}

func TestReadLastThread_Missing(t *testing.T) {
	root := t.TempDir()
	got, ok, err := ReadLastThread(root, "http://gw", "user-1", "proj-a")
	if err != nil {
		t.Fatal(err)
	}
	if ok || got != "" {
		t.Fatalf("ReadLastThread = %q ok=%v", got, ok)
	}
}

func TestWriteLastThread_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := WriteLastThread(root, "http://gw/", "user-1", "proj-a", "thread-xyz"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadLastThread(root, "http://gw", "user-1", "proj-a")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != "thread-xyz" {
		t.Fatalf("ReadLastThread = %q ok=%v", got, ok)
	}
	path := filepath.Join(root, lastThreadsFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected flat file at %s: %v", path, err)
	}
}

func TestWriteLastThread_NormalizesGatewayURL(t *testing.T) {
	root := t.TempDir()
	if err := WriteLastThread(root, "http://gw/", "u", "", "t1"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadLastThread(root, "http://gw", "u", "")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != "t1" {
		t.Fatalf("ReadLastThread = %q ok=%v", got, ok)
	}
}

func TestWriteLastThread_MergesMultipleKeys(t *testing.T) {
	root := t.TempDir()
	if err := WriteLastThread(root, "http://gw", "u1", "p1", "t1"); err != nil {
		t.Fatal(err)
	}
	if err := WriteLastThread(root, "http://gw", "u2", "p1", "t2"); err != nil {
		t.Fatal(err)
	}
	got1, ok1, err := ReadLastThread(root, "http://gw", "u1", "p1")
	if err != nil || !ok1 || got1 != "t1" {
		t.Fatalf("u1: %q ok=%v err=%v", got1, ok1, err)
	}
	got2, ok2, err := ReadLastThread(root, "http://gw", "u2", "p1")
	if err != nil || !ok2 || got2 != "t2" {
		t.Fatalf("u2: %q ok=%v err=%v", got2, ok2, err)
	}
}

func TestWriteLastThread_DifferentUserDifferentFile(t *testing.T) {
	root := t.TempDir()
	if err := WriteLastThread(root, "http://gw", "u1", "", "t1"); err != nil {
		t.Fatal(err)
	}
	if err := WriteLastThread(root, "http://gw", "u2", "", "t2"); err != nil {
		t.Fatal(err)
	}
	got1, ok1, err := ReadLastThread(root, "http://gw", "u1", "")
	if err != nil {
		t.Fatal(err)
	}
	got2, ok2, err := ReadLastThread(root, "http://gw", "u2", "")
	if err != nil {
		t.Fatal(err)
	}
	if !ok1 || got1 != "t1" || !ok2 || got2 != "t2" {
		t.Fatalf("got %q %q ok %v %v", got1, got2, ok1, ok2)
	}
}

func TestRoot_UsesCYNORK_CACHE_DIR(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CYNORK_CACHE_DIR", tmp)
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	if root != filepath.Clean(tmp) {
		t.Fatalf("Root = %q want %q", root, tmp)
	}
}

func TestWriteLastThread_EmptyThreadIDNoOp(t *testing.T) {
	root := t.TempDir()
	if err := WriteLastThread(root, "http://gw", "u", "", ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, lastThreadsFileName)
	_, statErr := os.Stat(path)
	if statErr == nil {
		t.Fatal("expected no last_threads.json when thread id empty")
	}
	if !os.IsNotExist(statErr) {
		t.Fatal(statErr)
	}
}
