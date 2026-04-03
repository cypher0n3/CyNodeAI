package natsutil

// Event versions for Phase 1 session and config notifications.
const (
	EventVersionSessionV1 = "1.0.0"
	EventVersionConfigV1  = "1.0.0"
)

// Producer identifies the emitting service instance (see docs/tech_specs/nats_messaging.md).
type Producer struct {
	Service    string `json:"service"`
	InstanceID string `json:"instance_id"`
}

// Scope carries tenant/project and sensitivity.
type Scope struct {
	TenantID    string  `json:"tenant_id"`
	ProjectID   *string `json:"project_id"`
	Sensitivity string  `json:"sensitivity"`
}

// Correlation links the event to sessions, jobs, or traces.
type Correlation struct {
	SessionID  *string `json:"session_id"`
	WorkItemID *string `json:"work_item_id"`
	JobID      *string `json:"job_id"`
	TraceID    *string `json:"trace_id"`
}

// Envelope is the common NATS message wrapper (docs/tech_specs/nats_messaging.md).
type Envelope struct {
	EventID      string         `json:"event_id"`
	EventType    string         `json:"event_type"`
	EventVersion string         `json:"event_version"`
	OccurredAt   string         `json:"occurred_at"`
	Producer     Producer       `json:"producer"`
	Scope        Scope          `json:"scope"`
	Correlation  Correlation    `json:"correlation"`
	Payload      map[string]any `json:"payload"`
}

// SessionActivityPayloadV1 is the payload for event_type session.activity.
type SessionActivityPayloadV1 struct {
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
	BindingKey string `json:"binding_key"`
	ClientType string `json:"client_type"`
	Ts         string `json:"ts"`
}

// SessionAttachedPayloadV1 is the payload for event_type session.attached.
type SessionAttachedPayloadV1 struct {
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
	BindingKey string `json:"binding_key"`
	ClientType string `json:"client_type"`
	Ts         string `json:"ts"`
}

// SessionDetachedPayloadV1 is the payload for event_type session.detached.
type SessionDetachedPayloadV1 struct {
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
	BindingKey string `json:"binding_key"`
	Reason     string `json:"reason"`
	Ts         string `json:"ts"`
}

// NodeConfigChangedPayloadV1 is the payload for event_type node.config_changed.
type NodeConfigChangedPayloadV1 struct {
	NodeID          string   `json:"node_id"`
	ConfigVersion   string   `json:"config_version"`
	ChangedSections []string `json:"changed_sections,omitempty"`
	Ts              string   `json:"ts"`
}
