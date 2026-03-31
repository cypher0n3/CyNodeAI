package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestPmaTeardown_ActiveBindingMarksTeardownPending(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	_, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive)
	if err != nil {
		t.Fatal(err)
	}
	if err := TeardownPMAForInteractiveSession(ctx, db, uid, rsID, "logout", nil); err != nil {
		t.Fatalf("TeardownPMAForInteractiveSession: %v", err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("state %q want teardown_pending", b.State)
	}
	rec := LastPMATeardownForTest()
	if rec == nil || rec.BindingKey != key {
		t.Fatalf("expected teardown record for key %q, got %+v", key, rec)
	}
}

func TestPmaTeardown_NoBindingNoOp(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	if err := TeardownPMAForInteractiveSession(ctx, db, uid, rsID, "logout", nil); err != nil {
		t.Fatal(err)
	}
	if LastPMATeardownForTest() != nil {
		t.Fatal("expected no teardown record")
	}
}

func TestPmaTeardown_AllActiveForUser(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rs1 := uuid.New()
	rs2 := uuid.New()
	for _, rsID := range []uuid.UUID{rs1, rs2} {
		lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
		key := models.DeriveSessionBindingKey(lineage)
		if _, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive); err != nil {
			t.Fatal(err)
		}
	}
	if err := TeardownAllActivePMABindingsForUser(ctx, db, uid, "admin_revoke", nil); err != nil {
		t.Fatal(err)
	}
	for _, rsID := range []uuid.UUID{rs1, rs2} {
		lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
		key := models.DeriveSessionBindingKey(lineage)
		b, err := db.GetSessionBindingByKey(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if b.State != models.SessionBindingStateTeardownPending {
			t.Fatalf("session %s state %q", rsID, b.State)
		}
	}
}

func TestTouchPMABindingActivity_UpdatesLastActivityAt(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	before := time.Now().UTC().Add(-time.Hour)
	_, err := db.UpsertSessionBinding(ctx, lineage, models.PMAServiceIDForBindingKey(key), models.SessionBindingStateActive)
	if err != nil {
		t.Fatal(err)
	}
	// Force an older timestamp so touch visibly advances it.
	db.SessionBindingsByKey[key].LastActivityAt = &before
	db.SessionBindingsByKey[key].UpdatedAt = before

	if err := TouchPMABindingActivity(ctx, db, uid); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.LastActivityAt == nil || !b.LastActivityAt.After(before) {
		t.Fatalf("last activity not updated: before=%v after=%v", before, b.LastActivityAt)
	}
	if !b.UpdatedAt.After(before) {
		t.Fatalf("updated_at not advanced: %v", b.UpdatedAt)
	}
}

func TestScanPMABindingsOnce_IdleTimeout(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	now := time.Now().UTC()
	db.RefreshSessions[rsID] = &models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:    uid,
			IsActive:  true,
			ExpiresAt: now.Add(time.Hour),
		},
		ID:        rsID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	old := now.Add(-2 * time.Hour)
	db.SessionBindingsByKey[key] = &models.SessionBinding{
		SessionBindingBase: models.SessionBindingBase{
			BindingKey:     key,
			UserID:         uid,
			SessionID:      rsID,
			ServiceID:      models.PMAServiceIDForBindingKey(key),
			State:          models.SessionBindingStateActive,
			LastActivityAt: &old,
		},
		ID:        uuid.New(),
		CreatedAt: old,
		UpdatedAt: old,
	}
	if err := scanPMABindingsOnce(ctx, db, 30*time.Minute, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("expected teardown after idle scan, state=%q", b.State)
	}
}

func TestScanPMABindingsOnce_ExpiredOrInactiveRefreshSession(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	now := time.Now().UTC()
	db.RefreshSessions[rsID] = &models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:    uid,
			IsActive:  false,
			ExpiresAt: now.Add(time.Hour),
		},
		ID:        rsID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := putActiveBindingForScanTest(db, uid, rsID, now)
	if err := scanPMABindingsOnce(ctx, db, time.Hour, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("expected teardown when refresh session inactive, state=%q", b.State)
	}
}

func TestScanPMABindingsOnce_ExpiredRefreshToken(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	now := time.Now().UTC()
	db.RefreshSessions[rsID] = &models.RefreshSession{
		RefreshSessionBase: models.RefreshSessionBase{
			UserID:    uid,
			IsActive:  true,
			ExpiresAt: now.Add(-time.Hour),
		},
		ID:        rsID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	key := putActiveBindingForScanTest(db, uid, rsID, now)
	if err := scanPMABindingsOnce(ctx, db, time.Hour, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("expected teardown when refresh token expired, state=%q", b.State)
	}
}

func TestScanPMABindingsOnce_MissingRefreshSession(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	rsID := uuid.New()
	now := time.Now().UTC()
	key := putActiveBindingForScanTest(db, uid, rsID, now)
	if err := scanPMABindingsOnce(ctx, db, time.Hour, nil); err != nil {
		t.Fatal(err)
	}
	b, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != models.SessionBindingStateTeardownPending {
		t.Fatalf("expected teardown when refresh session missing, state=%q", b.State)
	}
}

func TestTeardownPMAForInteractiveSession_NoBindingReturnsNil(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	if err := TeardownPMAForInteractiveSession(ctx, db, uuid.New(), uuid.New(), "test", nil); err != nil {
		t.Fatal(err)
	}
	if LastPMATeardownForTest() != nil {
		t.Fatal("expected no teardown record when binding is absent")
	}
}

func TestTeardownPMAForInteractiveSession_GetBindingError(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ForceError = errors.New("get binding failed")
	if err := TeardownPMAForInteractiveSession(ctx, db, uuid.New(), uuid.New(), "test", nil); err == nil {
		t.Fatal("expected error from GetSessionBindingByKey")
	}
}

func TestTeardownAllActivePMABindingsForUser_ListError(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list bindings failed")
	if err := TeardownAllActivePMABindingsForUser(ctx, db, uuid.New(), "test", nil); err == nil {
		t.Fatal("expected error from ListActiveBindingsForUser")
	}
}

func TestScanPMABindingsOnce_ListAllError(t *testing.T) {
	t.Cleanup(ResetPMATeardownForTest)
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list bindings failed")
	if err := scanPMABindingsOnce(ctx, db, time.Minute, nil); err == nil {
		t.Fatal("expected error from ListAllActiveSessionBindings")
	}
}

func putActiveBindingForScanTest(db *testutil.MockDB, uid, rsID uuid.UUID, now time.Time) string {
	lineage := models.SessionBindingLineage{UserID: uid, SessionID: rsID, ThreadID: nil}
	key := models.DeriveSessionBindingKey(lineage)
	db.SessionBindingsByKey[key] = &models.SessionBinding{
		SessionBindingBase: models.SessionBindingBase{
			BindingKey: key,
			UserID:     uid,
			SessionID:  rsID,
			ServiceID:  models.PMAServiceIDForBindingKey(key),
			State:      models.SessionBindingStateActive,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	return key
}
