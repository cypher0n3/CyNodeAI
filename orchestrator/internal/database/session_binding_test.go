package database

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestIntegration_SessionBinding_UpsertGetList(t *testing.T) {
	db := tcOpenDB(t, context.Background())
	ctx := context.Background()

	u := uuid.New()
	sess := uuid.New()
	th := uuid.New()
	lineage := models.SessionBindingLineage{UserID: u, SessionID: sess, ThreadID: &th}
	key := models.DeriveSessionBindingKey(lineage)

	got, err := db.UpsertSessionBinding(ctx, lineage, "svc-pma-1", models.SessionBindingStateActive)
	if err != nil {
		t.Fatalf("UpsertSessionBinding: %v", err)
	}
	if got.BindingKey != key {
		t.Fatalf("binding key: %q vs %q", got.BindingKey, key)
	}

	byKey, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatalf("GetSessionBindingByKey: %v", err)
	}
	if byKey.ServiceID != "svc-pma-1" {
		t.Fatalf("service_id: %q", byKey.ServiceID)
	}

	list, err := db.ListActiveBindingsForUser(ctx, u)
	if err != nil {
		t.Fatalf("ListActiveBindingsForUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len: %d", len(list))
	}

	_, err = db.UpsertSessionBinding(ctx, lineage, "svc-pma-2", models.SessionBindingStateActive)
	if err != nil {
		t.Fatalf("UpsertSessionBinding update: %v", err)
	}
	updated, err := db.GetSessionBindingByKey(ctx, key)
	if err != nil {
		t.Fatalf("GetSessionBindingByKey after update: %v", err)
	}
	if updated.ServiceID != "svc-pma-2" {
		t.Fatalf("updated service_id: %q", updated.ServiceID)
	}

	_, err = db.UpsertSessionBinding(ctx, lineage, "svc-pma-2", models.SessionBindingStateTeardownPending)
	if err != nil {
		t.Fatalf("UpsertSessionBinding teardown state: %v", err)
	}
	list2, err := db.ListActiveBindingsForUser(ctx, u)
	if err != nil {
		t.Fatalf("ListActiveBindingsForUser after teardown-pending: %v", err)
	}
	if len(list2) != 0 {
		t.Fatalf("expected no active bindings, got %d", len(list2))
	}
}
