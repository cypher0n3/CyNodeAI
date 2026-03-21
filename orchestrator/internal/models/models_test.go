package models

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTaskStatusConstants(t *testing.T) {
	statuses := []string{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCanceled,
		TaskStatusSuperseded,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Task status constant is empty")
		}
	}
}

func TestJobStatusConstants(t *testing.T) {
	statuses := []string{
		JobStatusQueued,
		JobStatusRunning,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCanceled,
		JobStatusLeaseExpired,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Job status constant is empty")
		}
	}
}

func TestNodeStatusConstants(t *testing.T) {
	statuses := []string{
		NodeStatusRegistered,
		NodeStatusActive,
		NodeStatusInactive,
		NodeStatusDrained,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("Node status constant is empty")
		}
	}
}

func TestUser_Fields(t *testing.T) {
	user := User{
		UserBase: UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
	}

	if user.Handle != "testuser" {
		t.Errorf("Handle = %s, want testuser", user.Handle)
	}

	if !user.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestTask_Fields(t *testing.T) {
	prompt := "test prompt"
	task := Task{
		TaskBase: TaskBase{
			Status: TaskStatusPending,
			Prompt: &prompt,
		},
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Status = %s, want %s", task.Status, TaskStatusPending)
	}

	if *task.Prompt != "test prompt" {
		t.Errorf("Prompt = %s, want test prompt", *task.Prompt)
	}
}

func TestJob_Fields(t *testing.T) {
	job := Job{
		JobBase: JobBase{
			Status: JobStatusQueued,
		},
	}

	if job.Status != JobStatusQueued {
		t.Errorf("Status = %s, want %s", job.Status, JobStatusQueued)
	}
}

func TestNode_Fields(t *testing.T) {
	node := Node{
		NodeBase: NodeBase{
			NodeSlug: "test-node",
			Status:   NodeStatusRegistered,
		},
	}

	if node.NodeSlug != "test-node" {
		t.Errorf("NodeSlug = %s, want test-node", node.NodeSlug)
	}

	if node.Status != NodeStatusRegistered {
		t.Errorf("Status = %s, want %s", node.Status, NodeStatusRegistered)
	}
}

func TestJSONBString_Value(t *testing.T) {
	var j JSONBString
	v, err := j.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("nil JSONBString should Value() nil, got %v", v)
	}
	s := "hello"
	j = NewJSONBString(&s)
	v, err = j.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v == nil {
		t.Fatal("expected non-nil")
	}
}

func TestJSONBString_Scan(t *testing.T) {
	var j JSONBString
	if err := j.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if j.Ptr() != nil {
		t.Error("Scan(nil) should set nil")
	}
	if err := j.Scan([]byte(`"x"`)); err != nil {
		t.Fatal(err)
	}
	if j.Ptr() == nil || *j.Ptr() != "x" {
		t.Errorf("Scan: %v", j.Ptr())
	}
	// Scan with string type (driver may pass string instead of []byte)
	var j2 JSONBString
	if err := j2.Scan(`"y"`); err != nil {
		t.Fatal(err)
	}
	if j2.Ptr() == nil || *j2.Ptr() != "y" {
		t.Errorf("Scan(string): %v", j2.Ptr())
	}
}

func TestJSONBString_Scan_UnsupportedType(t *testing.T) {
	var j JSONBString
	if err := j.Scan(123); err == nil {
		t.Error("Scan(123) should return error")
	}
}

func TestJSONBString_MarshalJSON_Nil(t *testing.T) {
	var j JSONBString
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "null" {
		t.Errorf("nil JSONBString should marshal to null, got %s", data)
	}
}

func TestJSONBString_UnmarshalJSON_Invalid(t *testing.T) {
	var j JSONBString
	if err := json.Unmarshal([]byte("123"), &j); err == nil {
		t.Error("Unmarshal(123) should return error")
	}
	if err := json.Unmarshal([]byte("{}"), &j); err == nil {
		t.Error("Unmarshal({}) should return error")
	}
}

func TestJSONBString_MarshalUnmarshalJSON(t *testing.T) {
	s := "payload"
	j := NewJSONBString(&s)
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var j2 JSONBString
	if err := json.Unmarshal(data, &j2); err != nil {
		t.Fatal(err)
	}
	if j2.Ptr() == nil || *j2.Ptr() != "payload" {
		t.Errorf("round-trip: %v", j2.Ptr())
	}
	var j3 JSONBString
	if err := json.Unmarshal([]byte("null"), &j3); err != nil {
		t.Fatal(err)
	}
	if j3.Ptr() != nil {
		t.Error("null should unmarshal to nil")
	}
}

func TestJSONBString_Value_implementsDriverValuer(t *testing.T) {
	var _ driver.Valuer = (*JSONBString)(nil)
}

func assertSetGeneratedAuditIDs(t *testing.T, name string, id uuid.UUID, ts time.Time, gotID uuid.UUID, gotAt time.Time) {
	t.Helper()
	if gotID != id || !gotAt.Equal(ts) {
		t.Fatalf("%s.SetGeneratedAuditIDs: id=%v createdAt=%v", name, gotID, gotAt)
	}
}

func TestAuditLogTypes_SetGeneratedAuditIDs(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	ts1 := time.Unix(1, 2).UTC()
	var chat ChatAuditLog
	chat.SetGeneratedAuditIDs(id1, ts1)
	assertSetGeneratedAuditIDs(t, "ChatAuditLog", id1, ts1, chat.ID, chat.CreatedAt)

	id2 := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	ts2 := time.Unix(3, 4).UTC()
	var ac AccessControlAuditLog
	ac.SetGeneratedAuditIDs(id2, ts2)
	assertSetGeneratedAuditIDs(t, "AccessControlAuditLog", id2, ts2, ac.ID, ac.CreatedAt)
}

func TestTableNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		// User no longer has TableName() - it's a domain base struct; TableName() is on UserRecord in database package
		// {"User", (User{}).TableName(), "users"},
		// PasswordCredential, RefreshSession, AuthAuditLog no longer have TableName() - they're domain types; TableName() is on record types in database package
		// {"PasswordCredential", (PasswordCredential{}).TableName(), "password_credentials"},
		// {"RefreshSession", (RefreshSession{}).TableName(), "refresh_sessions"},
		// {"AuthAuditLog", (AuthAuditLog{}).TableName(), "auth_audit_log"},
		// Node, NodeCapability, Task, Job no longer have TableName() - they're domain types; TableName() is on record types in database package
		// {"Node", (Node{}).TableName(), "nodes"},
		// {"NodeCapability", (NodeCapability{}).TableName(), "node_capabilities"},
		// {"Task", (Task{}).TableName(), "tasks"},
		// {"Job", (Job{}).TableName(), "jobs"},
		// TableName() methods are now on record structs in database package
		// {"Project", (Project{}).TableName(), "projects"},
		// {"Session", (Session{}).TableName(), "sessions"},
		// {"ChatThread", (ChatThread{}).TableName(), "chat_threads"},
		// {"ChatMessage", (ChatMessage{}).TableName(), "chat_messages"},
		// TableName() methods are now on record structs in database package
		// {"ChatAuditLog", (ChatAuditLog{}).TableName(), "chat_audit_log"},
		// {"McpToolCallAuditLog", (McpToolCallAuditLog{}).TableName(), "mcp_tool_call_audit_log"},
		// {"PreferenceEntry", (PreferenceEntry{}).TableName(), "preference_entries"},
		// {"PreferenceAuditLog", (PreferenceAuditLog{}).TableName(), "preference_audit_log"},
		// {"WorkflowCheckpoint", (WorkflowCheckpoint{}).TableName(), "workflow_checkpoints"},
		// {"TaskWorkflowLease", (TaskWorkflowLease{}).TableName(), "task_workflow_leases"},
		// TableName() methods are now on record structs in database package
		// {"SandboxImage", (SandboxImage{}).TableName(), "sandbox_images"},
		// {"SandboxImageVersion", (SandboxImageVersion{}).TableName(), "sandbox_image_versions"},
		// {"NodeSandboxImageAvailability", (NodeSandboxImageAvailability{}).TableName(), "node_sandbox_image_availability"},
		// TableName() methods are now on record structs in database package
		// {"Skill", (Skill{}).TableName(), "skills"},
		// {"AccessControlRule", (AccessControlRule{}).TableName(), "access_control_rules"},
		// {"AccessControlAuditLog", (AccessControlAuditLog{}).TableName(), "access_control_audit_log"},
		// {"ApiCredential", (ApiCredential{}).TableName(), "api_credentials"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("TableName() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
