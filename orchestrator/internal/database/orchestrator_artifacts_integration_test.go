// Integration tests for orchestrator artifact DB methods (requires POSTGRES_TEST_DSN or testcontainers TestMain).
package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestIntegration_OrchestratorArtifactStore(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "orch-art-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	part := "user:" + uid.String()
	path := "db-int/a.txt"
	storage := "s3://test/key"

	base := models.OrchestratorArtifactBase{
		ScopeLevel:     "user",
		ScopePartition: part,
		OwnerUserID:    &uid,
		Path:           path,
		StorageRef:     storage,
	}
	row := &models.OrchestratorArtifact{OrchestratorArtifactBase: base}

	created, err := db.CreateOrchestratorArtifact(ctx, row)
	if err != nil {
		t.Fatalf("CreateOrchestratorArtifact: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected id")
	}

	got, err := db.GetOrchestratorArtifactByID(ctx, created.ID)
	if err != nil || got.Path != path {
		t.Fatalf("GetOrchestratorArtifactByID: %v, %+v", err, got)
	}

	byPath, err := db.GetOrchestratorArtifactByScopePartitionAndPath(ctx, part, path)
	if err != nil || byPath.ID != created.ID {
		t.Fatalf("GetOrchestratorArtifactByScopePartitionAndPath: %v", err)
	}

	list, err := db.ListOrchestratorArtifacts(ctx, ListOrchestratorArtifactsParams{
		ScopeLevel:  "user",
		OwnerUserID: &uid,
		Limit:       10,
		Offset:      0,
	})
	if err != nil || len(list) < 1 {
		t.Fatalf("ListOrchestratorArtifacts: %v, n=%d", err, len(list))
	}
	orchestratorArtifactTestMetadataDupAndDelete(t, db, ctx, &base, created)
}

func orchestratorArtifactTestMetadataDupAndDelete(t *testing.T, db *DB, ctx context.Context, base *models.OrchestratorArtifactBase, created *models.OrchestratorArtifact) {
	t.Helper()
	sz := int64(42)
	ct := "text/plain"
	sum := "abc"
	if err := db.UpdateOrchestratorArtifactMetadata(ctx, created.ID, &sz, &ct, &sum, nil); err != nil {
		t.Fatalf("UpdateOrchestratorArtifactMetadata: %v", err)
	}
	if err := db.UpdateOrchestratorArtifactMetadata(ctx, created.ID, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpdateOrchestratorArtifactMetadata empty updates: %v", err)
	}
	if err := db.UpdateOrchestratorArtifactMetadata(ctx, uuid.New(), &sz, nil, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateOrchestratorArtifactMetadata missing id: %v", err)
	}

	if _, err := db.CreateOrchestratorArtifact(ctx, nil); err == nil {
		t.Fatal("CreateOrchestratorArtifact nil row: want error")
	}

	dup := &models.OrchestratorArtifact{OrchestratorArtifactBase: *base}
	if _, err := db.CreateOrchestratorArtifact(ctx, dup); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate create: want ErrExists, got %v", err)
	}

	if err := db.DeleteOrchestratorArtifactByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing: %v", err)
	}

	if err := db.DeleteOrchestratorArtifactByID(ctx, created.ID); err != nil {
		t.Fatalf("DeleteOrchestratorArtifactByID: %v", err)
	}
	if _, err := db.GetOrchestratorArtifactByID(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete: %v", err)
	}
}

func TestIntegration_OrchestratorArtifactChecksumAndStaleLists(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "orch-chk-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	part := "user:" + uid.String()
	path := "db-int/missing-sum.txt"
	row := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel:     "user",
			ScopePartition: part,
			OwnerUserID:    &uid,
			Path:           path,
			StorageRef:     "s3://test/m",
		},
	}
	created, err := db.CreateOrchestratorArtifact(ctx, row)
	if err != nil {
		t.Fatalf("CreateOrchestratorArtifact: %v", err)
	}
	// Clear checksum so backfill list picks it up.
	if err := db.GORM().WithContext(ctx).Model(&OrchestratorArtifactRecord{}).
		Where("id = ?", created.ID).
		Update("checksum_sha256", nil).Error; err != nil {
		t.Fatalf("clear checksum: %v", err)
	}
	ids, err := db.ListOrchestratorArtifactIDsMissingChecksum(ctx, 20)
	if err != nil {
		t.Fatalf("ListOrchestratorArtifactIDsMissingChecksum: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected artifact id in missing checksum list")
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	if err := db.GORM().WithContext(ctx).Model(&OrchestratorArtifactRecord{}).
		Where("id = ?", created.ID).
		Update("created_at", old).Error; err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	staleIDs, err := db.ListOrchestratorArtifactIDsCreatedBefore(ctx, cutoff, 20)
	if err != nil {
		t.Fatalf("ListOrchestratorArtifactIDsCreatedBefore: %v", err)
	}
	foundStale := false
	for _, id := range staleIDs {
		if id == created.ID {
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Fatal("expected artifact id in stale list")
	}

	_ = db.DeleteVectorItemsForArtifact(ctx, created.ID)
}

func TestOrchestratorArtifactRecord_ToOrchestratorArtifact_nilReceiver(t *testing.T) {
	var r *OrchestratorArtifactRecord
	if got := r.ToOrchestratorArtifact(); got != nil {
		t.Fatalf("nil receiver: got %+v", got)
	}
}

func TestIntegration_ArtifactReadGrant(t *testing.T) {
	db, ctx := integrationDB(t)
	owner, err := db.CreateUser(ctx, "grant-owner-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := db.CreateUser(ctx, "grant-other-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := owner.ID
	part := "user:" + uid.String()
	row := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel:     "user",
			ScopePartition: part,
			OwnerUserID:    &uid,
			Path:           "grant-path.txt",
			StorageRef:     "s3://test/grant",
		},
	}
	created, err := db.CreateOrchestratorArtifact(ctx, row)
	if err != nil {
		t.Fatalf("CreateOrchestratorArtifact: %v", err)
	}
	ok, err := db.HasArtifactReadGrant(ctx, created.ID, other.ID)
	if err != nil || ok {
		t.Fatalf("HasArtifactReadGrant before grant: ok=%v err=%v", ok, err)
	}
	if err := db.GrantArtifactRead(ctx, created.ID, other.ID); err != nil {
		t.Fatalf("GrantArtifactRead: %v", err)
	}
	ok, err = db.HasArtifactReadGrant(ctx, created.ID, other.ID)
	if err != nil || !ok {
		t.Fatalf("HasArtifactReadGrant after grant: ok=%v err=%v", ok, err)
	}
	if err := db.GrantArtifactRead(ctx, created.ID, other.ID); err != nil {
		t.Fatalf("GrantArtifactRead idempotent: %v", err)
	}
}

func TestIntegration_GrantArtifactRead_nilIDs(t *testing.T) {
	db, ctx := integrationDB(t)
	err := db.GrantArtifactRead(ctx, uuid.Nil, uuid.New())
	if err == nil {
		t.Fatal("expected error for nil artifact id")
	}
}

func TestIntegration_HasArtifactReadGrant_nilIDs(t *testing.T) {
	db, ctx := integrationDB(t)
	ok, err := db.HasArtifactReadGrant(ctx, uuid.Nil, uuid.New())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestIntegration_ListOrchestratorArtifacts_limitOffsetCaps(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "orch-list-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	part := "user:" + uid.String()
	row := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel:     "user",
			ScopePartition: part,
			OwnerUserID:    &uid,
			Path:           "list-cap.txt",
			StorageRef:     "s3://test/cap",
		},
	}
	if _, err := db.CreateOrchestratorArtifact(ctx, row); err != nil {
		t.Fatalf("CreateOrchestratorArtifact: %v", err)
	}
	if _, err := db.ListOrchestratorArtifacts(ctx, ListOrchestratorArtifactsParams{
		ScopeLevel:  "user",
		OwnerUserID: &uid,
		Limit:       9999,
		Offset:      -5,
	}); err != nil {
		t.Fatalf("ListOrchestratorArtifacts: %v", err)
	}
}

func TestIntegration_ArtifactIDLists_defaultLimits(t *testing.T) {
	db, ctx := integrationDB(t)
	if _, err := db.ListOrchestratorArtifactIDsMissingChecksum(ctx, 0); err != nil {
		t.Fatalf("ListOrchestratorArtifactIDsMissingChecksum: %v", err)
	}
	if _, err := db.ListOrchestratorArtifactIDsCreatedBefore(ctx, time.Now().UTC(), 0); err != nil {
		t.Fatalf("ListOrchestratorArtifactIDsCreatedBefore: %v", err)
	}
}
