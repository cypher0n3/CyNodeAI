package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// --- Node Tests ---

func TestCreateNode(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`INSERT INTO nodes`).
		WithArgs(sqlmock.AnyArg(), "test-node", models.NodeStatusRegistered, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	node, err := db.CreateNode(context.Background(), "test-node")
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	if node.NodeSlug != "test-node" {
		t.Errorf("expected node slug test-node, got %s", node.NodeSlug)
	}
}

func TestGetNodeBySlug(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	nodeID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "node_slug", "status", "capability_hash", "config_version", "last_seen_at", "last_capability_at", "metadata", "created_at", "updated_at"}).
		AddRow(nodeID, "test-node", models.NodeStatusActive, nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM nodes WHERE node_slug`).
		WithArgs("test-node").
		WillReturnRows(rows)

	node, err := db.GetNodeBySlug(context.Background(), "test-node")
	if err != nil {
		t.Fatalf("GetNodeBySlug failed: %v", err)
	}
	if node.ID != nodeID {
		t.Errorf("expected nodeID %v, got %v", nodeID, node.ID)
	}
}

func TestGetNodeByID(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	nodeID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "node_slug", "status", "capability_hash", "config_version", "last_seen_at", "last_capability_at", "metadata", "created_at", "updated_at"}).
		AddRow(nodeID, "test-node", models.NodeStatusActive, nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM nodes WHERE id`).
		WithArgs(nodeID).
		WillReturnRows(rows)

	node, err := db.GetNodeByID(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetNodeByID failed: %v", err)
	}
	if node.NodeSlug != "test-node" {
		t.Errorf("expected node slug test-node, got %s", node.NodeSlug)
	}
}

func TestSaveNodeCapabilitySnapshot(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	nodeID := uuid.New()
	snapshot := `{"version": 1}`
	mock.ExpectExec(`INSERT INTO node_capabilities`).
		WithArgs(sqlmock.AnyArg(), nodeID, sqlmock.AnyArg(), snapshot).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, snapshot)
	if err != nil {
		t.Fatalf("SaveNodeCapabilitySnapshot failed: %v", err)
	}
}

func TestSaveNodeCapabilitySnapshotError(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	nodeID := uuid.New()
	mock.ExpectExec(`INSERT INTO node_capabilities`).
		WillReturnError(errors.New("db error"))

	err := db.SaveNodeCapabilitySnapshot(context.Background(), nodeID, `{"version": 1}`)
	assertError(t, err)
}

func TestListActiveNodes(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	nodeID := uuid.New()
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "node_slug", "status", "capability_hash", "config_version", "last_seen_at", "last_capability_at", "metadata", "created_at", "updated_at"}).
		AddRow(nodeID, "test-node", models.NodeStatusActive, nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(`SELECT .* FROM nodes WHERE status`).
		WithArgs(models.NodeStatusActive).
		WillReturnRows(rows)

	nodes, err := db.ListActiveNodes(context.Background())
	if err != nil {
		t.Fatalf("ListActiveNodes failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}
}

func TestListActiveNodesEmpty(t *testing.T) {
	db, mock := newMockDB(t)
	defer func() { _ = db.Close() }()

	rows := sqlmock.NewRows([]string{"id", "node_slug", "status", "capability_hash", "config_version", "last_seen_at", "last_capability_at", "metadata", "created_at", "updated_at"})

	mock.ExpectQuery(`SELECT .* FROM nodes WHERE status`).
		WithArgs(models.NodeStatusActive).
		WillReturnRows(rows)

	nodes, err := db.ListActiveNodes(context.Background())
	if err != nil {
		t.Fatalf("ListActiveNodes failed: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}
