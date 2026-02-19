package database

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateNode creates a new node.
func (db *DB) CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error) {
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  nodeSlug,
		Status:    models.NodeStatusRegistered,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return node, db.createRecord(ctx, node, "create node")
}

// GetNodeBySlug retrieves a node by slug.
func (db *DB) GetNodeBySlug(ctx context.Context, nodeSlug string) (*models.Node, error) {
	return getWhere[models.Node](db, ctx, "node_slug", nodeSlug, "get node by slug")
}

// GetNodeByID retrieves a node by ID.
func (db *DB) GetNodeByID(ctx context.Context, id uuid.UUID) (*models.Node, error) {
	return getByID[models.Node](db, ctx, id, "get node by id")
}

// UpdateNodeStatus updates a node's status.
func (db *DB) UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error {
	return db.updateWhere(ctx, &models.Node{}, "id", nodeID,
		map[string]interface{}{"status": status}, "update node status")
}

// UpdateNodeLastSeen updates a node's last seen timestamp.
func (db *DB) UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &models.Node{}, "id", nodeID,
		map[string]interface{}{"last_seen_at": now}, "update node last seen")
}

// UpdateNodeCapability updates a node's capability hash and timestamp.
func (db *DB) UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &models.Node{}, "id", nodeID,
		map[string]interface{}{"capability_hash": capHash, "last_capability_at": now}, "update node capability")
}

// ListActiveNodes lists all active nodes.
func (db *DB) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	var nodes []*models.Node
	err := db.db.WithContext(ctx).Where("status = ?", models.NodeStatusActive).Find(&nodes).Error
	if err != nil {
		return nil, wrapErr(err, "list active nodes")
	}
	return nodes, nil
}

// SaveNodeCapabilitySnapshot saves a capability snapshot for a node.
func (db *DB) SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, snapshot string) error {
	nodeCap := &models.NodeCapability{
		ID:                 uuid.New(),
		NodeID:             nodeID,
		ReportedAt:         time.Now().UTC(),
		CapabilitySnapshot: snapshot,
	}
	return db.createRecord(ctx, nodeCap, "save node capability snapshot")
}

// UpdateNodeConfigVersion sets the node's config_version.
func (db *DB) UpdateNodeConfigVersion(ctx context.Context, nodeID uuid.UUID, configVersion string) error {
	return db.updateWhere(ctx, &models.Node{}, "id", nodeID,
		map[string]interface{}{"config_version": configVersion}, "update node config version")
}

// UpdateNodeConfigAck records the node's configuration acknowledgement.
func (db *DB) UpdateNodeConfigAck(ctx context.Context, nodeID uuid.UUID, configVersion, status string, ackAt time.Time, errMsg *string) error {
	updates := map[string]interface{}{
		"config_ack_at":     ackAt,
		"config_ack_status": status,
	}
	if errMsg != nil {
		updates["config_ack_error"] = *errMsg
	}
	return db.updateWhere(ctx, &models.Node{}, "id", nodeID, updates, "update node config ack")
}
