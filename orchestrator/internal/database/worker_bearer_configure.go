package database

import (
	"context"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/fieldcrypt"
)

// ConfigureWorkerBearerEncryptionFromJWT derives the AES key from the orchestrator JWT secret and
// configures at-rest encryption for worker_api_bearer_token on *DB. No-op for other Store implementations.
func ConfigureWorkerBearerEncryptionFromJWT(store Store, jwtSecret string) {
	db, ok := store.(*DB)
	if !ok || jwtSecret == "" {
		return
	}
	db.SetWorkerBearerTokenKey(fieldcrypt.DeriveKeyFromJWTSecret(jwtSecret))
}

// ApplyWorkerBearerEncryptionAtStartup derives the encryption key from jwtSecret and migrates legacy
// plaintext worker_api_bearer_token values to enc1: form. Safe to call on every process start.
func ApplyWorkerBearerEncryptionAtStartup(ctx context.Context, db *DB, jwtSecret string) error {
	ConfigureWorkerBearerEncryptionFromJWT(db, jwtSecret)
	return db.MigratePlaintextWorkerBearerTokens(ctx)
}
