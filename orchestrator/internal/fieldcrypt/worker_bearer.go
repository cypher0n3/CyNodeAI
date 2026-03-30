// Package fieldcrypt provides at-rest encryption for sensitive DB string fields (orchestrator).
package fieldcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// EncryptedWorkerBearerPrefix marks AES-GCM ciphertext stored in worker_api_bearer_token.
const EncryptedWorkerBearerPrefix = "enc1:"

// DeriveKeyFromJWTSecret derives a 32-byte AES key from the orchestrator JWT secret (Task 6).
func DeriveKeyFromJWTSecret(jwtSecret string) []byte {
	if jwtSecret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte("cynodeai:v1:worker_api_bearer_token:" + jwtSecret))
	return sum[:]
}

// EncryptWorkerBearerToken returns an enc1:-prefixed ciphertext, or plaintext unchanged if key is nil/empty.
func EncryptWorkerBearerToken(plaintext string, key []byte) (string, error) {
	if plaintext == "" || len(key) == 0 {
		return plaintext, nil
	}
	if strings.HasPrefix(plaintext, EncryptedWorkerBearerPrefix) {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	combined := append(append([]byte{}, nonce...), sealed...)
	return EncryptedWorkerBearerPrefix + base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptWorkerBearerToken decrypts enc1: values; legacy plaintext is returned as-is.
func DecryptWorkerBearerToken(stored string, key []byte) (string, error) {
	if stored == "" || len(key) == 0 {
		return stored, nil
	}
	if !strings.HasPrefix(stored, EncryptedWorkerBearerPrefix) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, EncryptedWorkerBearerPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("worker bearer ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// IsEncryptedWorkerBearerToken reports whether the DB value uses enc1: encoding.
func IsEncryptedWorkerBearerToken(s string) bool {
	return strings.HasPrefix(s, EncryptedWorkerBearerPrefix)
}
