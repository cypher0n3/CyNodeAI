package auth

import (
	"bytes"
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hash, err := HashPassword(password, nil)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if len(hash) == 0 {
		t.Fatal("HashPassword() returned empty hash")
	}

	// Hash should be different each time (different salt)
	hash2, _ := HashPassword(password, nil)
	if bytes.Equal(hash, hash2) {
		t.Error("HashPassword() returned same hash for same password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-123"

	hash, _ := HashPassword(password, nil)

	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	if !valid {
		t.Error("VerifyPassword() returned false for correct password")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	password := "test-password-123"
	wrongPassword := "wrong-password"

	hash, _ := HashPassword(password, nil)

	valid, err := VerifyPassword(wrongPassword, hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	if valid {
		t.Error("VerifyPassword() returned true for wrong password")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	_, err := VerifyPassword("password", []byte("invalid-hash"))
	if err == nil {
		t.Error("VerifyPassword() expected error for invalid hash")
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token-12345"

	hash := HashToken(token)

	if len(hash) != 32 {
		t.Errorf("HashToken() returned hash of length %d, want 32", len(hash))
	}

	// Same token should produce same hash
	hash2 := HashToken(token)
	if !bytes.Equal(hash, hash2) {
		t.Error("HashToken() returned different hash for same token")
	}

	// Different token should produce different hash
	hash3 := HashToken("different-token")
	if bytes.Equal(hash, hash3) {
		t.Error("HashToken() returned same hash for different token")
	}
}

func TestDefaultArgon2Params(t *testing.T) {
	params := DefaultArgon2Params()

	if params.Memory == 0 {
		t.Error("DefaultArgon2Params() Memory is 0")
	}

	if params.Iterations == 0 {
		t.Error("DefaultArgon2Params() Iterations is 0")
	}

	if params.Parallelism == 0 {
		t.Error("DefaultArgon2Params() Parallelism is 0")
	}

	if params.SaltLength == 0 {
		t.Error("DefaultArgon2Params() SaltLength is 0")
	}

	if params.KeyLength == 0 {
		t.Error("DefaultArgon2Params() KeyLength is 0")
	}
}

func TestHashPasswordWithCustomParams(t *testing.T) {
	params := &Argon2Params{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hash, err := HashPassword("password", params)
	if err != nil {
		t.Fatalf("HashPassword() with custom params error = %v", err)
	}

	valid, err := VerifyPassword("password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !valid {
		t.Error("VerifyPassword() returned false for correct password with custom params")
	}
}

func TestDecodeArgon2Hash_MalformedHash(t *testing.T) {
	testCases := []struct {
		name string
		hash []byte
	}{
		{"wrong prefix", []byte("$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA")},
		{"missing parts", []byte("$argon2id$v=19")},
		{"invalid version", []byte("$argon2id$v=abc$m=65536,t=3,p=2$c2FsdA$aGFzaA")},
		{"invalid params", []byte("$argon2id$v=19$invalid$c2FsdA$aGFzaA")},
		{"invalid base64 salt", []byte("$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA")},
		{"invalid base64 hash", []byte("$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyPassword("password", tc.hash)
			if err == nil {
				t.Error("VerifyPassword() expected error for malformed hash")
			}
		})
	}
}

func TestArgon2ParamsStruct(t *testing.T) {
	p := Argon2Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}

	if p.Memory != 64*1024 {
		t.Errorf("expected Memory 64*1024, got %d", p.Memory)
	}
}
