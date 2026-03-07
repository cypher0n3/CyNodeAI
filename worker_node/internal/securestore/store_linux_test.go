//go:build linux

package securestore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestGetFIPSStatusLinux_FileMissing covers getFIPSStatusLinux when the path does not exist (fipsOff).
func TestGetFIPSStatusLinux_FileMissing(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	prevPath := testFIPSPath
	testFIPSPath = ""
	defer func() { testFIPSPath = prevPath }()
	_ = os.Unsetenv(envFIPSMode)
	defer func() { _ = os.Unsetenv(envFIPSMode) }()
	fipsEnabledPathOverride = filepath.Join(t.TempDir(), "nonexistent")
	defer func() { fipsEnabledPathOverride = "" }()

	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("expected Open to succeed when FIPS file missing: %v", err)
	}
	if store == nil {
		t.Fatal("expected store")
	}
}

// TestGetFIPSStatusLinux_FileSaysOn covers getFIPSStatusLinux when file content is "1" (fipsOn).
func TestGetFIPSStatusLinux_FileSaysOn(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	prevPath := testFIPSPath
	testFIPSPath = ""
	defer func() { testFIPSPath = prevPath }()
	_ = os.Unsetenv(envFIPSMode)
	defer func() { _ = os.Unsetenv(envFIPSMode) }()

	fipsFile := filepath.Join(t.TempDir(), "fips")
	if err := os.WriteFile(fipsFile, []byte("1"), 0o644); err != nil {
		t.Fatalf("write fips file: %v", err)
	}
	fipsEnabledPathOverride = fipsFile
	defer func() { fipsEnabledPathOverride = "" }()

	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail when FIPS file says 1 and env key used")
	}
	if !errors.Is(err, ErrFIPSRequiresNonEnvKey) {
		t.Errorf("expected ErrFIPSRequiresNonEnvKey, got: %v", err)
	}
}

// TestGetFIPSStatusLinux_FileSaysOff covers getFIPSStatusLinux when file content is not "1" (fipsOff).
func TestGetFIPSStatusLinux_FileSaysOff(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	prevPath := testFIPSPath
	testFIPSPath = ""
	defer func() { testFIPSPath = prevPath }()
	_ = os.Unsetenv(envFIPSMode)
	defer func() { _ = os.Unsetenv(envFIPSMode) }()

	fipsFile := filepath.Join(t.TempDir(), "fips")
	if err := os.WriteFile(fipsFile, []byte("0"), 0o644); err != nil {
		t.Fatalf("write fips file: %v", err)
	}
	fipsEnabledPathOverride = fipsFile
	defer func() { fipsEnabledPathOverride = "" }()

	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("expected Open to succeed when FIPS file says 0: %v", err)
	}
	if store == nil {
		t.Fatal("expected store")
	}
}

// TestGetFIPSStatusLinux_ReadError covers getFIPSStatusLinux when ReadFile fails with non-IsNotExist (fipsUnknown).
func TestGetFIPSStatusLinux_ReadError(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	prevPath := testFIPSPath
	testFIPSPath = ""
	defer func() { testFIPSPath = prevPath }()
	_ = os.Unsetenv(envFIPSMode)
	defer func() { _ = os.Unsetenv(envFIPSMode) }()

	// Path is a directory so ReadFile fails with a non-IsNotExist error → fipsUnknown → fail closed.
	dir := t.TempDir()
	fipsEnabledPathOverride = dir
	defer func() { fipsEnabledPathOverride = "" }()

	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail when FIPS status unknown (read error)")
	}
}

