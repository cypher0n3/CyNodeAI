// Package securestore provides encrypted-at-rest secret persistence for worker-node secrets.
package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultStateDir                     = "/var/lib/cynode/state"
	masterKeyEnvName                    = "CYNODE_SECURE_STORE_MASTER_KEY_B64"
	systemCredentialMasterKeyFile       = "cynode-secure-store-master-key.b64"
	agentTokenStoreDir                  = "agent_tokens"
	agentTokenEncryptedFileSuffix       = ".json.enc"
	agentTokenEncryptionAlgorithm       = "AES-256-GCM"
	envelopeVersionAEAD                 = 1
	envelopeVersionPQ                    = 2
	algorithmPQKEM                       = "ML-KEM-768+AES-256-GCM"
	kemKeystoreFile                     = ".kem_keystore.enc"
	requiredKeyLenBytes                 = 32
)

var (
	// ErrMasterKeyNotConfigured indicates that no secure-store master key source was available.
	ErrMasterKeyNotConfigured = errors.New("secure store master key is not configured")
	// ErrMasterKeyInvalid indicates the configured key is invalid.
	ErrMasterKeyInvalid = errors.New("secure store master key is invalid")
	// ErrTokenExpired indicates a stored token has expired and must not be used.
	ErrTokenExpired = errors.New("agent token expired")
	// ErrFIPSRequiresNonEnvKey indicates FIPS mode is on and env fallback is not allowed; use TPM, OS key store, or system credential.
	ErrFIPSRequiresNonEnvKey = errors.New("FIPS mode: secure store master key must not come from env; use TPM, OS key store, or system credential")
)

// MasterKeySource identifies which master-key backend was used.
type MasterKeySource string

const (
	MasterKeySourceTPM              MasterKeySource = "tpm"
	MasterKeySourceOSKeyStore       MasterKeySource = "os_key_store"
	MasterKeySourceSystemCredential MasterKeySource = "system_credential"
	MasterKeySourceEnvB64           MasterKeySource = "env_b64"
)

type encryptedEnvelope struct {
	Version           int    `json:"version"`
	Algorithm         string `json:"algorithm"`
	KemCiphertextB64  string `json:"kem_ciphertext_b64,omitempty"` // v2 only
	NonceB64          string `json:"nonce_b64"`
	PayloadB64        string `json:"payload_b64"`
}

// AgentTokenRecord is the plaintext token record stored encrypted at rest.
type AgentTokenRecord struct {
	ServiceID string `json:"service_id"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at,omitempty"`
	WrittenAt string `json:"written_at"`
}

// Store is the secure-store handle.
type Store struct {
	rootDir   string
	key       []byte
	kemKey    *mlkem.DecapsulationKey768
	kemKeyMu  sync.Mutex
}

// Open initializes the secure store under <state_dir>/secrets and resolves a master key.
// Master key resolution runs inside runtime/secret when available so temporaries are erased.
func Open(stateDir string) (*Store, MasterKeySource, error) {
	if strings.TrimSpace(stateDir) == "" {
		stateDir = defaultStateDir
	}
	rootDir := filepath.Join(stateDir, "secrets")
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create secure store dir: %w", err)
	}
	if err := os.Chmod(rootDir, 0o700); err != nil {
		return nil, "", fmt.Errorf("set secure store dir permissions: %w", err)
	}
	var key []byte
	var source MasterKeySource
	var err error
	runWithSecret(func() {
		key, source, err = resolveMasterKey()
	})
	if err != nil {
		return nil, "", err
	}
	// Per CYNAI.WORKER.NodeLocalSecureStore: in FIPS mode, do not allow env fallback (use stronger key source).
	if isFIPSMode() && source == MasterKeySourceEnvB64 {
		zeroBytes(key)
		return nil, "", ErrFIPSRequiresNonEnvKey
	}
	return &Store{rootDir: rootDir, key: key}, source, nil
}

// resolveMasterKey returns the 256-bit master key using spec precedence: TPM, OS key store, system credential, env fallback.
func resolveMasterKey() ([]byte, MasterKeySource, error) {
	if key, err := loadMasterKeyFromTPM(); err == nil {
		return key, MasterKeySourceTPM, nil
	}
	if key, err := loadMasterKeyFromOSKeyStore(); err == nil {
		return key, MasterKeySourceOSKeyStore, nil
	}
	if key, err := loadMasterKeyFromSystemCredential(); err == nil {
		return key, MasterKeySourceSystemCredential, nil
	}
	key, err := loadMasterKeyFromEnv()
	if err == nil {
		return key, MasterKeySourceEnvB64, nil
	}
	if errors.Is(err, ErrMasterKeyNotConfigured) {
		return nil, "", ErrMasterKeyNotConfigured
	}
	return nil, "", err
}

// loadMasterKeyFromTPM returns the master key from TPM-sealed storage when supported and configured. Not yet implemented.
func loadMasterKeyFromTPM() ([]byte, error) {
	return nil, ErrMasterKeyNotConfigured
}

// loadMasterKeyFromOSKeyStore returns the master key from the OS key store when supported and configured. Not yet implemented.
func loadMasterKeyFromOSKeyStore() ([]byte, error) {
	return nil, ErrMasterKeyNotConfigured
}

func loadMasterKeyFromSystemCredential() ([]byte, error) {
	credDir := strings.TrimSpace(os.Getenv("CREDENTIALS_DIRECTORY"))
	if credDir == "" {
		return nil, ErrMasterKeyNotConfigured
	}
	path := filepath.Join(credDir, systemCredentialMasterKeyFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrMasterKeyNotConfigured
		}
		return nil, fmt.Errorf("read secure store system credential: %w", err)
	}
	return decodeMasterKey(strings.TrimSpace(string(raw)))
}

func loadMasterKeyFromEnv() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(masterKeyEnvName))
	if raw == "" {
		return nil, ErrMasterKeyNotConfigured
	}
	return decodeMasterKey(raw)
}

func decodeMasterKey(rawB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode", ErrMasterKeyInvalid)
	}
	if len(key) != requiredKeyLenBytes {
		return nil, fmt.Errorf("%w: expected 32 bytes", ErrMasterKeyInvalid)
	}
	return key, nil
}

func (s *Store) tokenDir() string {
	return filepath.Join(s.rootDir, agentTokenStoreDir)
}

func sanitizeServiceID(serviceID string) (string, error) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return "", errors.New("service_id is required")
	}
	if strings.Contains(serviceID, "/") || strings.Contains(serviceID, "\\") || strings.Contains(serviceID, "..") {
		return "", errors.New("invalid service_id path")
	}
	return serviceID, nil
}

func (s *Store) tokenPath(serviceID string) (string, error) {
	serviceID, err := sanitizeServiceID(serviceID)
	if err != nil {
		return "", err
	}
	filename := serviceID + agentTokenEncryptedFileSuffix
	return filepath.Join(s.tokenDir(), filename), nil
}

// PutAgentToken writes or rotates a per-service token record.
func (s *Store) PutAgentToken(serviceID, token, expiresAt string) error {
	serviceID, err := sanitizeServiceID(serviceID)
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("agent token is required")
	}
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt != "" {
		if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
			return fmt.Errorf("invalid expires_at: %w", err)
		}
	}
	record := AgentTokenRecord{
		ServiceID: serviceID,
		Token:     token,
		ExpiresAt: expiresAt,
		WrittenAt: time.Now().UTC().Format(time.RFC3339),
	}
	var env encryptedEnvelope
	var encErr error
	runWithSecret(func() {
		plaintext, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			encErr = fmt.Errorf("marshal token record: %w", marshalErr)
			return
		}
		defer zeroBytes(plaintext)
		var e encryptedEnvelope
		e, encErr = s.buildEncryptedEnvelope(plaintext)
		if encErr != nil {
			return
		}
		env = e
	})
	if encErr != nil {
		return encErr
	}
	serialized, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal encrypted envelope: %w", err)
	}
	dir := s.tokenDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	path, err := s.tokenPath(serviceID)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, serialized, 0o600); err != nil {
		return fmt.Errorf("write token tmp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit token file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set token file permissions: %w", err)
	}
	return nil
}

// GetAgentToken reads and decrypts a per-service token record.
// Decrypt and plaintext handling run inside runtime/secret when available; zeroBytes remains fallback.
func (s *Store) GetAgentToken(serviceID string) (*AgentTokenRecord, error) {
	path, err := s.tokenPath(serviceID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env encryptedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode encrypted envelope: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.NonceB64)
	if err != nil {
		return nil, fmt.Errorf("decode envelope nonce: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode envelope payload: %w", err)
	}
	var record *AgentTokenRecord
	var decErr error
	runWithSecret(func() {
		plaintext, err := s.decryptEnvelope(&env, nonce, payload)
		if err != nil {
			decErr = err
			return
		}
		defer zeroBytes(plaintext)
		var r AgentTokenRecord
		if err := json.Unmarshal(plaintext, &r); err != nil {
			decErr = fmt.Errorf("decode token record: %w", err)
			return
		}
		if strings.TrimSpace(r.ExpiresAt) != "" {
			expiresAt, err := time.Parse(time.RFC3339, r.ExpiresAt)
			if err != nil {
				decErr = fmt.Errorf("invalid stored expires_at: %w", err)
				return
			}
			if !time.Now().UTC().Before(expiresAt) {
				decErr = ErrTokenExpired
				return
			}
		}
		record = &r
	})
	if decErr != nil {
		return nil, decErr
	}
	return record, nil
}

// DeleteAgentToken removes a per-service token record.
func (s *Store) DeleteAgentToken(serviceID string) error {
	path, err := s.tokenPath(serviceID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ListAgentTokenServiceIDs returns service IDs present in the token store.
func (s *Store) ListAgentTokenServiceIDs() ([]string, error) {
	entries, err := os.ReadDir(s.tokenDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, agentTokenEncryptedFileSuffix) {
			continue
		}
		serviceID := strings.TrimSuffix(name, agentTokenEncryptedFileSuffix)
		if _, err := sanitizeServiceID(serviceID); err != nil {
			continue
		}
		out = append(out, serviceID)
	}
	return out, nil
}

func encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("read nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plaintext, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// isPQPermitted reports whether post-quantum KEM is allowed. When FIPS mode is on we use AEAD-only fallback.
func isPQPermitted() bool {
	return !isFIPSMode()
}

func (s *Store) kemKeystorePath() string {
	return filepath.Join(s.rootDir, kemKeystoreFile)
}

// getOrCreateKEMKey returns the store's ML-KEM decapsulation key, creating and persisting it if needed.
func (s *Store) getOrCreateKEMKey() (*mlkem.DecapsulationKey768, error) {
	s.kemKeyMu.Lock()
	defer s.kemKeyMu.Unlock()
	if s.kemKey != nil {
		return s.kemKey, nil
	}
	path := s.kemKeystorePath()
	dk, err := s.loadKEMKeyFromFile(path)
	if err == nil {
		s.kemKey = dk
		return dk, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	dk, err = s.persistNewKEMKey(path)
	if err != nil {
		return nil, err
	}
	s.kemKey = dk
	return dk, nil
}

func (s *Store) loadKEMKeyFromFile(path string) (*mlkem.DecapsulationKey768, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env encryptedEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode kem keystore envelope: %w", err)
	}
	if env.Version != envelopeVersionAEAD || env.Algorithm != agentTokenEncryptionAlgorithm {
		return nil, errors.New("unsupported kem keystore envelope")
	}
	nonce, err := base64.StdEncoding.DecodeString(env.NonceB64)
	if err != nil {
		return nil, fmt.Errorf("decode kem keystore nonce: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(env.PayloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode kem keystore payload: %w", err)
	}
	var dk *mlkem.DecapsulationKey768
	var loadErr error
	runWithSecret(func() {
		seed, err := decrypt(payload, nonce, s.key)
		if err != nil {
			loadErr = fmt.Errorf("decrypt kem keystore: %w", err)
			return
		}
		defer zeroBytes(seed)
		if len(seed) != mlkem.SeedSize {
			loadErr = fmt.Errorf("kem keystore payload length %d, want %d", len(seed), mlkem.SeedSize)
			return
		}
		dk, loadErr = mlkem.NewDecapsulationKey768(seed)
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return dk, nil
}

func (s *Store) persistNewKEMKey(path string) (*mlkem.DecapsulationKey768, error) {
	dk, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("generate kem key: %w", err)
	}
	var ciphertext, nonce []byte
	var persistErr error
	runWithSecret(func() {
		seed := dk.Bytes()
		defer zeroBytes(seed)
		ciphertext, nonce, persistErr = encrypt(seed, s.key)
	})
	if persistErr != nil {
		return nil, fmt.Errorf("encrypt kem keystore: %w", persistErr)
	}
	env := encryptedEnvelope{
		Version:    envelopeVersionAEAD,
		Algorithm:  agentTokenEncryptionAlgorithm,
		NonceB64:   base64.StdEncoding.EncodeToString(nonce),
		PayloadB64: base64.StdEncoding.EncodeToString(ciphertext),
	}
	serialized, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal kem keystore: %w", err)
	}
	if err := os.WriteFile(path, serialized, 0o600); err != nil {
		return nil, fmt.Errorf("write kem keystore: %w", err)
	}
	return dk, nil
}

func (s *Store) buildEncryptedEnvelope(plaintext []byte) (encryptedEnvelope, error) {
	if isPQPermitted() {
		kemCt, nonce, gcmCt, err := s.encryptPQ(plaintext)
		if err != nil {
			return encryptedEnvelope{}, err
		}
		return encryptedEnvelope{
			Version:          envelopeVersionPQ,
			Algorithm:       algorithmPQKEM,
			KemCiphertextB64: base64.StdEncoding.EncodeToString(kemCt),
			NonceB64:         base64.StdEncoding.EncodeToString(nonce),
			PayloadB64:       base64.StdEncoding.EncodeToString(gcmCt),
		}, nil
	}
	ciphertext, nonce, err := encrypt(plaintext, s.key)
	if err != nil {
		return encryptedEnvelope{}, err
	}
	return encryptedEnvelope{
		Version:    envelopeVersionAEAD,
		Algorithm:  agentTokenEncryptionAlgorithm,
		NonceB64:   base64.StdEncoding.EncodeToString(nonce),
		PayloadB64: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (s *Store) decryptEnvelope(env *encryptedEnvelope, nonce, payload []byte) ([]byte, error) {
	if env.Version == envelopeVersionPQ && env.Algorithm == algorithmPQKEM {
		if env.KemCiphertextB64 == "" {
			return nil, errors.New("missing kem_ciphertext_b64 in v2 envelope")
		}
		kemCt, err := base64.StdEncoding.DecodeString(env.KemCiphertextB64)
		if err != nil {
			return nil, fmt.Errorf("decode envelope kem_ciphertext: %w", err)
		}
		return s.decryptPQ(kemCt, nonce, payload)
	}
	if env.Version == envelopeVersionAEAD && env.Algorithm == agentTokenEncryptionAlgorithm {
		return decrypt(payload, nonce, s.key)
	}
	return nil, errors.New("unsupported secure store envelope")
}

// encryptPQ encrypts plaintext using ML-KEM-768 + AES-256-GCM; returns (kemCiphertext, nonce, gcmCiphertext).
func (s *Store) encryptPQ(plaintext []byte) (kemCt, nonce, gcmCt []byte, err error) {
	dk, err := s.getOrCreateKEMKey()
	if err != nil {
		return nil, nil, nil, err
	}
	ek := dk.EncapsulationKey()
	sharedKey, kemCt := ek.Encapsulate()
	defer zeroBytes(sharedKey)
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create gcm: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, fmt.Errorf("read nonce: %w", err)
	}
	gcmCt = gcm.Seal(nil, nonce, plaintext, nil)
	return kemCt, nonce, gcmCt, nil
}

// decryptPQ decrypts a v2 envelope payload using the store's ML-KEM key.
func (s *Store) decryptPQ(kemCt, nonce, gcmCt []byte) ([]byte, error) {
	dk, err := s.getOrCreateKEMKey()
	if err != nil {
		return nil, err
	}
	sharedKey, err := dk.Decapsulate(kemCt)
	if err != nil {
		return nil, fmt.Errorf("mlkem decapsulate: %w", err)
	}
	defer zeroBytes(sharedKey)
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, gcmCt, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plaintext, nil
}
