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

func TestLoadDefaultSkill_missingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadDefaultSkill(dir)
	if err != nil {
		t.Fatalf("LoadDefaultSkill() err = %v", err)
	}
	if got != "" {
		t.Errorf("LoadDefaultSkill() = %q, want empty", got)
	}
}

func TestLoadDefaultSkill_readsFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, DefaultSkillFilename)
	const body = "Default skill content."
	if err := os.WriteFile(fpath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDefaultSkill(dir)
	if err != nil {
		t.Fatalf("LoadDefaultSkill() err = %v", err)
	}
	if got != body {
		t.Errorf("LoadDefaultSkill() = %q, want %q", got, body)
	}
}

func TestLoadInstructions_nestedSubdirMd(t *testing.T) {
	root := t.TempDir()
	// loadEntry only recurses into subdirectories whose names have a .md or .txt extension.
	sub := filepath.Join(root, "bundle.md")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inner.md"), []byte("nested body"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInstructions(root)
	if err != nil {
		t.Fatalf("LoadInstructions: %v", err)
	}
	if !strings.Contains(got, "nested body") {
		t.Errorf("LoadInstructions = %q", got)
	}
}

func TestLoadInstructions_readFileDenied(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "blocked.md")
	if err := os.WriteFile(p, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o600) }()
	_, err := LoadInstructions(dir)
	if err == nil {
		t.Fatal("expected error when file is not readable")
	}
}

func TestLoadDefaultSkill_readDenied(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, DefaultSkillFilename)
	if err := os.WriteFile(p, []byte("skill"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o600) }()
	_, err := LoadDefaultSkill(dir)
	if err == nil {
		t.Fatal("expected error when skill file is not readable")
	}
}
