package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateNode creates a new node.
func (db *DB) CreateNode(ctx context.Context, nodeSlug string) (*models.Node, error) {
	record := &NodeRecord{
		GormModelUUID: newGormModelUUIDNow(),
		NodeBase: models.NodeBase{
			NodeSlug: nodeSlug,
			Status:   models.NodeStatusRegistered,
		},
	}
	if err := db.createRecord(ctx, record, "create node"); err != nil {
		return nil, err
	}
	return record.ToNode(), nil
}

// GetNodeBySlug retrieves a node by slug.
func (db *DB) GetNodeBySlug(ctx context.Context, nodeSlug string) (*models.Node, error) {
	return getDomainWhere(db, ctx, "node_slug", nodeSlug, "get node by slug", (*NodeRecord).ToNode)
}

// GetNodeByID retrieves a node by ID.
func (db *DB) GetNodeByID(ctx context.Context, id uuid.UUID) (*models.Node, error) {
	return getDomainByID(db, ctx, id, "get node by id", (*NodeRecord).ToNode)
}

// UpdateNodeStatus updates a node's status.
func (db *DB) UpdateNodeStatus(ctx context.Context, nodeID uuid.UUID, status string) error {
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID,
		map[string]interface{}{"status": status}, "update node status")
}

// UpdateNodeLastSeen updates a node's last seen timestamp.
func (db *DB) UpdateNodeLastSeen(ctx context.Context, nodeID uuid.UUID) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID,
		map[string]interface{}{"last_seen_at": now}, "update node last seen")
}

// UpdateNodeCapability updates a node's capability hash and timestamp.
func (db *DB) UpdateNodeCapability(ctx context.Context, nodeID uuid.UUID, capHash string) error {
	now := time.Now().UTC()
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID,
		map[string]interface{}{"capability_hash": capHash, "last_capability_at": now}, "update node capability")
}

// ListActiveNodes lists all active nodes.
func (db *DB) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	var records []*NodeRecord
	err := db.db.WithContext(ctx).Where("status = ?", models.NodeStatusActive).Find(&records).Error
	if err != nil {
		return nil, wrapErr(err, "list active nodes")
	}
	nodes := make([]*models.Node, len(records))
	for i, r := range records {
		nodes[i] = r.ToNode()
	}
	return nodes, nil
}

// ListDispatchableNodes lists active nodes that have acknowledged config and have Worker API URL and token set.
func (db *DB) ListDispatchableNodes(ctx context.Context) ([]*models.Node, error) {
	var records []*NodeRecord
	err := db.db.WithContext(ctx).
		Where("status = ?", models.NodeStatusActive).
		Where("config_ack_status = ?", "applied").
		Where("worker_api_target_url IS NOT NULL AND worker_api_target_url != ''").
		Where("worker_api_bearer_token IS NOT NULL AND worker_api_bearer_token != ''").
		Find(&records).Error
	if err != nil {
		return nil, wrapErr(err, "list dispatchable nodes")
	}
	nodes := make([]*models.Node, len(records))
	for i, r := range records {
		nodes[i] = r.ToNode()
	}
	return nodes, nil
}

// SaveNodeCapabilitySnapshot saves a capability snapshot for a node.
func (db *DB) SaveNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID, snapshot string) error {
	record := &NodeCapabilityRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		NodeCapabilityBase: models.NodeCapabilityBase{
			NodeID:             nodeID,
			ReportedAt:         time.Now().UTC(),
			CapabilitySnapshot: snapshot,
		},
	}
	return db.createRecord(ctx, record, "save node capability snapshot")
}

// GetLatestNodeCapabilitySnapshot returns the most recent capability snapshot JSON for the node, or ErrNotFound.
func (db *DB) GetLatestNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID) (string, error) {
	var record NodeCapabilityRecord
	err := db.db.WithContext(ctx).Where("node_id = ?", nodeID).Order("reported_at DESC").First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", wrapErr(err, "get latest node capability snapshot")
	}
	return record.CapabilitySnapshot, nil
}

// UpdateNodeConfigVersion sets the node's config_version.
func (db *DB) UpdateNodeConfigVersion(ctx context.Context, nodeID uuid.UUID, configVersion string) error {
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID,
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
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID, updates, "update node config ack")
}

// UpdateNodeWorkerAPIConfig stores the Worker API target URL and bearer token delivered to the node (for dispatch).
func (db *DB) UpdateNodeWorkerAPIConfig(ctx context.Context, nodeID uuid.UUID, targetURL, bearerToken string) error {
	return db.updateWhere(ctx, &NodeRecord{}, "id", nodeID,
		map[string]interface{}{
			"worker_api_target_url":   targetURL,
			"worker_api_bearer_token": bearerToken,
		}, "update node worker api config")
}
