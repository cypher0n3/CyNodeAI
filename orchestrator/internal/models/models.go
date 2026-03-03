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
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	NodeSlug             string     `gorm:"column:node_slug;uniqueIndex" json:"node_slug"`
	Status               string     `gorm:"column:status;index" json:"status"`
	CapabilityHash       *string    `gorm:"column:capability_hash" json:"capability_hash,omitempty"`
	ConfigVersion        *string    `gorm:"column:config_version" json:"config_version,omitempty"`
	WorkerAPITargetURL   *string    `gorm:"column:worker_api_target_url" json:"worker_api_target_url,omitempty"`
	WorkerAPIBearerToken *string    `gorm:"column:worker_api_bearer_token" json:"-"`
	ConfigAckAt          *time.Time `gorm:"column:config_ack_at" json:"config_ack_at,omitempty"`
	ConfigAckStatus      *string    `gorm:"column:config_ack_status" json:"config_ack_status,omitempty"`
	ConfigAckError       *string    `gorm:"column:config_ack_error" json:"config_ack_error,omitempty"`
	LastSeenAt           *time.Time `gorm:"column:last_seen_at" json:"last_seen_at,omitempty"`
	LastCapabilityAt     *time.Time `gorm:"column:last_capability_at" json:"last_capability_at,omitempty"`
	Metadata             *string    `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
	CreatedAt            time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at" json:"updated_at"`
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

// McpToolCallAuditLog stores metadata for each MCP tool call routed by the gateway.
// Per docs/tech_specs/mcp_tool_call_auditing.md and postgres_schema.md; tool args/results not stored for MVP.
type McpToolCallAuditLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt   time.Time  `gorm:"column:created_at;index" json:"created_at"`
	TaskID      *uuid.UUID `gorm:"column:task_id;index" json:"task_id,omitempty"`
	ProjectID   *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	RunID       *uuid.UUID `gorm:"column:run_id" json:"run_id,omitempty"`
	JobID       *uuid.UUID `gorm:"column:job_id" json:"job_id,omitempty"`
	SubjectType *string    `gorm:"column:subject_type" json:"subject_type,omitempty"`
	SubjectID   *uuid.UUID `gorm:"column:subject_id" json:"subject_id,omitempty"`
	UserID      *uuid.UUID `gorm:"column:user_id;index" json:"user_id,omitempty"`
	GroupIDs    *string    `gorm:"column:group_ids;type:jsonb" json:"group_ids,omitempty"`
	RoleNames   *string    `gorm:"column:role_names;type:jsonb" json:"role_names,omitempty"`
	ToolName    string     `gorm:"column:tool_name;index" json:"tool_name"`
	Decision    string     `gorm:"column:decision" json:"decision"` // allow or deny
	Status      string     `gorm:"column:status" json:"status"`     // success or error
	DurationMs  *int       `gorm:"column:duration_ms" json:"duration_ms,omitempty"`
	ErrorType   *string    `gorm:"column:error_type" json:"error_type,omitempty"`
}

func (McpToolCallAuditLog) TableName() string { return "mcp_tool_call_audit_log" }

// PreferenceEntry stores a single preference in a scope (system, user, group, project, task).
// Per docs/tech_specs/postgres_schema.md and user_preferences.md.
// Unique on (scope_type, scope_id, key).
type PreferenceEntry struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ScopeType string     `gorm:"column:scope_type;uniqueIndex:uix_pref_scope_key,priority:1" json:"scope_type"`
	ScopeID   *uuid.UUID `gorm:"column:scope_id;uniqueIndex:uix_pref_scope_key,priority:2" json:"scope_id,omitempty"`
	Key       string     `gorm:"column:key;uniqueIndex:uix_pref_scope_key,priority:3;index" json:"key"`
	Value     *string    `gorm:"column:value;type:jsonb" json:"value,omitempty"`
	ValueType string     `gorm:"column:value_type" json:"value_type"`
	Version   int        `gorm:"column:version" json:"version"`
	UpdatedAt time.Time  `gorm:"column:updated_at;index" json:"updated_at"`
	UpdatedBy *string    `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

func (PreferenceEntry) TableName() string { return "preference_entries" }

// PreferenceAuditLog records preference entry change history per postgres_schema.md.
type PreferenceAuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	EntryID   uuid.UUID `gorm:"column:entry_id;index" json:"entry_id"`
	OldValue  *string   `gorm:"column:old_value;type:jsonb" json:"old_value,omitempty"`
	NewValue  *string   `gorm:"column:new_value;type:jsonb" json:"new_value,omitempty"`
	ChangedAt time.Time `gorm:"column:changed_at;index" json:"changed_at"`
	ChangedBy *string   `gorm:"column:changed_by" json:"changed_by,omitempty"`
	Reason    *string   `gorm:"column:reason" json:"reason,omitempty"`
}

func (PreferenceAuditLog) TableName() string { return "preference_audit_log" }

// Project represents a workspace boundary (see postgres_schema.md Projects).
type Project struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Slug        string    `gorm:"column:slug;uniqueIndex" json:"slug"`
	DisplayName string    `gorm:"column:display_name" json:"display_name"`
	Description *string   `gorm:"column:description" json:"description,omitempty"`
	IsActive    bool      `gorm:"column:is_active;index" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
	UpdatedBy   *string   `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

func (Project) TableName() string { return "projects" }

// Session represents a user session (see postgres_schema.md Sessions).
type Session struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ParentSessionID *uuid.UUID `gorm:"column:parent_session_id;index" json:"parent_session_id,omitempty"`
	UserID          uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	Title           *string    `gorm:"column:title" json:"title,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Session) TableName() string { return "sessions" }

// ChatThread represents a chat conversation container (see postgres_schema.md Chat Threads).
type ChatThread struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	ProjectID *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	SessionID *uuid.UUID `gorm:"column:session_id;index" json:"session_id,omitempty"`
	Title     *string    `gorm:"column:title" json:"title,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ChatThread) TableName() string { return "chat_threads" }

// ChatMessage represents one message in a thread (see postgres_schema.md Chat Messages).
type ChatMessage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ThreadID  uuid.UUID `gorm:"column:thread_id;index" json:"thread_id"`
	Role      string    `gorm:"column:role" json:"role"`
	Content   string    `gorm:"column:content" json:"content"`
	Metadata  *string   `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// ChatAuditLog records chat completion request audit (redaction, outcome) per openai_compatible_chat_api.md.
type ChatAuditLog struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt        time.Time  `gorm:"column:created_at;index" json:"created_at"`
	UserID           *uuid.UUID `gorm:"column:user_id;index" json:"user_id,omitempty"`
	ProjectID        *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	Outcome          string     `gorm:"column:outcome" json:"outcome"`
	ErrorCode        *string    `gorm:"column:error_code" json:"error_code,omitempty"`
	RedactionApplied bool       `gorm:"column:redaction_applied" json:"redaction_applied"`
	RedactionKinds   *string    `gorm:"column:redaction_kinds;type:jsonb" json:"redaction_kinds,omitempty"`
	DurationMs       *int       `gorm:"column:duration_ms" json:"duration_ms,omitempty"`
	RequestID        *string    `gorm:"column:request_id" json:"request_id,omitempty"`
}

func (ChatAuditLog) TableName() string { return "chat_audit_log" }

// WorkflowCheckpoint stores the current LangGraph checkpoint state per task.
// Per docs/tech_specs/postgres_schema.md and langgraph_mvp.md; one row per task (upsert by task_id).
type WorkflowCheckpoint struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID     uuid.UUID `gorm:"column:task_id;uniqueIndex;not null" json:"task_id"`
	State      *string   `gorm:"column:state;type:jsonb" json:"state,omitempty"`
	LastNodeID string    `gorm:"column:last_node_id" json:"last_node_id,omitempty"`
	UpdatedAt  time.Time `gorm:"column:updated_at;index" json:"updated_at"`
}

func (WorkflowCheckpoint) TableName() string { return "workflow_checkpoints" }

// TaskWorkflowLease enforces one active workflow per task; held by workflow runner.
// Per docs/tech_specs/postgres_schema.md and langgraph_mvp.md.
type TaskWorkflowLease struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID    uuid.UUID  `gorm:"column:task_id;uniqueIndex;not null" json:"task_id"`
	LeaseID   uuid.UUID  `gorm:"column:lease_id;not null" json:"lease_id"`
	HolderID  *string    `gorm:"column:holder_id" json:"holder_id,omitempty"`
	ExpiresAt *time.Time `gorm:"column:expires_at;index" json:"expires_at,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (TaskWorkflowLease) TableName() string { return "task_workflow_leases" }

// SandboxImage is a logical sandbox image (e.g. python-tools, node-build). Per postgres_schema.md Sandbox Image Registry.
type SandboxImage struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"column:name;uniqueIndex" json:"name"`
	Description *string   `gorm:"column:description" json:"description,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
	UpdatedBy   *string   `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

func (SandboxImage) TableName() string { return "sandbox_images" }

// SandboxImageVersion is a version/tag of a sandbox image with OCI reference. Per postgres_schema.md.
type SandboxImageVersion struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SandboxImageID uuid.UUID `gorm:"column:sandbox_image_id;uniqueIndex:uix_sandbox_image_version,priority:1;not null" json:"sandbox_image_id"`
	Version        string    `gorm:"column:version;uniqueIndex:uix_sandbox_image_version,priority:2" json:"version"`
	ImageRef       string    `gorm:"column:image_ref" json:"image_ref"`
	ImageDigest    *string   `gorm:"column:image_digest" json:"image_digest,omitempty"`
	Capabilities   *string   `gorm:"column:capabilities;type:jsonb" json:"capabilities,omitempty"`
	IsAllowed      bool      `gorm:"column:is_allowed;index" json:"is_allowed"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SandboxImageVersion) TableName() string { return "sandbox_image_versions" }

// NodeSandboxImageAvailability records whether a node has a sandbox image version available. Per postgres_schema.md.
type NodeSandboxImageAvailability struct {
	ID                    uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	NodeID                uuid.UUID `gorm:"column:node_id;uniqueIndex:uix_node_sandbox_avail,priority:1;not null" json:"node_id"`
	SandboxImageVersionID uuid.UUID `gorm:"column:sandbox_image_version_id;uniqueIndex:uix_node_sandbox_avail,priority:2;not null" json:"sandbox_image_version_id"`
	Status                string    `gorm:"column:status" json:"status"`
	LastCheckedAt         time.Time `gorm:"column:last_checked_at" json:"last_checked_at"`
	Details               *string   `gorm:"column:details;type:jsonb" json:"details,omitempty"`
}

func (NodeSandboxImageAvailability) TableName() string { return "node_sandbox_image_availability" }

// TaskArtifact stores artifact metadata for a task (path, storage ref, size). Per postgres_schema.md Task Artifacts.
// Unique on (task_id, path). Content may be inline in storage_ref or in object storage.
type TaskArtifact struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID       uuid.UUID  `gorm:"column:task_id;uniqueIndex:uix_task_artifact_path,priority:1;not null" json:"task_id"`
	RunID        *uuid.UUID `gorm:"column:run_id;index" json:"run_id,omitempty"`
	Path         string     `gorm:"column:path;uniqueIndex:uix_task_artifact_path,priority:2" json:"path"`
	StorageRef   string     `gorm:"column:storage_ref" json:"storage_ref"`
	SizeBytes    *int64     `gorm:"column:size_bytes" json:"size_bytes,omitempty"`
	ContentType  *string    `gorm:"column:content_type" json:"content_type,omitempty"`
	ChecksumSHA256 *string  `gorm:"column:checksum_sha256" json:"checksum_sha256,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (TaskArtifact) TableName() string { return "task_artifacts" }

// Skill stores AI skill content and metadata. Per docs/tech_specs/skills_storage_and_inference.md.
// Stable id for retrieval; scope: user | group | project | global; default user. is_system true for default CyNodeAI skill.
type Skill struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string     `gorm:"column:name;index" json:"name"`
	Content   string     `gorm:"column:content;type:text" json:"content"`
	Scope     string     `gorm:"column:scope;index" json:"scope"` // user, group, project, global
	OwnerID   *uuid.UUID `gorm:"column:owner_id;index" json:"owner_id,omitempty"`
	IsSystem  bool       `gorm:"column:is_system;index" json:"is_system"`
	CreatedAt time.Time  `gorm:"column:created_at;index" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;index" json:"updated_at"`
}

func (Skill) TableName() string { return "skills" }
