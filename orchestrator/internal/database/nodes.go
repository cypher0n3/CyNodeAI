package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// --- Node Operations ---

const selectNodeCols = `SELECT id, node_slug, status, capability_hash, config_version, last_seen_at, last_capability_at, metadata, created_at, updated_at FROM nodes`

func nodePtrs(n *models.Node) []any {
	return []any{&n.ID, &n.NodeSlug, &n.Status, &n.CapabilityHash, &n.ConfigVersion,
		&n.LastSeenAt, &n.LastCapabilityAt, &n.Metadata, &n.CreatedAt, &n.UpdatedAt}
}

func scanNodeRow(row *sql.Row) (*models.Node, error) { return scanOne(row, nodePtrs) }

func scanNodeRows(r *sql.Rows) (*models.Node, error) { return scanOne(r, nodePtrs) }

// CreateNode creates a new node.
func (db *DB) CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error) {
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  nodeSlug,
		Status:    models.NodeStatusRegistered,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return execAndReturn(func() error {
		return db.execContext(ctx, "create node",
			`INSERT INTO nodes (id, node_slug, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
			node.ID, node.NodeSlug, node.Status, node.CreatedAt, node.UpdatedAt)
	}, node)
}

// GetNodeBySlug retrieves a node by slug.
func (db *DB) GetNodeBySlug(ctx context.Context, nodeSlug string) (*models.Node, error) {
	return queryRowInto(db, ctx, "get node by slug", selectNodeCols+` WHERE node_slug = $1`, []any{nodeSlug}, scanNodeRow)
}

// GetNodeByID retrieves a node by ID.
func (db *DB) GetNodeByID(ctx context.Context, id uuid.UUID) (*models.Node, error) {
	return queryRowInto(db, ctx, "get node by id", selectNodeCols+` WHERE id = $1`, []any{id}, scanNodeRow)
}

// UpdateNodeStatus updates a node's status.
func (db *DB) UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error {
	return db.execContext(ctx, "update node status",
		`UPDATE nodes SET status = $2, updated_at = $3 WHERE id = $1`,
		nodeID, status, time.Now().UTC())
}

// UpdateNodeLastSeen updates a node's last seen timestamp.
func (db *DB) UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	return db.execContext(ctx, "update node last seen",
		`UPDATE nodes SET last_seen_at = $2, updated_at = $2 WHERE id = $1`, nodeID, now)
}

// UpdateNodeCapability updates a node's capability hash and timestamp.
func (db *DB) UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capabilityHash string) error {
	now := time.Now().UTC()
	return db.execContext(ctx, "update node capability",
		`UPDATE nodes SET capability_hash = $2, last_capability_at = $3, updated_at = $3 WHERE id = $1`,
		nodeID, capabilityHash, now)
}

// ListActiveNodes lists all active nodes.
func (db *DB) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	return queryRows(db, ctx, "list active nodes", selectNodeCols+` WHERE status = $1`, []any{models.NodeStatusActive}, scanNodeRows)
}

// --- Node Capability Operations ---

// SaveNodeCapabilitySnapshot saves a capability snapshot for a node.
func (db *DB) SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, snapshot string) error {
	return db.execContext(ctx, "save node capability snapshot",
		`INSERT INTO node_capabilities (id, node_id, reported_at, capability_snapshot)
		 VALUES ($1, $2, $3, $4)`,
		uuid.New(), nodeID, time.Now().UTC(), snapshot)
}
