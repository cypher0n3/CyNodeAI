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

// UserBase is the domain base struct for User (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in UserRecord along with GormModelUUID.
// For API/handler consumption, use User (which includes ID, CreatedAt, UpdatedAt).
type UserBase struct {
	Handle         string  `gorm:"column:handle;uniqueIndex" json:"handle"`
	Email          *string `gorm:"column:email;uniqueIndex" json:"email,omitempty"`
	IsActive       bool    `gorm:"column:is_active;index" json:"is_active"`
	ExternalSource *string `gorm:"column:external_source" json:"external_source,omitempty"`
	ExternalID     *string `gorm:"column:external_id" json:"external_id,omitempty"`
}

// User represents a system user (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use UserRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by UserRecord.ToUser() from GormModelUUID.
type User struct {
	UserBase
	// Identity/timestamps (populated from GormModelUUID by ToUser())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PasswordCredentialBase is the domain base struct for PasswordCredential (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in PasswordCredentialRecord along with GormModelUUID.
// For API/handler consumption, use PasswordCredential (which includes ID, CreatedAt, UpdatedAt).
type PasswordCredentialBase struct {
	UserID       uuid.UUID `gorm:"column:user_id;uniqueIndex" json:"user_id"`
	PasswordHash []byte    `gorm:"column:password_hash;type:bytea" json:"-"`
	HashAlg      string    `gorm:"column:hash_alg" json:"hash_alg"`
}

// PasswordCredential represents a password credential (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use PasswordCredentialRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by PasswordCredentialRecord.ToPasswordCredential() from GormModelUUID.
type PasswordCredential struct {
	PasswordCredentialBase
	// Identity/timestamps (populated from GormModelUUID by ToPasswordCredential())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RefreshSessionBase is the domain base struct for RefreshSession (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in RefreshSessionRecord along with GormModelUUID.
// For API/handler consumption, use RefreshSession (which includes ID, CreatedAt, UpdatedAt).
type RefreshSessionBase struct {
	UserID           uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	RefreshTokenHash []byte     `gorm:"column:refresh_token_hash;type:bytea" json:"-"`
	RefreshTokenKID  *string    `gorm:"column:refresh_token_kid" json:"-"`
	IsActive         bool       `gorm:"column:is_active" json:"is_active"`
	ExpiresAt        time.Time  `gorm:"column:expires_at" json:"expires_at"`
	LastUsedAt       *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
}

// RefreshSession represents an active refresh token session (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use RefreshSessionRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by RefreshSessionRecord.ToRefreshSession() from GormModelUUID.
type RefreshSession struct {
	RefreshSessionBase
	// Identity/timestamps (populated from GormModelUUID by ToRefreshSession())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthAuditLogBase is the domain base struct for AuthAuditLog (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in AuthAuditLogRecord along with GormModelUUID.
// For API/handler consumption, use AuthAuditLog (which includes ID, CreatedAt).
// Note: AuthAuditLog only uses CreatedAt (not UpdatedAt), but GormModelUUID includes UpdatedAt for consistency.
type AuthAuditLogBase struct {
	UserID        *uuid.UUID `gorm:"column:user_id;index" json:"user_id,omitempty"`
	EventType     string     `gorm:"column:event_type;index" json:"event_type"`
	Success       bool       `gorm:"column:success" json:"success"`
	SubjectHandle *string    `gorm:"column:subject_handle" json:"subject_handle,omitempty"`
	IPAddress     *string    `gorm:"column:ip_address" json:"ip_address,omitempty"`
	UserAgent     *string    `gorm:"column:user_agent" json:"user_agent,omitempty"`
	Reason        *string    `gorm:"column:reason" json:"reason,omitempty"`
}

// AuthAuditLog records authentication events (domain type for API/handler consumption).
// Schema: subject_handle, reason, ip_address (inet); no details column.
// This is the type returned from Store methods.
// For GORM persistence, use AuthAuditLogRecord in the database package.
// ID, CreatedAt are populated by AuthAuditLogRecord.ToAuthAuditLog() from GormModelUUID.
type AuthAuditLog struct {
	AuthAuditLogBase
	// Identity/timestamps (populated from GormModelUUID by ToAuthAuditLog())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// NodeBase is the domain base struct for Node (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in NodeRecord along with GormModelUUID.
// For API/handler consumption, use Node (which includes ID, CreatedAt, UpdatedAt).
type NodeBase struct {
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
}

// Node represents a registered worker node (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use NodeRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by NodeRecord.ToNode() from GormModelUUID.
type Node struct {
	NodeBase
	// Identity/timestamps (populated from GormModelUUID by ToNode())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeCapabilityBase is the domain base struct for NodeCapability (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in NodeCapabilityRecord along with GormModelUUID.
// For API/handler consumption, use NodeCapability (which includes ID, CreatedAt, UpdatedAt).
// Note: NodeCapability has ReportedAt which is a domain field, not a timestamp from GormModelUUID.
type NodeCapabilityBase struct {
	NodeID             uuid.UUID `gorm:"column:node_id;index" json:"node_id"`
	ReportedAt         time.Time `gorm:"column:reported_at" json:"reported_at"`
	CapabilitySnapshot string    `gorm:"column:capability_snapshot;type:jsonb" json:"capability_snapshot"`
}

// NodeCapability stores a snapshot of node capabilities (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use NodeCapabilityRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by NodeCapabilityRecord.ToNodeCapability() from GormModelUUID.
type NodeCapability struct {
	NodeCapabilityBase
	// Identity/timestamps (populated from GormModelUUID by ToNodeCapability())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskBase is the domain base struct for Task (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in TaskRecord along with GormModelUUID.
// For API/handler consumption, use Task (which includes ID, CreatedAt, UpdatedAt).
type TaskBase struct {
	CreatedBy          *uuid.UUID `gorm:"column:created_by;index" json:"created_by,omitempty"`
	ProjectID          *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	PlanID             *uuid.UUID `gorm:"column:plan_id;index" json:"plan_id,omitempty"`
	Status             string     `gorm:"column:status;index" json:"status"`
	Closed             bool       `gorm:"column:closed;index" json:"closed"`
	Prompt             *string    `gorm:"column:prompt" json:"prompt,omitempty"`
	AcceptanceCriteria *string    `gorm:"column:acceptance_criteria;type:jsonb" json:"acceptance_criteria,omitempty"`
	Summary            *string    `gorm:"column:summary" json:"summary,omitempty"`
	Metadata           *string    `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
}

// Task represents a user-submitted task (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use TaskRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by TaskRecord.ToTask() from GormModelUUID.
type Task struct {
	TaskBase
	// Identity/timestamps (populated from GormModelUUID by ToTask())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectPlan represents a project plan (postgres_schema.md project_plans). Used by workflow start gate.
// ProjectPlanBase is the domain base struct for project plans (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ProjectPlanRecord along with GormModelUUID.
// For API/handler consumption, use ProjectPlan (which includes ID, CreatedAt, UpdatedAt).
type ProjectPlanBase struct {
	ProjectID uuid.UUID `gorm:"column:project_id;index" json:"project_id"`
	State     string    `gorm:"column:state;index" json:"state"`
	Archived  bool      `gorm:"column:archived;index" json:"archived"`
}

// ProjectPlan represents a project plan (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ProjectPlanRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by ProjectPlanRecord.ToProjectPlan() from GormModelUUID.
type ProjectPlan struct {
	ProjectPlanBase
	// Identity/timestamps (populated from GormModelUUID by ToProjectPlan())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskDependencyBase is the domain base struct for TaskDependency (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in TaskDependencyRecord along with GormModelUUID.
// For API/handler consumption, use TaskDependency (which includes ID, CreatedAt, UpdatedAt).
// Note: TaskDependency currently doesn't use CreatedAt/UpdatedAt in business logic, but GormModelUUID includes them for consistency.
type TaskDependencyBase struct {
	TaskID          uuid.UUID `gorm:"column:task_id;index" json:"task_id"`
	DependsOnTaskID uuid.UUID `gorm:"column:depends_on_task_id;index" json:"depends_on_task_id"`
}

// TaskDependency represents task_dependencies (task_id depends on depends_on_task_id) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use TaskDependencyRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by TaskDependencyRecord.ToTaskDependency() from GormModelUUID.
type TaskDependency struct {
	TaskDependencyBase
	// Identity/timestamps (populated from GormModelUUID by ToTaskDependency())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobBase is the domain base struct for Job (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in JobRecord along with GormModelUUID.
// For API/handler consumption, use Job (which includes ID, CreatedAt, UpdatedAt).
// Payload and Result are stored as jsonb via JSONBString.
type JobBase struct {
	TaskID         uuid.UUID   `gorm:"column:task_id;index" json:"task_id"`
	NodeID         *uuid.UUID  `gorm:"column:node_id;index" json:"node_id,omitempty"`
	Status         string      `gorm:"column:status;index" json:"status"`
	Payload        JSONBString `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Result         JSONBString `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	LeaseID        *uuid.UUID  `gorm:"column:lease_id" json:"lease_id,omitempty"`
	LeaseExpiresAt *time.Time  `gorm:"column:lease_expires_at" json:"lease_expires_at,omitempty"`
	StartedAt      *time.Time  `gorm:"column:started_at" json:"started_at,omitempty"`
	EndedAt        *time.Time  `gorm:"column:ended_at" json:"ended_at,omitempty"`
}

// Job represents a unit of work dispatched to a node (domain type for API/handler consumption).
// Payload and Result are stored as jsonb via JSONBString.
// This is the type returned from Store methods.
// For GORM persistence, use JobRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by JobRecord.ToJob() from GormModelUUID.
type Job struct {
	JobBase
	// Identity/timestamps (populated from GormModelUUID by ToJob())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskStatus constants.
const (
	TaskStatusPending    = "pending"
	TaskStatusRunning    = "running"
	TaskStatusCompleted  = "completed"
	TaskStatusFailed     = "failed"
	TaskStatusCanceled   = "canceled"
	TaskStatusSuperseded = "superseded"
)

// JobStatus constants.
const (
	JobStatusQueued       = "queued"
	JobStatusRunning      = "running"
	JobStatusCompleted    = "completed"
	JobStatusFailed       = "failed"
	JobStatusCanceled     = "canceled"
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
// McpToolCallAuditLogBase is the domain base struct for MCP tool call audit logs (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in McpToolCallAuditLogRecord along with GormModelUUID.
// For API/handler consumption, use McpToolCallAuditLog (which includes ID, CreatedAt).
type McpToolCallAuditLogBase struct {
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

// McpToolCallAuditLog represents an MCP tool call audit log entry (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use McpToolCallAuditLogRecord in the database package.
// ID, CreatedAt are populated by McpToolCallAuditLogRecord.ToMcpToolCallAuditLog() from GormModelUUID.
type McpToolCallAuditLog struct {
	McpToolCallAuditLogBase
	// Identity/timestamps (populated from GormModelUUID by ToMcpToolCallAuditLog())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// PreferenceEntryBase is the domain base struct for preference entries (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// Note: PreferenceEntry uses UpdatedAt but not CreatedAt in the domain type (though GormModelUUID provides CreatedAt).
// This is embedded in PreferenceEntryRecord along with GormModelUUID.
// For API/handler consumption, use PreferenceEntry (which includes ID, UpdatedAt).
type PreferenceEntryBase struct {
	ScopeType string     `gorm:"column:scope_type;uniqueIndex:uix_pref_scope_key,priority:1" json:"scope_type"`
	ScopeID   *uuid.UUID `gorm:"column:scope_id;uniqueIndex:uix_pref_scope_key,priority:2" json:"scope_id,omitempty"`
	Key       string     `gorm:"column:key;uniqueIndex:uix_pref_scope_key,priority:3;index" json:"key"`
	Value     *string    `gorm:"column:value;type:jsonb" json:"value,omitempty"`
	ValueType string     `gorm:"column:value_type" json:"value_type"`
	Version   int        `gorm:"column:version" json:"version"`
	UpdatedBy *string    `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

// PreferenceEntry stores a single preference in a scope (system, user, group, project, task).
// Per docs/tech_specs/postgres_schema.md and user_preferences.md (domain type for API/handler consumption).
// Unique on (scope_type, scope_id, key).
// This is the type returned from Store methods.
// For GORM persistence, use PreferenceEntryRecord in the database package.
// ID, UpdatedAt are populated by PreferenceEntryRecord.ToPreferenceEntry() from GormModelUUID.
type PreferenceEntry struct {
	PreferenceEntryBase
	// Identity/timestamps (populated from GormModelUUID by ToPreferenceEntry())
	ID        uuid.UUID `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SystemSetting is one operator-level system setting (orchestrator_bootstrap.md system_settings table).
// Distinct from preferences; used by system_setting.* MCP tools.
type SystemSetting struct {
	Key       string    `json:"key"`
	Value     *string   `json:"value,omitempty"`
	ValueType string    `json:"value_type"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy *string   `json:"updated_by,omitempty"`
}

// PreferenceAuditLogBase is the domain base struct for preference audit logs (fields without ID, ChangedAt).
// Note: PreferenceAuditLog uses ChangedAt instead of CreatedAt/UpdatedAt.
// This is embedded in PreferenceAuditLogRecord along with GormModelUUID (but ChangedAt is in the base).
// For API/handler consumption, use PreferenceAuditLog (which includes ID, ChangedAt).
type PreferenceAuditLogBase struct {
	EntryID   uuid.UUID `gorm:"column:entry_id;index" json:"entry_id"`
	OldValue  *string   `gorm:"column:old_value;type:jsonb" json:"old_value,omitempty"`
	NewValue  *string   `gorm:"column:new_value;type:jsonb" json:"new_value,omitempty"`
	ChangedAt time.Time `gorm:"column:changed_at;index" json:"changed_at"`
	ChangedBy *string   `gorm:"column:changed_by" json:"changed_by,omitempty"`
	Reason    *string   `gorm:"column:reason" json:"reason,omitempty"`
}

// PreferenceAuditLog records preference entry change history per postgres_schema.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use PreferenceAuditLogRecord in the database package.
// ID is populated by PreferenceAuditLogRecord.ToPreferenceAuditLog() from GormModelUUID.
type PreferenceAuditLog struct {
	PreferenceAuditLogBase
	// Identity (populated from GormModelUUID by ToPreferenceAuditLog())
	ID uuid.UUID `json:"id"`
}

// ProjectBase is the domain base struct for projects (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ProjectRecord along with GormModelUUID.
// For API/handler consumption, use Project (which includes ID, CreatedAt, UpdatedAt).
type ProjectBase struct {
	Slug        string  `gorm:"column:slug;uniqueIndex" json:"slug"`
	DisplayName string  `gorm:"column:display_name" json:"display_name"`
	Description *string `gorm:"column:description" json:"description,omitempty"`
	IsActive    bool    `gorm:"column:is_active;index" json:"is_active"`
	UpdatedBy   *string `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

// Project represents a workspace boundary (see postgres_schema.md Projects) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ProjectRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by ProjectRecord.ToProject() from GormModelUUID.
type Project struct {
	ProjectBase
	// Identity/timestamps (populated from GormModelUUID by ToProject())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionBase is the domain base struct for sessions (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in SessionRecord along with GormModelUUID.
// For API/handler consumption, use Session (which includes ID, CreatedAt, UpdatedAt).
type SessionBase struct {
	ParentSessionID *uuid.UUID `gorm:"column:parent_session_id;index" json:"parent_session_id,omitempty"`
	UserID          uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	Title           *string    `gorm:"column:title" json:"title,omitempty"`
}

// Session represents a user session (see postgres_schema.md Sessions) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use SessionRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by SessionRecord.ToSession() from GormModelUUID.
type Session struct {
	SessionBase
	// Identity/timestamps (populated from GormModelUUID by ToSession())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatThreadBase is the domain base struct for chat threads (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ChatThreadRecord along with GormModelUUID.
// For API/handler consumption, use ChatThread (which includes ID, CreatedAt, UpdatedAt).
type ChatThreadBase struct {
	UserID    uuid.UUID  `gorm:"column:user_id;index" json:"user_id"`
	ProjectID *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	SessionID *uuid.UUID `gorm:"column:session_id;index" json:"session_id,omitempty"`
	Title     *string    `gorm:"column:title" json:"title,omitempty"`
}

// ChatThread represents a chat conversation container (see postgres_schema.md Chat Threads) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ChatThreadRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by ChatThreadRecord.ToChatThread() from GormModelUUID.
type ChatThread struct {
	ChatThreadBase
	// Identity/timestamps (populated from GormModelUUID by ToChatThread())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessageBase is the domain base struct for chat messages (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ChatMessageRecord along with GormModelUUID.
// For API/handler consumption, use ChatMessage (which includes ID, CreatedAt).
type ChatMessageBase struct {
	ThreadID uuid.UUID `gorm:"column:thread_id;index" json:"thread_id"`
	Role     string    `gorm:"column:role" json:"role"`
	Content  string    `gorm:"column:content" json:"content"`
	Metadata *string   `gorm:"column:metadata;type:jsonb" json:"metadata,omitempty"`
}

// ChatMessage represents one message in a thread (see postgres_schema.md Chat Messages) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ChatMessageRecord in the database package.
// ID, CreatedAt are populated by ChatMessageRecord.ToChatMessage() from GormModelUUID.
type ChatMessage struct {
	ChatMessageBase
	// Identity/timestamps (populated from GormModelUUID by ToChatMessage())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatAuditLogBase is the domain base struct for chat audit logs (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ChatAuditLogRecord along with GormModelUUID.
// For API/handler consumption, use ChatAuditLog (which includes ID, CreatedAt).
type ChatAuditLogBase struct {
	UserID           *uuid.UUID `gorm:"column:user_id;index" json:"user_id,omitempty"`
	ProjectID        *uuid.UUID `gorm:"column:project_id;index" json:"project_id,omitempty"`
	Outcome          string     `gorm:"column:outcome" json:"outcome"`
	ErrorCode        *string    `gorm:"column:error_code" json:"error_code,omitempty"`
	RedactionApplied bool       `gorm:"column:redaction_applied" json:"redaction_applied"`
	RedactionKinds   *string    `gorm:"column:redaction_kinds;type:jsonb" json:"redaction_kinds,omitempty"`
	DurationMs       *int       `gorm:"column:duration_ms" json:"duration_ms,omitempty"`
	RequestID        *string    `gorm:"column:request_id" json:"request_id,omitempty"`
}

// ChatAuditLog records chat completion request audit (redaction, outcome) per openai_compatible_chat_api.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ChatAuditLogRecord in the database package.
// ID, CreatedAt are populated by ChatAuditLogRecord.ToChatAuditLog() from GormModelUUID.
type ChatAuditLog struct {
	ChatAuditLogBase
	// Identity/timestamps (populated from GormModelUUID by ToChatAuditLog())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// SetGeneratedAuditIDs sets DB-assigned id and created_at after insert.
func (m *ChatAuditLog) SetGeneratedAuditIDs(id uuid.UUID, createdAt time.Time) {
	m.ID = id
	m.CreatedAt = createdAt
}

// WorkflowCheckpointBase is the domain base struct for workflow checkpoints (fields without ID, UpdatedAt).
// Note: WorkflowCheckpoint uses UpdatedAt but not CreatedAt in the domain type (though GormModelUUID provides CreatedAt).
// This is embedded in WorkflowCheckpointRecord along with GormModelUUID.
// For API/handler consumption, use WorkflowCheckpoint (which includes ID, UpdatedAt).
type WorkflowCheckpointBase struct {
	TaskID     uuid.UUID `gorm:"column:task_id;uniqueIndex;not null" json:"task_id"`
	State      *string   `gorm:"column:state;type:jsonb" json:"state,omitempty"`
	LastNodeID string    `gorm:"column:last_node_id" json:"last_node_id,omitempty"`
}

// WorkflowCheckpoint stores the current LangGraph checkpoint state per task.
// Per docs/tech_specs/postgres_schema.md and langgraph_mvp.md; one row per task (upsert by task_id) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use WorkflowCheckpointRecord in the database package.
// ID, UpdatedAt are populated by WorkflowCheckpointRecord.ToWorkflowCheckpoint() from GormModelUUID.
type WorkflowCheckpoint struct {
	WorkflowCheckpointBase
	// Identity/timestamps (populated from GormModelUUID by ToWorkflowCheckpoint())
	ID        uuid.UUID `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskWorkflowLeaseBase is the domain base struct for task workflow leases (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in TaskWorkflowLeaseRecord along with GormModelUUID.
// For API/handler consumption, use TaskWorkflowLease (which includes ID, CreatedAt, UpdatedAt).
type TaskWorkflowLeaseBase struct {
	TaskID    uuid.UUID  `gorm:"column:task_id;uniqueIndex;not null" json:"task_id"`
	LeaseID   uuid.UUID  `gorm:"column:lease_id;not null" json:"lease_id"`
	HolderID  *string    `gorm:"column:holder_id" json:"holder_id,omitempty"`
	ExpiresAt *time.Time `gorm:"column:expires_at;index" json:"expires_at,omitempty"`
}

// TaskWorkflowLease enforces one active workflow per task; held by workflow runner.
// Per docs/tech_specs/postgres_schema.md and langgraph_mvp.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use TaskWorkflowLeaseRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by TaskWorkflowLeaseRecord.ToTaskWorkflowLease() from GormModelUUID.
type TaskWorkflowLease struct {
	TaskWorkflowLeaseBase
	// Identity/timestamps (populated from GormModelUUID by ToTaskWorkflowLease())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SandboxImageBase is the domain base struct for sandbox images (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in SandboxImageRecord along with GormModelUUID.
// For API/handler consumption, use SandboxImage (which includes ID, CreatedAt, UpdatedAt).
type SandboxImageBase struct {
	Name        string  `gorm:"column:name;uniqueIndex" json:"name"`
	Description *string `gorm:"column:description" json:"description,omitempty"`
	UpdatedBy   *string `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

// SandboxImage is a logical sandbox image (e.g. python-tools, node-build). Per postgres_schema.md Sandbox Image Registry (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use SandboxImageRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by SandboxImageRecord.ToSandboxImage() from GormModelUUID.
type SandboxImage struct {
	SandboxImageBase
	// Identity/timestamps (populated from GormModelUUID by ToSandboxImage())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SandboxImageVersionBase is the domain base struct for sandbox image versions (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in SandboxImageVersionRecord along with GormModelUUID.
// For API/handler consumption, use SandboxImageVersion (which includes ID, CreatedAt, UpdatedAt).
type SandboxImageVersionBase struct {
	SandboxImageID uuid.UUID `gorm:"column:sandbox_image_id;uniqueIndex:uix_sandbox_image_version,priority:1;not null" json:"sandbox_image_id"`
	Version        string    `gorm:"column:version;uniqueIndex:uix_sandbox_image_version,priority:2" json:"version"`
	ImageRef       string    `gorm:"column:image_ref" json:"image_ref"`
	ImageDigest    *string   `gorm:"column:image_digest" json:"image_digest,omitempty"`
	Capabilities   *string   `gorm:"column:capabilities;type:jsonb" json:"capabilities,omitempty"`
	IsAllowed      bool      `gorm:"column:is_allowed;index" json:"is_allowed"`
}

// SandboxImageVersion is a version/tag of a sandbox image with OCI reference. Per postgres_schema.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use SandboxImageVersionRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by SandboxImageVersionRecord.ToSandboxImageVersion() from GormModelUUID.
type SandboxImageVersion struct {
	SandboxImageVersionBase
	// Identity/timestamps (populated from GormModelUUID by ToSandboxImageVersion())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeSandboxImageAvailabilityBase is the domain base struct for node sandbox image availability (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// Note: NodeSandboxImageAvailability uses LastCheckedAt but not CreatedAt/UpdatedAt in the domain type (though GormModelUUID provides CreatedAt/UpdatedAt).
// This is embedded in NodeSandboxImageAvailabilityRecord along with GormModelUUID.
// For API/handler consumption, use NodeSandboxImageAvailability (which includes ID, LastCheckedAt).
type NodeSandboxImageAvailabilityBase struct {
	NodeID                uuid.UUID `gorm:"column:node_id;uniqueIndex:uix_node_sandbox_avail,priority:1;not null" json:"node_id"`
	SandboxImageVersionID uuid.UUID `gorm:"column:sandbox_image_version_id;uniqueIndex:uix_node_sandbox_avail,priority:2;not null" json:"sandbox_image_version_id"`
	Status                string    `gorm:"column:status" json:"status"`
	Details               *string   `gorm:"column:details;type:jsonb" json:"details,omitempty"`
}

// NodeSandboxImageAvailability records whether a node has a sandbox image version available. Per postgres_schema.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use NodeSandboxImageAvailabilityRecord in the database package.
// ID is populated by NodeSandboxImageAvailabilityRecord.ToNodeSandboxImageAvailability() from GormModelUUID.
// LastCheckedAt is a separate field in the domain type (not from GormModelUUID).
type NodeSandboxImageAvailability struct {
	NodeSandboxImageAvailabilityBase
	// Identity (populated from GormModelUUID by ToNodeSandboxImageAvailability())
	ID            uuid.UUID `json:"id"`
	LastCheckedAt time.Time `gorm:"column:last_checked_at" json:"last_checked_at"`
}

// TaskArtifactBase is the domain base struct for task artifacts (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in TaskArtifactRecord along with GormModelUUID.
// For API/handler consumption, use TaskArtifact (which includes ID, CreatedAt, UpdatedAt).
type TaskArtifactBase struct {
	TaskID         uuid.UUID  `gorm:"column:task_id;uniqueIndex:uix_task_artifact_path,priority:1;not null" json:"task_id"`
	RunID          *uuid.UUID `gorm:"column:run_id;index" json:"run_id,omitempty"`
	Path           string     `gorm:"column:path;uniqueIndex:uix_task_artifact_path,priority:2" json:"path"`
	StorageRef     string     `gorm:"column:storage_ref" json:"storage_ref"`
	SizeBytes      *int64     `gorm:"column:size_bytes" json:"size_bytes,omitempty"`
	ContentType    *string    `gorm:"column:content_type" json:"content_type,omitempty"`
	ChecksumSHA256 *string    `gorm:"column:checksum_sha256" json:"checksum_sha256,omitempty"`
}

// TaskArtifact stores artifact metadata for a task (path, storage ref, size). Per postgres_schema.md Task Artifacts.
// Unique on (task_id, path). Content may be inline in storage_ref or in object storage (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use TaskArtifactRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by TaskArtifactRecord.ToTaskArtifact() from GormModelUUID.
type TaskArtifact struct {
	TaskArtifactBase
	// Identity/timestamps (populated from GormModelUUID by ToTaskArtifact())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SkillBase is the domain base struct for skills (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in SkillRecord along with GormModelUUID.
// For API/handler consumption, use Skill (which includes ID, CreatedAt, UpdatedAt).
type SkillBase struct {
	Name     string     `gorm:"column:name;index" json:"name"`
	Content  string     `gorm:"column:content;type:text" json:"content"`
	Scope    string     `gorm:"column:scope;index" json:"scope"` // user, group, project, global
	OwnerID  *uuid.UUID `gorm:"column:owner_id;index" json:"owner_id,omitempty"`
	IsSystem bool       `gorm:"column:is_system;index" json:"is_system"`
}

// Skill stores AI skill content and metadata. Per docs/tech_specs/skills_storage_and_inference.md.
// Stable id for retrieval; scope: user | group | project | global; default user. is_system true for default CyNodeAI skill (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use SkillRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by SkillRecord.ToSkill() from GormModelUUID.
type Skill struct {
	SkillBase
	// Identity/timestamps (populated from GormModelUUID by ToSkill())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccessControlRuleBase is the domain base struct for access control rules (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in AccessControlRuleRecord along with GormModelUUID.
// For API/handler consumption, use AccessControlRule (which includes ID, CreatedAt, UpdatedAt).
type AccessControlRuleBase struct {
	SubjectType     string     `gorm:"column:subject_type;index:idx_ac_rules_subject" json:"subject_type"`
	SubjectID       *uuid.UUID `gorm:"column:subject_id;index:idx_ac_rules_subject" json:"subject_id,omitempty"`
	Action          string     `gorm:"column:action;index" json:"action"`
	ResourceType    string     `gorm:"column:resource_type;index" json:"resource_type"`
	ResourcePattern string     `gorm:"column:resource_pattern" json:"resource_pattern"`
	Effect          string     `gorm:"column:effect" json:"effect"` // allow | deny
	Priority        int        `gorm:"column:priority;index" json:"priority"`
	Conditions      *string    `gorm:"column:conditions;type:jsonb" json:"conditions,omitempty"`
	UpdatedBy       *string    `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

// AccessControlRule defines an allow/deny rule for a subject, action, and resource. Per docs/tech_specs/access_control.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use AccessControlRuleRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by AccessControlRuleRecord.ToAccessControlRule() from GormModelUUID.
type AccessControlRule struct {
	AccessControlRuleBase
	// Identity/timestamps (populated from GormModelUUID by ToAccessControlRule())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccessControlAuditLogBase is the domain base struct for access control audit logs (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in AccessControlAuditLogRecord along with GormModelUUID.
// For API/handler consumption, use AccessControlAuditLog (which includes ID, CreatedAt).
type AccessControlAuditLogBase struct {
	SubjectType  string     `gorm:"column:subject_type" json:"subject_type"`
	SubjectID    *uuid.UUID `gorm:"column:subject_id" json:"subject_id,omitempty"`
	Action       string     `gorm:"column:action" json:"action"`
	ResourceType string     `gorm:"column:resource_type" json:"resource_type"`
	Resource     string     `gorm:"column:resource" json:"resource"`
	Decision     string     `gorm:"column:decision" json:"decision"` // allow | deny
	Reason       *string    `gorm:"column:reason" json:"reason,omitempty"`
	TaskID       *uuid.UUID `gorm:"column:task_id;index" json:"task_id,omitempty"`
}

// AccessControlAuditLog records an access control decision. Per docs/tech_specs/access_control.md (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use AccessControlAuditLogRecord in the database package.
// ID, CreatedAt are populated by AccessControlAuditLogRecord.ToAccessControlAuditLog() from GormModelUUID.
type AccessControlAuditLog struct {
	AccessControlAuditLogBase
	// Identity/timestamps (populated from GormModelUUID by ToAccessControlAuditLog())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// SetGeneratedAuditIDs sets DB-assigned id and created_at after insert.
func (m *AccessControlAuditLog) SetGeneratedAuditIDs(id uuid.UUID, createdAt time.Time) {
	m.ID = id
	m.CreatedAt = createdAt
}

// ApiCredentialBase is the domain base struct for API credentials (fields without ID, CreatedAt, UpdatedAt, DeletedAt).
// This is embedded in ApiCredentialRecord along with GormModelUUID.
// For API/handler consumption, use ApiCredential (which includes ID, CreatedAt, UpdatedAt).
type ApiCredentialBase struct {
	OwnerType            string     `gorm:"column:owner_type;uniqueIndex:idx_api_cred_owner_provider_name" json:"owner_type"` // user | group
	OwnerID              uuid.UUID  `gorm:"column:owner_id;uniqueIndex:idx_api_cred_owner_provider_name" json:"owner_id"`
	Provider             string     `gorm:"column:provider;uniqueIndex:idx_api_cred_owner_provider_name;index" json:"provider"`
	CredentialType       string     `gorm:"column:credential_type" json:"credential_type"`
	CredentialName       string     `gorm:"column:credential_name;uniqueIndex:idx_api_cred_owner_provider_name" json:"credential_name"`
	CredentialCiphertext []byte     `gorm:"column:credential_ciphertext;type:bytea" json:"-"`
	CredentialKID        *string    `gorm:"column:credential_kid" json:"credential_kid,omitempty"`
	IsActive             bool       `gorm:"column:is_active;index" json:"is_active"`
	ExpiresAt            *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	UpdatedBy            *string    `gorm:"column:updated_by" json:"updated_by,omitempty"`
}

// ApiCredential stores an encrypted API credential for egress. Per docs/tech_specs/postgres_schema.md (API Credentials Table) (domain type for API/handler consumption).
// This is the type returned from Store methods.
// For GORM persistence, use ApiCredentialRecord in the database package.
// ID, CreatedAt, UpdatedAt are populated by ApiCredentialRecord.ToApiCredential() from GormModelUUID.
type ApiCredential struct {
	ApiCredentialBase
	// Identity/timestamps (populated from GormModelUUID by ToApiCredential())
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
