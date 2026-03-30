package database

import (
	"context"
	"testing"
)

func TestConfigureWorkerBearerEncryptionFromJWT_EmptyJWTNoOp(t *testing.T) {
	db := &DB{}
	ConfigureWorkerBearerEncryptionFromJWT(db, "")
	if len(db.workerBearerKey) != 0 {
		t.Fatal("empty JWT should not set key")
	}
}

func TestConfigureWorkerBearerEncryptionFromJWT_SetsKey(t *testing.T) {
	db := &DB{}
	ConfigureWorkerBearerEncryptionFromJWT(db, "test-jwt")
	if len(db.workerBearerKey) != 32 {
		t.Fatalf("want 32-byte AES key, got %d", len(db.workerBearerKey))
	}
}

func TestMigratePlaintextWorkerBearerTokens_NoKeyNoOp(t *testing.T) {
	db := &DB{}
	if err := db.MigratePlaintextWorkerBearerTokens(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestApplyWorkerBearerEncryptionAtStartup_EmptyJWTNoError(t *testing.T) {
	db := &DB{}
	if err := ApplyWorkerBearerEncryptionAtStartup(context.Background(), db, ""); err != nil {
		t.Fatal(err)
	}
}
