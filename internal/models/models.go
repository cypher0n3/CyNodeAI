// Package models defines the domain models for CyNodeAI.
// See docs/tech_specs/postgres_schema.md for schema details.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a system user.
type User struct {
	ID             uuid.UUID `json:"id"`
	Handle         string    `json:"handle"`
	Email          *string   `json:"email,omitempty"`
	IsActive       bool      `json:"is_active"`
	ExternalSource *string   `json:"external_source,omitempty"`
	ExternalID     *string   `json:"external_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PasswordCredential stores hashed password for a user.
type PasswordCredential struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	PasswordHash []byte    `json:"-"`
	HashAlg      string    `json:"hash_alg"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RefreshSession represents an active refresh token session.
type RefreshSession struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	RefreshTokenHash []byte     `json:"-"`
	RefreshTokenKID  *string    `json:"-"`
	IsActive         bool       `json:"is_active"`
	ExpiresAt        time.Time  `json:"expires_at"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// AuthAuditLog records authentication events.
type AuthAuditLog struct {
	ID        uuid.UUID  `json:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	EventType string     `json:"event_type"`
	Success   bool       `json:"success"`
	IPAddress *string    `json:"ip_address,omitempty"`
	UserAgent *string    `json:"user_agent,omitempty"`
	Details   *string    `json:"details,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Node represents a registered worker node.
type Node struct {
	ID               uuid.UUID  `json:"id"`
	NodeSlug         string     `json:"node_slug"`
	Status           string     `json:"status"`
	CapabilityHash   *string    `json:"capability_hash,omitempty"`
	ConfigVersion    *string    `json:"config_version,omitempty"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
	LastCapabilityAt *time.Time `json:"last_capability_at,omitempty"`
	Metadata         *string    `json:"metadata,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// NodeCapability stores a snapshot of node capabilities.
type NodeCapability struct {
	ID                 uuid.UUID `json:"id"`
	NodeID             uuid.UUID `json:"node_id"`
	ReportedAt         time.Time `json:"reported_at"`
	CapabilitySnapshot string    `json:"capability_snapshot"`
}

// Task represents a user-submitted task.
type Task struct {
	ID                 uuid.UUID  `json:"id"`
	CreatedBy          *uuid.UUID `json:"created_by,omitempty"`
	ProjectID          *uuid.UUID `json:"project_id,omitempty"`
	Status             string     `json:"status"`
	Prompt             *string    `json:"prompt,omitempty"`
	AcceptanceCriteria *string    `json:"acceptance_criteria,omitempty"`
	Summary            *string    `json:"summary,omitempty"`
	Metadata           *string    `json:"metadata,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Job represents a unit of work dispatched to a node.
type Job struct {
	ID             uuid.UUID  `json:"id"`
	TaskID         uuid.UUID  `json:"task_id"`
	NodeID         *uuid.UUID `json:"node_id,omitempty"`
	Status         string     `json:"status"`
	Payload        *string    `json:"payload,omitempty"`
	Result         *string    `json:"result,omitempty"`
	LeaseID        *uuid.UUID `json:"lease_id,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TaskStatus constants.
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

// JobStatus constants.
const (
	JobStatusQueued       = "queued"
	JobStatusRunning      = "running"
	JobStatusCompleted    = "completed"
	JobStatusFailed       = "failed"
	JobStatusCancelled    = "cancelled"
	JobStatusLeaseExpired = "lease_expired"
)

// NodeStatus constants.
const (
	NodeStatusRegistered = "registered"
	NodeStatusActive     = "active"
	NodeStatusInactive   = "inactive"
	NodeStatusDrained    = "drained"
)
