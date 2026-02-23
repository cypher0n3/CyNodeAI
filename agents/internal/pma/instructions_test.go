package pma

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInstructions_missingDirReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadInstructions(filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("LoadInstructions() err = %v", err)
	}
	if got != "" {
		t.Errorf("LoadInstructions() = %q, want empty", got)
	}
}

func TestLoadInstructions_emptyDirReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "empty")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInstructions(sub)
	if err != nil {
		t.Fatalf("LoadInstructions() err = %v", err)
	}
	if got != "" {
		t.Errorf("LoadInstructions() = %q, want empty", got)
	}
}

func TestLoadInstructions_readsMdAndTxtFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "bundle")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("content a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("content b"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInstructions(sub)
	if err != nil {
		t.Fatalf("LoadInstructions() err = %v", err)
	}
	if !strings.Contains(got, "content a") || !strings.Contains(got, "content b") {
		t.Errorf("LoadInstructions() = %q, want to contain content a and b", got)
	}
}

func TestLoadInstructions_filePathReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.md")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInstructions(f)
	if err != nil {
		t.Fatalf("LoadInstructions() err = %v", err)
	}
	if got != "" {
		t.Errorf("LoadInstructions(file path) = %q, want empty", got)
	}
}

func TestLoadInstructions_skipsNonMdTxt(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "skip")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "x.yaml"), []byte("yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInstructions(sub)
	if err != nil {
		t.Fatalf("LoadInstructions() err = %v", err)
	}
	if got != "" {
		t.Errorf("LoadInstructions() = %q, want empty (yaml skipped)", got)
	}
}
