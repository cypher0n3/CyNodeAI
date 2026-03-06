package securestore

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Default: simulate known non-FIPS environment so tests that use env fallback succeed on all platforms.
	testFIPSModeKnownOff = true
	code := m.Run()
	testFIPSModeKnownOff = false
	os.Exit(code)
}

func validMasterKeyB64() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestOpen_EnvMasterKey(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, source, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected store")
	}
	if source != MasterKeySourceEnvB64 {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestOpen_InvalidMasterKey(t *testing.T) {
	t.Setenv(masterKeyEnvName, "bad")
	_, _, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail for invalid key")
	}
	if !strings.Contains(err.Error(), ErrMasterKeyInvalid.Error()) {
		t.Fatalf("expected invalid master key error, got: %v", err)
	}
}

func TestOpen_NoMasterKey(t *testing.T) {
	t.Setenv(masterKeyEnvName, "")
	_, _, err := Open(t.TempDir())
	if !errors.Is(err, ErrMasterKeyNotConfigured) {
		t.Fatalf("expected ErrMasterKeyNotConfigured, got: %v", err)
	}
}

func TestOpen_FIPSModeRejectsEnvFallback(t *testing.T) {
	// Override TestMain: simulate FIPS on so env fallback is rejected.
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	fipsFile := filepath.Join(t.TempDir(), "fips_enabled")
	if err := os.WriteFile(fipsFile, []byte("1"), 0o644); err != nil {
		t.Fatalf("write fips flag file: %v", err)
	}
	prev := testFIPSPath
	testFIPSPath = fipsFile
	defer func() { testFIPSPath = prev }()

	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected Open to fail in FIPS mode with env fallback")
	}
	if !errors.Is(err, ErrFIPSRequiresNonEnvKey) {
		t.Fatalf("expected ErrFIPSRequiresNonEnvKey, got: %v", err)
	}
}

func expectOpenFailsWithFIPSEnvKey(t *testing.T, fipsEnvVal, failMsg string) {
	t.Helper()
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(envFIPSMode, fipsEnvVal)
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open(t.TempDir())
	if err == nil {
		t.Fatal(failMsg)
	}
	if !errors.Is(err, ErrFIPSRequiresNonEnvKey) {
		t.Fatalf("expected ErrFIPSRequiresNonEnvKey, got: %v", err)
	}
}

func TestOpen_FIPSModeEnvOverride_ExplicitOnRejectsEnv(t *testing.T) {
	expectOpenFailsWithFIPSEnvKey(t, "1", "expected Open to fail when CYNODE_FIPS_MODE=1 and env key used")
}

func TestOpen_FIPSModeEnvOverride_ExplicitOffAllowsEnv(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(envFIPSMode, "0")
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("CYNODE_FIPS_MODE=0 should allow env fallback: %v", err)
	}
}

func TestOpen_FIPSModeUnknownFailClosed(t *testing.T) {
	// No testFIPSPath, testFIPSModeKnownOff false, invalid CYNODE_FIPS_MODE → unknown → fail closed.
	expectOpenFailsWithFIPSEnvKey(t, "invalid", "expected Open to fail when FIPS status unknown (fail closed)")
}

func TestOpen_FIPSModeFileSaysOffAllowsEnv(t *testing.T) {
	testFIPSModeKnownOff = false
	defer func() { testFIPSModeKnownOff = true }()
	fipsFile := filepath.Join(t.TempDir(), "fips_off")
	if err := os.WriteFile(fipsFile, []byte("0"), 0o644); err != nil {
		t.Fatalf("write fips file: %v", err)
	}
	prev := testFIPSPath
	testFIPSPath = fipsFile
	defer func() { testFIPSPath = prev }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, source, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("FIPS file 0 should allow env fallback: %v", err)
	}
	if store == nil || source != MasterKeySourceEnvB64 {
		t.Fatalf("expected env_b64 source, got %q", source)
	}
}

func TestMasterKeyPrecedence_TPMAndOSKeyStoreNotConfigured(t *testing.T) {
	// TPM and OS key store are stubs and return not configured; env is used when set.
	_, err := loadMasterKeyFromTPM()
	if !errors.Is(err, ErrMasterKeyNotConfigured) {
		t.Errorf("loadMasterKeyFromTPM: want ErrMasterKeyNotConfigured, got %v", err)
	}
	_, err = loadMasterKeyFromOSKeyStore()
	if !errors.Is(err, ErrMasterKeyNotConfigured) {
		t.Errorf("loadMasterKeyFromOSKeyStore: want ErrMasterKeyNotConfigured, got %v", err)
	}
}

func TestOpen_SystemCredentialPreferred(t *testing.T) {
	credDir := t.TempDir()
	keyFile := filepath.Join(credDir, systemCredentialMasterKeyFile)
	if err := os.WriteFile(keyFile, []byte(validMasterKeyB64()), 0o600); err != nil {
		t.Fatalf("write system credential key file: %v", err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)
	t.Setenv(masterKeyEnvName, "bad")
	_, source, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if source != MasterKeySourceSystemCredential {
		t.Fatalf("expected system credential source, got: %q", source)
	}
}

func TestLoadMasterKeyFromSystemCredential_ReadError(t *testing.T) {
	credDir := t.TempDir()
	// Create directory where file is expected, forcing ReadFile error.
	if err := os.Mkdir(filepath.Join(credDir, systemCredentialMasterKeyFile), 0o700); err != nil {
		t.Fatalf("mkdir colliding credential path: %v", err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)
	if _, err := loadMasterKeyFromSystemCredential(); err == nil {
		t.Fatal("expected read error for system credential path directory")
	}
}

func TestLoadMasterKeyFromSystemCredential_InvalidContent(t *testing.T) {
	credDir := t.TempDir()
	keyFile := filepath.Join(credDir, systemCredentialMasterKeyFile)
	if err := os.WriteFile(keyFile, []byte("not-base64"), 0o600); err != nil {
		t.Fatalf("write invalid key: %v", err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)
	if _, err := loadMasterKeyFromSystemCredential(); err == nil {
		t.Fatal("expected invalid content to fail master key decode")
	}
}

func TestPutGetDeleteAgentToken(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.PutAgentToken("pma-main", "tok-1", ""); err != nil {
		t.Fatalf("PutAgentToken failed: %v", err)
	}
	record, err := store.GetAgentToken("pma-main")
	if err != nil {
		t.Fatalf("GetAgentToken failed: %v", err)
	}
	if record.Token != "tok-1" || record.ServiceID != "pma-main" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if err := store.DeleteAgentToken("pma-main"); err != nil {
		t.Fatalf("DeleteAgentToken failed: %v", err)
	}
	if _, err := store.GetAgentToken("pma-main"); err == nil {
		t.Fatal("expected GetAgentToken to fail after delete")
	}
}

func TestPutAgentToken_EncryptedAtRest(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.PutAgentToken("svc-1", "super-secret-token", ""); err != nil {
		t.Fatalf("PutAgentToken failed: %v", err)
	}
	path := filepath.Join(stateDir, "secrets", "agent_tokens", "svc-1"+agentTokenEncryptedFileSuffix)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted token file: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-token") {
		t.Fatal("token leaked in ciphertext file")
	}
}

func TestGetAgentToken_Expired(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	expired := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if err := store.PutAgentToken("svc-expired", "tok-exp", expired); err != nil {
		t.Fatalf("PutAgentToken failed: %v", err)
	}
	_, err = store.GetAgentToken("svc-expired")
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestListAgentTokenServiceIDs(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_ = store.PutAgentToken("svc-a", "tok-a", "")
	_ = store.PutAgentToken("svc-b", "tok-b", "")
	ids, err := store.ListAgentTokenServiceIDs()
	if err != nil {
		t.Fatalf("ListAgentTokenServiceIDs failed: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d (%+v)", len(ids), ids)
	}
}

func TestPutAgentToken_InvalidInputs(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.PutAgentToken("../escape", "tok", ""); err == nil {
		t.Fatal("expected invalid service_id error")
	}
	if err := store.PutAgentToken("svc-a", "", ""); err == nil {
		t.Fatal("expected missing token error")
	}
	if err := store.PutAgentToken("svc-a", "tok", "not-a-time"); err == nil {
		t.Fatal("expected invalid expires_at error")
	}
}

func TestGetAgentToken_EnvelopeDecodeFailures(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatalf("mkdir token dir: %v", err)
	}
	path := filepath.Join(tokenDir, "svc-a"+agentTokenEncryptedFileSuffix)
	for _, tt := range []struct {
		name     string
		envelope string
	}{
		{"invalid b64 fields", `{"version":1,"algorithm":"AES-256-GCM","nonce_b64":"bad","payload_b64":"bad"}`},
		{"unsupported version", `{"version":2,"algorithm":"AES-256-GCM","nonce_b64":"AA==","payload_b64":"AA=="}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.envelope), 0o600); err != nil {
				t.Fatalf("write envelope: %v", err)
			}
			if _, err := store.GetAgentToken("svc-a"); err == nil {
				t.Fatal("expected GetAgentToken to fail")
			}
		})
	}
}

func TestDeleteAgentToken_Missing(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.DeleteAgentToken("svc-missing"); err != nil {
		t.Fatalf("DeleteAgentToken should ignore missing file: %v", err)
	}
}

func TestDecodeMasterKey_LengthValidation(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := decodeMasterKey(short); err == nil {
		t.Fatal("expected decodeMasterKey to fail for short key")
	}
}

func TestLoadMasterKeyFromSystemCredential_NotConfigured(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", "")
	if _, err := loadMasterKeyFromSystemCredential(); !errors.Is(err, ErrMasterKeyNotConfigured) {
		t.Fatalf("expected ErrMasterKeyNotConfigured, got: %v", err)
	}
}

func TestLoadMasterKeyFromSystemCredential_NotExist(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", t.TempDir())
	if _, err := loadMasterKeyFromSystemCredential(); !errors.Is(err, ErrMasterKeyNotConfigured) {
		t.Fatalf("expected ErrMasterKeyNotConfigured, got: %v", err)
	}
}

func TestListAgentTokenServiceIDs_NoDir(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	ids, err := store.ListAgentTokenServiceIDs()
	if err != nil {
		t.Fatalf("ListAgentTokenServiceIDs failed: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no ids, got: %+v", ids)
	}
}

func TestListAgentTokenServiceIDs_IgnoresInvalidFiles(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	dir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad/evil"+agentTokenEncryptedFileSuffix), []byte("x"), 0o600); err == nil {
		t.Fatal("expected invalid path write to fail")
	}
	if err := os.WriteFile(filepath.Join(dir, ".."+agentTokenEncryptedFileSuffix), []byte("x"), 0o600); err != nil {
		t.Fatalf("write odd filename: %v", err)
	}
	ids, err := store.ListAgentTokenServiceIDs()
	if err != nil {
		t.Fatalf("list ids: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected invalid token filenames to be ignored, got: %+v", ids)
	}
}

func TestEncryptDecrypt_RejectsBadKey(t *testing.T) {
	if _, _, err := encrypt([]byte("x"), []byte("short")); err == nil {
		t.Fatal("expected encrypt to fail with bad key")
	}
	if _, err := decrypt([]byte("x"), []byte("nonce"), []byte("short")); err == nil {
		t.Fatal("expected decrypt to fail with bad key")
	}
}

func TestOpen_EmptyStateDirUsesDefaultPathAndFailsInUnprivilegedEnv(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	_, _, err := Open("")
	if err == nil {
		t.Fatal("expected Open(\"\") to fail in unprivileged test env")
	}
}

func TestPutAgentToken_StoreDirCreateFailure(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	parent := t.TempDir()
	// Force tokenDir path collision: <root>/agent_tokens is a file, not a directory.
	root := filepath.Join(parent, "secrets")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent_tokens"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write colliding file: %v", err)
	}
	store := &Store{rootDir: root, key: []byte("0123456789abcdef0123456789abcdef")}
	if err := store.PutAgentToken("svc-a", "tok", ""); err == nil {
		t.Fatal("expected PutAgentToken to fail when token dir cannot be created")
	}
}

func TestPutAgentToken_EncryptFailure(t *testing.T) {
	store := &Store{rootDir: t.TempDir(), key: []byte("short")}
	if err := store.PutAgentToken("svc-a", "tok", ""); err == nil {
		t.Fatal("expected PutAgentToken to fail with invalid key length")
	}
}

func TestGetAgentToken_PathValidationFailure(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if _, err := store.GetAgentToken("../bad"); err == nil {
		t.Fatal("expected invalid service_id path to fail")
	}
}

func TestDeleteAgentToken_PathValidationFailure(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	store, _, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := store.DeleteAgentToken("../bad"); err == nil {
		t.Fatal("expected invalid service_id path to fail")
	}
}

// Note: unsupported/invalid envelope cases are covered by TestGetAgentToken_EnvelopeDecodeFailures.

func TestGetAgentToken_InvalidStoredExpiry(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	record := []byte(`{"service_id":"svc-a","token":"tok","expires_at":"invalid","written_at":"2026-03-06T00:00:00Z"}`)
	ciphertext, nonce, err := encrypt(record, store.key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	envelope := `{"version":1,"algorithm":"AES-256-GCM","nonce_b64":"` +
		base64.StdEncoding.EncodeToString(nonce) +
		`","payload_b64":"` + base64.StdEncoding.EncodeToString(ciphertext) + `"}`
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatalf("mkdir token dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "svc-a"+agentTokenEncryptedFileSuffix), []byte(envelope), 0o600); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
	if _, err := store.GetAgentToken("svc-a"); err == nil {
		t.Fatal("expected invalid stored expiry to fail")
	}
}

func TestDeleteAgentToken_RemoveDirectoryFailure(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	tokenPath := filepath.Join(tokenDir, "svc-a"+agentTokenEncryptedFileSuffix)
	if err := os.MkdirAll(tokenPath, 0o700); err != nil {
		t.Fatalf("mkdir colliding token path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tokenPath, "nested"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	if err := store.DeleteAgentToken("svc-a"); err == nil {
		t.Fatal("expected delete failure when token path is a directory")
	}
}

func TestListAgentTokenServiceIDs_ReadDirError(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	tokenPath := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.WriteFile(tokenPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write token path file: %v", err)
	}
	if _, err := store.ListAgentTokenServiceIDs(); err == nil {
		t.Fatal("expected readDir error when token path is a file")
	}
}

type failingReader struct{}

func (f failingReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestEncrypt_NonceReaderFailure(t *testing.T) {
	orig := cryptorand.Reader
	cryptorand.Reader = failingReader{}
	defer func() { cryptorand.Reader = orig }()
	if _, _, err := encrypt([]byte("plaintext"), []byte("0123456789abcdef0123456789abcdef")); err == nil {
		t.Fatal("expected encrypt to fail when nonce read fails")
	}
}

func TestSanitizeServiceID_Empty(t *testing.T) {
	if _, err := sanitizeServiceID(""); err == nil {
		t.Fatal("expected empty service_id to fail")
	}
}

func TestDecrypt_PayloadFailure(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	nonce := []byte("0123456789ab") // 12 bytes
	if _, err := decrypt([]byte("bad-ciphertext"), nonce, key); err == nil {
		t.Fatal("expected decrypt payload failure")
	}
}

func TestListAgentTokenServiceIDs_IgnoresDirectoryEntries(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(filepath.Join(tokenDir, "nested-dir"), 0o700); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	ids, err := store.ListAgentTokenServiceIDs()
	if err != nil {
		t.Fatalf("list ids: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected directories to be ignored, got ids: %+v", ids)
	}
}
