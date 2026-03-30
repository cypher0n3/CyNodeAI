package fieldcrypt

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptWorkerBearerToken_NotPlaintextAtRest(t *testing.T) {
	key := DeriveKeyFromJWTSecret("test-jwt-secret-for-encryption")
	plain := "my-secret-bearer-token-value"
	enc, err := EncryptWorkerBearerToken(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("ciphertext must not equal plaintext")
	}
	if !strings.HasPrefix(enc, EncryptedWorkerBearerPrefix) {
		t.Fatalf("want enc1 prefix, got %q", enc)
	}
	got, err := DecryptWorkerBearerToken(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("round-trip: got %q want %q", got, plain)
	}
}

func TestDecryptWorkerBearerToken_LegacyPlaintext(t *testing.T) {
	key := DeriveKeyFromJWTSecret("x")
	plain := "legacy-unencrypted"
	got, err := DecryptWorkerBearerToken(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q", got)
	}
}

func TestEncryptWorkerBearerToken_EmptyKeyNoOp(t *testing.T) {
	plain := "tok"
	got, err := EncryptWorkerBearerToken(plain, nil)
	if err != nil || got != plain {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestDeriveKeyFromJWTSecret_EmptyReturnsNil(t *testing.T) {
	if DeriveKeyFromJWTSecret("") != nil {
		t.Fatal("empty JWT secret should yield nil key")
	}
}

func TestEncryptWorkerBearerToken_EmptyPlaintextNoOp(t *testing.T) {
	key := DeriveKeyFromJWTSecret("k")
	got, err := EncryptWorkerBearerToken("", key)
	if err != nil || got != "" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestEncryptWorkerBearerToken_IdempotentWhenAlreadyEnc1(t *testing.T) {
	key := DeriveKeyFromJWTSecret("k")
	first, err := EncryptWorkerBearerToken("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptWorkerBearerToken(first, key)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("re-encrypt enc1 value should be no-op, got %q from %q", second, first)
	}
}

func TestDecryptWorkerBearerToken_EmptyStored(t *testing.T) {
	key := DeriveKeyFromJWTSecret("k")
	got, err := DecryptWorkerBearerToken("", key)
	if err != nil || got != "" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestDecryptWorkerBearerToken_InvalidBase64(t *testing.T) {
	key := DeriveKeyFromJWTSecret("k")
	_, err := DecryptWorkerBearerToken(EncryptedWorkerBearerPrefix+"@@@", key)
	if err == nil {
		t.Fatal("want base64 error")
	}
}

func TestDecryptWorkerBearerToken_CiphertextTooShort(t *testing.T) {
	key := DeriveKeyFromJWTSecret("k")
	short := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	_, err := DecryptWorkerBearerToken(EncryptedWorkerBearerPrefix+short, key)
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("want too short error, got %v", err)
	}
}

func TestDecryptWorkerBearerToken_WrongKeyFailsOpen(t *testing.T) {
	k1 := DeriveKeyFromJWTSecret("secret-a")
	k2 := DeriveKeyFromJWTSecret("secret-b")
	enc, err := EncryptWorkerBearerToken("payload", k1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecryptWorkerBearerToken(enc, k2)
	if err == nil {
		t.Fatal("wrong key should fail GCM open")
	}
}

func TestIsEncryptedWorkerBearerToken(t *testing.T) {
	if !IsEncryptedWorkerBearerToken(EncryptedWorkerBearerPrefix + "eA==") {
		t.Error("enc1 prefix should count as encrypted")
	}
	if IsEncryptedWorkerBearerToken("plain") {
		t.Error("plain should be false")
	}
}

func TestEncryptWorkerBearerToken_InvalidAESKeyLength(t *testing.T) {
	_, err := EncryptWorkerBearerToken("secret", []byte{1, 2, 3})
	if err == nil {
		t.Fatal("invalid AES key length must fail NewCipher")
	}
}
