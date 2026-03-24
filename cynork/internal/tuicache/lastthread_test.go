package tuicache

import (
	"os"
	"path/filepath"
	"testing"
)

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
