package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/mlkem"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadKEMKeyFromV1LegacyFile asserts a v1 (no-AAD) kem keystore file still loads; next persist uses v3+AAD.
func TestLoadKEMKeyFromV1LegacyFile(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store0, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		t.Fatal(err)
	}
	seed := dk.Bytes()
	defer zeroBytes(seed)
	ct, nonce, err := encrypt(seed, store0.key, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := encryptedEnvelope{
		Version:    envelopeVersionAEADLegacy,
		Algorithm:  agentTokenEncryptionAlgorithm,
		NonceB64:   base64.StdEncoding.EncodeToString(nonce),
		PayloadB64: base64.StdEncoding.EncodeToString(ct),
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	kemPath := filepath.Join(stateDir, "secrets", kemKeystoreFile)
	if err := os.WriteFile(kemPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store1, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if err := store1.PutAgentToken("kem-v1", "tok", ""); err != nil {
		t.Fatalf("PutAgentToken: %v", err)
	}
	if _, err := store1.GetAgentToken("kem-v1"); err != nil {
		t.Fatalf("GetAgentToken: %v", err)
	}
}

// TestGetAgentToken_ReadsV2LegacyPQEnvelope asserts legacy v2 (direct shared key, no AAD) still decrypts and migrates.
func TestGetAgentToken_ReadsV2LegacyPQEnvelope(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	dk, err := store.getOrCreateKEMKey()
	if err != nil {
		t.Fatalf("kem: %v", err)
	}
	ek := dk.EncapsulationKey()
	sharedKey, kemCt := ek.Encapsulate()
	defer zeroBytes(sharedKey)
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(cryptorand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	record := []byte(`{"service_id":"v2leg","token":"tok-v2","expires_at":"","written_at":"2026-03-06T00:00:00Z"}`)
	gcmCt := gcm.Seal(nil, nonce, record, nil)
	env := encryptedEnvelope{
		Version:          envelopeVersionPQLegacy,
		Algorithm:        algorithmPQKEM,
		KemCiphertextB64: base64.StdEncoding.EncodeToString(kemCt),
		NonceB64:         base64.StdEncoding.EncodeToString(nonce),
		PayloadB64:       base64.StdEncoding.EncodeToString(gcmCt),
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "v2leg"+agentTokenEncryptedFileSuffix), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAgentToken("v2leg")
	if err != nil {
		t.Fatalf("GetAgentToken: %v", err)
	}
	if got.Token != "tok-v2" || got.ServiceID != "v2leg" {
		t.Fatalf("record %+v", got)
	}
}

// TestGCMWithAAD verifies FIPS AEAD-only path seals with non-empty AAD (v3 envelope).
func TestGCMWithAAD(t *testing.T) {
	TestPutGetAgentToken_AEADOnlyFallback(t)
}

// TestKEMWithHKDF verifies PQ-permitted path uses ML-KEM + HKDF + AES-GCM with AAD (v4 envelope).
func TestKEMWithHKDF(t *testing.T) {
	TestPutGetAgentToken_PQPath(t)
}

func TestGetAgentToken_UnsupportedAEADAADAlgorithm(t *testing.T) {
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatal(err)
	}
	env := `{"version":3,"algorithm":"NOT-AES","nonce_b64":"AAAAAAAAAAAAAAAAAAAAAA==","payload_b64":"AAAAAAAAAAAAAAAAAAAAAA=="}`
	if err := os.WriteFile(filepath.Join(tokenDir, "badalg"+agentTokenEncryptedFileSuffix), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAgentToken("badalg"); err == nil {
		t.Fatal("expected error for unsupported algorithm on v3 envelope")
	}
}

func TestLoadKEMKeyFromFile_UnsupportedVersion(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store0, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = store0.PutAgentToken("warm", "x", "")
	kemPath := filepath.Join(stateDir, "secrets", kemKeystoreFile)
	env := `{"version":99,"algorithm":"AES-256-GCM","nonce_b64":"AAAAAAAAAAAAAAAAAAAAAA==","payload_b64":"AAAAAAAAAAAAAAAAAAAAAA=="}`
	if err := os.WriteFile(kemPath, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	store1, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if err := store1.PutAgentToken("y", "z", ""); err == nil {
		t.Fatal("expected failure when kem keystore envelope is unsupported")
	}
}

func TestGetAgentToken_PQv4DecryptError(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = store.PutAgentToken("seed", "x", "")
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	env := `{"version":4,"algorithm":"` + algorithmPQKEMHKDF + `","kem_ciphertext_b64":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==","nonce_b64":"AAAAAAAAAAAAAAAAAAAAAA==","payload_b64":"AAAA"}`
	if err := os.WriteFile(filepath.Join(tokenDir, "badpq"+agentTokenEncryptedFileSuffix), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAgentToken("badpq"); err == nil {
		t.Fatal("expected decrypt error")
	}
}

// TestLoadKEMKeyFromFile_WrongSeedLength asserts decrypting a v3 kem keystore whose plaintext is not mlkem.SeedSize bytes fails.
func TestLoadKEMKeyFromFile_WrongSeedLength(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store0, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wrongLen := make([]byte, mlkem.SeedSize-1)
	ct, nonce, err := encrypt(wrongLen, store0.key, aadKEMKeystore())
	if err != nil {
		t.Fatal(err)
	}
	env := encryptedEnvelope{
		Version:    envelopeVersionAEADAAD,
		Algorithm:  agentTokenEncryptionAlgorithm,
		NonceB64:   base64.StdEncoding.EncodeToString(nonce),
		PayloadB64: base64.StdEncoding.EncodeToString(ct),
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	kemPath := filepath.Join(stateDir, "secrets", kemKeystoreFile)
	if err := os.WriteFile(kemPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store1, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if err := store1.PutAgentToken("z", "z", ""); err == nil {
		t.Fatal("expected failure when kem keystore plaintext has wrong seed length")
	}
}

// TestDecryptPQLegacy_TamperedGCMFails asserts decryptPQLegacy returns an error when GCM authentication fails.
func TestDecryptPQLegacy_TamperedGCMFails(t *testing.T) {
	testFIPSModeKnownOff = true
	defer func() { testFIPSModeKnownOff = true }()
	t.Setenv(masterKeyEnvName, validMasterKeyB64())
	stateDir := t.TempDir()
	store, _, err := Open(stateDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	dk, err := store.getOrCreateKEMKey()
	if err != nil {
		t.Fatalf("kem: %v", err)
	}
	ek := dk.EncapsulationKey()
	sharedKey, kemCt := ek.Encapsulate()
	defer zeroBytes(sharedKey)
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(cryptorand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	record := []byte(`{"service_id":"tam","token":"x","expires_at":"","written_at":"2026-03-06T00:00:00Z"}`)
	gcmCt := gcm.Seal(nil, nonce, record, nil)
	if len(gcmCt) > 0 {
		gcmCt[len(gcmCt)-1] ^= 0xff
	}
	env := encryptedEnvelope{
		Version:          envelopeVersionPQLegacy,
		Algorithm:        algorithmPQKEM,
		KemCiphertextB64: base64.StdEncoding.EncodeToString(kemCt),
		NonceB64:         base64.StdEncoding.EncodeToString(nonce),
		PayloadB64:       base64.StdEncoding.EncodeToString(gcmCt),
	}
	tokenDir := filepath.Join(stateDir, "secrets", "agent_tokens")
	if err := os.MkdirAll(tokenDir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, "tam"+agentTokenEncryptedFileSuffix), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAgentToken("tam"); err == nil {
		t.Fatal("expected GCM open failure for tampered v2 payload")
	}
}
