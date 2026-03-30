package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/fieldcrypt"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func tcWorkerBearerDispatchableNode(t *testing.T, ctx context.Context, db *DB, token string) *models.Node {
	t.Helper()
	node, err := db.CreateNode(ctx, "wb-enc-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	if err := db.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://worker:12090", token); err != nil {
		t.Fatalf("UpdateNodeWorkerAPIConfig: %v", err)
	}
	if err := db.UpdateNodeConfigVersion(ctx, node.ID, "1"); err != nil {
		t.Fatalf("UpdateNodeConfigVersion: %v", err)
	}
	ackAt := time.Now().UTC()
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck: %v", err)
	}
	return node
}

func tcBearerFromDispatchable(t *testing.T, ctx context.Context, db *DB, nodeID uuid.UUID) string {
	t.Helper()
	dispatchable, err := db.ListDispatchableNodes(ctx)
	if err != nil {
		t.Fatalf("ListDispatchableNodes: %v", err)
	}
	for _, n := range dispatchable {
		if n.ID == nodeID && n.WorkerAPIBearerToken != nil {
			return *n.WorkerAPIBearerToken
		}
	}
	t.Fatalf("node %s not in dispatchable list or missing bearer", nodeID)
	return ""
}

// TestWithTestcontainers_WorkerBearerEncryption exercises enc1: storage, decrypt on read, and plaintext migration.
func TestWithTestcontainers_WorkerBearerEncryption(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	db, ok := store.(*DB)
	if !ok {
		t.Fatal("tcOpenDB should return *DB")
	}
	if err := ApplyWorkerBearerEncryptionAtStartup(ctx, db, "tc-jwt-secret-worker-bearer"); err != nil {
		t.Fatalf("ApplyWorkerBearerEncryptionAtStartup: %v", err)
	}

	const secretPlain = "secret-token-value"
	node := tcWorkerBearerDispatchableNode(t, ctx, db, secretPlain)
	var rec NodeRecord
	if err := db.db.WithContext(ctx).Where("id = ?", node.ID).First(&rec).Error; err != nil {
		t.Fatalf("load node row: %v", err)
	}
	if rec.WorkerAPIBearerToken == nil || !fieldcrypt.IsEncryptedWorkerBearerToken(*rec.WorkerAPIBearerToken) {
		t.Fatalf("DB row should store enc1: ciphertext, got %v", rec.WorkerAPIBearerToken)
	}
	if got := tcBearerFromDispatchable(t, ctx, db, node.ID); got != secretPlain {
		t.Fatalf("dispatchable node should decrypt token, got %q", got)
	}

	legacy := "legacy-plaintext-token"
	if err := db.db.WithContext(ctx).Exec(
		"UPDATE nodes SET worker_api_bearer_token = ? WHERE id = ?", legacy, node.ID,
	).Error; err != nil {
		t.Fatalf("set legacy plaintext: %v", err)
	}
	if err := db.MigratePlaintextWorkerBearerTokens(ctx); err != nil {
		t.Fatalf("MigratePlaintextWorkerBearerTokens: %v", err)
	}
	if err := db.db.WithContext(ctx).Where("id = ?", node.ID).First(&rec).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if rec.WorkerAPIBearerToken == nil || !fieldcrypt.IsEncryptedWorkerBearerToken(*rec.WorkerAPIBearerToken) {
		t.Fatalf("after migrate, DB should hold enc1:, got %v", rec.WorkerAPIBearerToken)
	}
	if got := tcBearerFromDispatchable(t, ctx, db, node.ID); got != legacy {
		t.Fatalf("after migrate, read path should decrypt to legacy plaintext, got %q", got)
	}
}

// TestWithTestcontainers_UpdateNodeWorkerAPIConfig_EncryptError covers the error path when the AES key is invalid.
func TestWithTestcontainers_UpdateNodeWorkerAPIConfig_EncryptError(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	db := store.(*DB)
	db.SetWorkerBearerTokenKey([]byte{1, 2, 3})
	node, err := db.CreateNode(ctx, "wb-badkey-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	err = db.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://worker:12090", "tok")
	if err == nil {
		t.Fatal("UpdateNodeWorkerAPIConfig: expected error from encrypt with invalid AES key length")
	}
}

// TestWithTestcontainers_MigratePlaintextWorkerBearerTokens_EncryptError covers migrate when encryption fails for a legacy row.
func TestWithTestcontainers_MigratePlaintextWorkerBearerTokens_EncryptError(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	db := store.(*DB)
	ConfigureWorkerBearerEncryptionFromJWT(db, "migrate-encrypt-err-jwt")
	node, err := db.CreateNode(ctx, "wb-mig-err-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.db.WithContext(ctx).Exec(
		"UPDATE nodes SET worker_api_bearer_token = ? WHERE id = ?", "legacy-plain", node.ID,
	).Error; err != nil {
		t.Fatalf("seed legacy token: %v", err)
	}
	db.SetWorkerBearerTokenKey([]byte{1, 2, 3})
	if err := db.MigratePlaintextWorkerBearerTokens(ctx); err == nil {
		t.Fatal("MigratePlaintextWorkerBearerTokens: expected encrypt error")
	}
}
