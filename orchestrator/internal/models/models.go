// Package models defines the domain models for CyNodeAI.
// See docs/tech_specs/postgres_schema.md for schema details.
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONBString stores a string in a PostgreSQL jsonb column (e.g. jobs.payload, jobs.result).
// It implements sql.Scanner and driver.Valuer so the string is serialized as JSON.
type JSONBString struct{ *string }

// Value implements driver.Valuer for GORM: write the string as a JSON value.
func (j JSONBString) Value() (driver.Value, error) {
	if j.string == nil {
		return nil, nil
	}
	return json.Marshal(*j.string)
}

// Scan implements sql.Scanner: read from jsonb (or json) into the string.
func (j *JSONBString) Scan(value interface{}) error {
	if value == nil {
		j.string = nil
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("JSONBString: unsupported type")
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	j.string = &s
	return nil
}

// NewJSONBString returns a JSONBString holding the given string pointer.
func NewJSONBString(s *string) JSONBString { return JSONBString{string: s} }

// Ptr returns the underlying *string for use by handlers.
func (j JSONBString) Ptr() *string { return j.string }

// MarshalJSON implements json.Marshaler so Job serializes with string payload/result.
func (j JSONBString) MarshalJSON() ([]byte, error) {
	if j.string == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*j.string)
}

// UnmarshalJSON implements json.Unmarshaler for API decoding.
func (j *JSONBString) UnmarshalJSON(data []byte) error {
	if len(data) == 4 && string(data) == "null" {
		j.string = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	j.string = &s
	return nil
}

// User represents a system user.
type User struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Handle         string    `gorm:"column:handle;uniqueIndex" json:"handle"`
	Email          *string   `gorm:"column:email;uniqueIndex" json:"email,omitempty"`
	IsActive       bool      `gorm:"column:is_active;index" json:"is_active"`
	ExternalSource *string   `gorm:"column:external_source" json:"external_source,omitempty"`
	ExternalID     *string   `gorm:"column:external_id" json:"external_id,omitempty"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (User) TableName() string { return "users" }

// PasswordCredential stores hashed password for a user.
type PasswordCredential struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"column:user_id;uniqueIndex" json:"user_id"`
	PasswordHash []byte    `gorm:"column:password_hash;type:bytea" json:"-"`
	HashAlg      string    `gorm:"column:hash_alg" json:"hash_alg"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PasswordCredential) TableName() string { return "password_credentials" }

// RefreshSession represents an active refresh token session.
type RefreshSession struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	RefreshTokenHash []byte     `gorm:"column:refresh_token_hash;type:bytea" json:"-"`
	RefreshTokenKID  *string    `gorm:"column:refresh_token_kid" json:"-"`
	IsActive         bool       `gorm:"column:is_active" json:"is_active"`
	ExpiresAt        time.Time  `gorm:"column:expires_at" json:"expires_at"`
	LastUsedAt       *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (RefreshSession) TableName() string { return "refresh_sessions" }

// AuthAuditLog records authentication events.
// Schema: subject_handle, reason, ip_address (inet); no details column.
type AuthAuditLog struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        *uuid.UUID `gorm:"column:user_id;index" json:"user_id,omitempty"`
	EventType     string     `gorm:"column:event_type;index" json:"event_type"`
	Success       bool       `gorm:"column:success" json:"success"`
	SubjectHandle *string    `gorm:"column:subject_handle" json:"subject_handle,omitempty"`
	IPAddress     *string    `gorm:"column:ip_address" json:"ip_address,omitempty"`
	UserAgent     *string    `gorm:"column:user_agent" json:"user_agent,omitempty"`
	Reason        *string    `gorm:"column:reason" json:"reason,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (AuthAuditLog) TableName() string { return "auth_audit_log" }

// Node represents a registered worker node.
type Node struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	NodeSlug         string     `gorm:"column:node_slug;uniqueIndex" json:"node_slug"`
	Status           string     `gorm:"column:status;index" json:"status"`
	CapabilityHash   *string    `gorm:"column:capability_hash" json:"capability_hash,omitempty"`
	ConfigVersion    *string    `gorm:"column:config_version" json:"config_version,omitempty"`
	LastSeenAt       *time.Time `gorm:"column:last_seen_at" json:"last_seen_at,omitempty"`
	LastCapabilityAt *time.Time `gorm:"column:last_capability_at" json:"last_capability_at,omitempty"`
	Metadata         *string    `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Node) TableName() string { return "nodes" }

// NodeCapability stores a snapshot of node capabilities.
type NodeCapability struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	NodeID             uuid.UUID `gorm:"column:node_id;index" json:"node_id"`
	ReportedAt         time.Time `gorm:"column:reported_at" json:"reported_at"`
	CapabilitySnapshot string    `gorm:"column:capability_snapshot;type:jsonb" json:"capability_snapshot"`
}

func (NodeCapability) TableName() string { return "node_capabilities" }

// Task represents a user-submitted task.
type Task struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedBy          *uuid.UUID `gorm:"column:created_by;index" json:"created_by,omitempty"`
	ProjectID          *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	Status             string     `gorm:"column:status;index" json:"status"`
	Prompt             *string    `gorm:"column:prompt" json:"prompt,omitempty"`
	AcceptanceCriteria *string    `gorm:"column:acceptance_criteria;type:jsonb" json:"acceptance_criteria,omitempty"`
	Summary            *string    `gorm:"column:summary" json:"summary,omitempty"`
	Metadata           *string    `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

// Job represents a unit of work dispatched to a node.
// Payload and Result are stored as jsonb via JSONBString.
type Job struct {
	ID             uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID         uuid.UUID   `gorm:"column:task_id;index" json:"task_id"`
	NodeID         *uuid.UUID  `gorm:"column:node_id;index" json:"node_id,omitempty"`
	Status         string      `gorm:"column:status;index" json:"status"`
	Payload        JSONBString `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Result         JSONBString `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	LeaseID        *uuid.UUID  `gorm:"column:lease_id" json:"lease_id,omitempty"`
	LeaseExpiresAt *time.Time  `gorm:"column:lease_expires_at" json:"lease_expires_at,omitempty"`
	StartedAt      *time.Time  `gorm:"column:started_at" json:"started_at,omitempty"`
	EndedAt        *time.Time  `gorm:"column:ended_at" json:"ended_at,omitempty"`
	CreatedAt      time.Time   `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time   `gorm:"column:updated_at" json:"updated_at"`
}

func (Job) TableName() string { return "jobs" }

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
