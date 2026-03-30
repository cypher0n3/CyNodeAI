package handlers_test

import (
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/fieldcrypt"
)

// TestTokenEncryption asserts worker bearer tokens are not stored as plaintext strings (Task 6).
func TestTokenEncryption(t *testing.T) {
	key := fieldcrypt.DeriveKeyFromJWTSecret("integration-test-jwt")
	plain := "plaintext-worker-api-bearer-for-at-rest-test"
	enc, err := fieldcrypt.EncryptWorkerBearerToken(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("stored form must not equal plaintext")
	}
	got, err := fieldcrypt.DecryptWorkerBearerToken(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("decrypt: got %q", got)
	}
}
