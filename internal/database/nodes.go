package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/internal/models"
)

// --- Node Operations ---

// CreateNode creates a new node.
func (db *DB) CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error) {
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  nodeSlug,
		Status:    models.NodeStatusRegistered,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO nodes (id, node_slug, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		node.ID, node.NodeSlug, node.Status, node.CreatedAt, node.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return node, nil
}

// GetNodeBySlug retrieves a node by slug.
func (db *DB) GetNodeBySlug(ctx context.Context, nodeSlug string) (*models.Node, error) {
	node := &models.Node{}
	err := db.QueryRowContext(ctx,
		`SELECT id, node_slug, status, capability_hash, config_version, last_seen_at, last_capability_at, metadata, created_at, updated_at
		 FROM nodes WHERE node_slug = $1`, nodeSlug).Scan(
		&node.ID, &node.NodeSlug, &node.Status, &node.CapabilityHash, &node.ConfigVersion,
		&node.LastSeenAt, &node.LastCapabilityAt, &node.Metadata, &node.CreatedAt, &node.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get node by slug: %w", err)
	}
	return node, nil
}

// GetNodeByID retrieves a node by ID.
func (db *DB) GetNodeByID(ctx context.Context, id uuid.UUID) (*models.Node, error) {
	node := &models.Node{}
	err := db.QueryRowContext(ctx,
		`SELECT id, node_slug, status, capability_hash, config_version, last_seen_at, last_capability_at, metadata, created_at, updated_at
		 FROM nodes WHERE id = $1`, id).Scan(
		&node.ID, &node.NodeSlug, &node.Status, &node.CapabilityHash, &node.ConfigVersion,
		&node.LastSeenAt, &node.LastCapabilityAt, &node.Metadata, &node.CreatedAt, &node.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get node by id: %w", err)
	}
	return node, nil
}

// UpdateNodeStatus updates a node's status.
func (db *DB) UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE nodes SET status = $2, updated_at = $3 WHERE id = $1`,
		nodeID, status, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update node status: %w", err)
	}
	return nil
}

// UpdateNodeLastSeen updates a node's last seen timestamp.
func (db *DB) UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE nodes SET last_seen_at = $2, updated_at = $2 WHERE id = $1`,
		nodeID, now)
	if err != nil {
		return fmt.Errorf("update node last seen: %w", err)
	}
	return nil
}

// UpdateNodeCapability updates a node's capability hash and timestamp.
func (db *DB) UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capabilityHash string) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE nodes SET capability_hash = $2, last_capability_at = $3, updated_at = $3 WHERE id = $1`,
		nodeID, capabilityHash, now)
	if err != nil {
		return fmt.Errorf("update node capability: %w", err)
	}
	return nil
}

// ListActiveNodes lists all active nodes.
func (db *DB) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, node_slug, status, capability_hash, config_version, last_seen_at, last_capability_at, metadata, created_at, updated_at
		 FROM nodes WHERE status = $1`, models.NodeStatusActive)
	if err != nil {
		return nil, fmt.Errorf("list active nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		node := &models.Node{}
		err := rows.Scan(&node.ID, &node.NodeSlug, &node.Status, &node.CapabilityHash, &node.ConfigVersion,
			&node.LastSeenAt, &node.LastCapabilityAt, &node.Metadata, &node.CreatedAt, &node.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// --- Node Capability Operations ---

// SaveNodeCapabilitySnapshot saves a capability snapshot for a node.
func (db *DB) SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, snapshot string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO node_capabilities (id, node_id, reported_at, capability_snapshot)
		 VALUES ($1, $2, $3, $4)`,
		uuid.New(), nodeID, time.Now().UTC(), snapshot)
	if err != nil {
		return fmt.Errorf("save node capability snapshot: %w", err)
	}
	return nil
}
