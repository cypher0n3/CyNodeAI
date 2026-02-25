// Integration tests for the database package. Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

const integrationEnv = "POSTGRES_TEST_DSN"

const integrationTestPreferenceValue = `"v1"`

func TestIntegration_User(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "inttest-user", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Fatalf("GetUserByHandle: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("GetUserByHandle: id mismatch")
	}
}

func TestIntegration_CreateTaskWithNilCreatedBy(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "prompt-nil-createdby")
	if err != nil {
		t.Fatalf("CreateTask(nil createdBy): %v", err)
	}
	if task.Summary == nil || *task.Summary == "" {
		t.Error("CreateTask with nil createdBy should set task_name_001 style summary")
	}
}

func TestIntegration_TaskAndJob(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	task, err := db.CreateTask(ctx, &user.ID, "prompt")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	job, err := db.CreateJob(ctx, task.ID, `{"x":1}`)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Payload.Ptr() == nil || *job.Payload.Ptr() != `{"x":1}` {
		t.Errorf("CreateJob: payload not round-tripped")
	}
	if _, err := db.GetTaskByID(ctx, task.ID); err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if _, err := db.GetJobByID(ctx, job.ID); err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
}

func TestIntegration_Node(t *testing.T) {
	db, ctx := integrationDB(t)
	node, err := db.CreateNode(ctx, "inttest-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	list, err := db.ListActiveNodes(ctx)
	if err != nil {
		t.Fatalf("ListActiveNodes: %v", err)
	}
	if len(list) < 1 {
		t.Error("ListActiveNodes: expected at least one")
	}
}

func TestIntegration_ListDispatchableNodesAndListTasksByUser(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	node, err := db.CreateNode(ctx, "inttest-dispatchable-"+uuid.New().String())
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	if err := db.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:12090", "token"); err != nil {
		t.Fatalf("UpdateNodeWorkerAPIConfig: %v", err)
	}
	ackAt := time.Now().UTC()
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck: %v", err)
	}
	list, err := db.ListDispatchableNodes(ctx)
	if err != nil {
		t.Fatalf("ListDispatchableNodes: %v", err)
	}
	if len(list) < 1 {
		t.Error("ListDispatchableNodes: expected at least one after setup")
	}
	tasks, err := db.ListTasksByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	_ = tasks
}

func TestIntegration_NodeConfigVersionAndAck(t *testing.T) {
	db, ctx := integrationDB(t)
	node, err := db.CreateNode(ctx, "inttest-config-node-"+uuid.New().String())
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeConfigVersion(ctx, node.ID, "1"); err != nil {
		t.Fatalf("UpdateNodeConfigVersion: %v", err)
	}
	ackAt := time.Now().UTC()
	errMsg := "test error"
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck applied: %v", err)
	}
	got, err := db.GetNodeByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNodeByID: %v", err)
	}
	if got.ConfigVersion == nil || *got.ConfigVersion != "1" {
		t.Errorf("ConfigVersion: expected 1, got %v", got.ConfigVersion)
	}
	if got.ConfigAckStatus == nil || *got.ConfigAckStatus != "applied" {
		t.Errorf("ConfigAckStatus: expected applied, got %v", got.ConfigAckStatus)
	}
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "failed", ackAt, &errMsg); err != nil {
		t.Fatalf("UpdateNodeConfigAck failed: %v", err)
	}
	got, _ = db.GetNodeByID(ctx, node.ID)
	if got.ConfigAckError == nil || *got.ConfigAckError != errMsg {
		t.Errorf("ConfigAckError: expected %q, got %v", errMsg, got.ConfigAckError)
	}
}

func TestIntegration_AuthAuditLog(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	reason := "test"
	if err := db.CreateAuthAuditLog(ctx, &user.ID, "login_success", true, nil, nil, nil, &reason); err != nil {
		t.Fatalf("CreateAuthAuditLog: %v", err)
	}
}

func TestIntegration_McpToolCallAuditLog(t *testing.T) {
	db, ctx := integrationDB(t)
	rec := &models.McpToolCallAuditLog{
		ToolName: "db.preference.get",
		Decision: "allow",
		Status:   "success",
	}
	if err := db.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateMcpToolCallAuditLog: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}
	if rec.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestIntegration_Preferences_GetAndList(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		ScopeID:   nil,
		Key:       "test.key",
		Value:     &val,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference entry: %v", err)
	}
	got, err := db.GetPreference(ctx, "system", nil, "test.key")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got.Key != "test.key" || got.ScopeType != "system" {
		t.Errorf("GetPreference: got %+v", got)
	}
	list, next, err := db.ListPreferences(ctx, "system", nil, "", 10, "")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(list) < 1 || next != "" {
		t.Errorf("ListPreferences: got %d entries, next_cursor %q", len(list), next)
	}
	_, err = db.GetPreference(ctx, "system", nil, "nonexistent.key")
	if err != ErrNotFound {
		t.Errorf("GetPreference nonexistent: got %v, want ErrNotFound", err)
	}
}

func TestIntegration_Preferences_EffectiveAndCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		Key:       "test.key",
		Value:     &val,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference: %v", err)
	}
	task, err := db.CreateTask(ctx, nil, "eff-prompt")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	effective, err := db.GetEffectivePreferencesForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if effective["test.key"] == nil {
		t.Errorf("effective should contain test.key, got %v", effective)
	}
	list2, next2, err := db.ListPreferences(ctx, "system", nil, "test.", 1, "")
	if err != nil {
		t.Fatalf("ListPreferences key_prefix: %v", err)
	}
	if len(list2) < 1 {
		t.Errorf("ListPreferences key_prefix: got %d", len(list2))
	}
	if next2 != "" {
		_, _, err = db.ListPreferences(ctx, "system", nil, "test.", 10, next2)
		if err != nil {
			t.Fatalf("ListPreferences cursor: %v", err)
		}
	}
}

func TestIntegration_Preferences_UserScope(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	uv := `"user-val"`
	uent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "user",
		ScopeID:   &user.ID,
		Key:       "user.key",
		Value:     &uv,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(uent).Error; err != nil {
		t.Fatalf("create user preference: %v", err)
	}
	got, err := db.GetPreference(ctx, "user", &user.ID, "user.key")
	if err != nil {
		t.Fatalf("GetPreference user scope: %v", err)
	}
	if got.Key != "user.key" {
		t.Errorf("GetPreference: got key %q", got.Key)
	}
	ulist, _, err := db.ListPreferences(ctx, "user", &user.ID, "", 1, "")
	if err != nil {
		t.Fatalf("ListPreferences user scope: %v", err)
	}
	if len(ulist) >= 1 {
		_, nextCur, _ := db.ListPreferences(ctx, "user", &user.ID, "", 1, "0")
		if nextCur != "" {
			_, _, _ = db.ListPreferences(ctx, "user", &user.ID, "", 1, nextCur)
		}
	}
}

func TestIntegration_Preferences_EffectiveWithProject(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first")
	}
	var proj models.Project
	err = db.GORM().WithContext(ctx).Where("slug = ?", "default").First(&proj).Error
	if err != nil {
		pid := uuid.New()
		now := time.Now().UTC()
		_ = db.GORM().WithContext(ctx).Create(&models.Project{ID: pid, Slug: "default", DisplayName: "Default", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2")
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task2.ID).Update("project_id", pid).Error
			_, _ = db.GetEffectivePreferencesForTask(ctx, task2.ID)
		}
	} else {
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2")
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task2.ID).Update("project_id", proj.ID).Error
			_, _ = db.GetEffectivePreferencesForTask(ctx, task2.ID)
		}
	}
}

// TestIntegration_Preferences_ListLimitAndCursor covers ListPreferences limit cap, invalid cursor, and nextCursor when offset+len < total.
func TestIntegration_Preferences_ListLimitAndCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	for i := 0; i < 3; i++ {
		ent := &models.PreferenceEntry{
			ID:        uuid.New(),
			ScopeType: "system",
			ScopeID:   nil,
			Key:       fmt.Sprintf("cap.key.%d", i),
			Value:     &val,
			ValueType: "string",
			Version:   1,
			UpdatedAt: time.Now().UTC(),
		}
		if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
			t.Fatalf("create preference: %v", err)
		}
	}
	// Limit cap: request > MaxPreferenceListLimit
	list, _, err := db.ListPreferences(ctx, "system", nil, "", 500, "")
	if err != nil {
		t.Fatalf("ListPreferences limit 500: %v", err)
	}
	if len(list) > MaxPreferenceListLimit {
		t.Errorf("ListPreferences: expected cap at %d, got %d", MaxPreferenceListLimit, len(list))
	}
	// Invalid or negative cursor leaves offset 0; limit <= 0 is capped to MaxPreferenceListLimit
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 10, "x")
	if err != nil {
		t.Fatalf("ListPreferences invalid cursor: %v", err)
	}
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 10, "-1")
	if err != nil {
		t.Fatalf("ListPreferences negative cursor: %v", err)
	}
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 0, "")
	if err != nil {
		t.Fatalf("ListPreferences limit 0: %v", err)
	}
	// Limit 2 with 3+ entries: nextCursor = offset + len(entries) when len(entries) <= limit but more exist
	list2, next2, err := db.ListPreferences(ctx, "system", nil, "cap.key.", 2, "")
	if err != nil {
		t.Fatalf("ListPreferences key_prefix limit 2: %v", err)
	}
	if len(list2) != 2 || next2 == "" {
		t.Errorf("ListPreferences: got %d entries, next_cursor %q", len(list2), next2)
	}
	_, _, _ = db.ListPreferences(ctx, "system", nil, "cap.key.", 2, next2)
}

// TestIntegration_Preferences_EffectiveWithNilValue covers GetEffectivePreferencesForTask when an entry has nil value (skip unmarshal).
func TestIntegration_Preferences_EffectiveWithNilValue(t *testing.T) {
	db, ctx := integrationDB(t)
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		ScopeID:   nil,
		Key:       "nil.val.key",
		Value:     nil, // nil value: effective still gets the key with nil
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference: %v", err)
	}
	task, err := db.CreateTask(ctx, nil, "eff-nil-val")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	effective, err := db.GetEffectivePreferencesForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if effective["nil.val.key"] != nil {
		t.Errorf("expected nil value to yield nil in effective, got %v", effective["nil.val.key"])
	}
}

func integrationDB(t *testing.T) (*DB, context.Context) {
	t.Helper()
	dsn := os.Getenv(integrationEnv)
	if dsn == "" {
		t.Skipf("set %s to run integration tests", integrationEnv)
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	if db.GORM() == nil {
		t.Fatal("GORM() must not be nil")
	}
	return db, ctx
}

func TestIntegration_GetNextQueuedJob_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	// Drain queue so we can assert ErrNotFound when empty (tests share one DB).
	for {
		job, err := db.GetNextQueuedJob(ctx)
		if err == ErrNotFound {
			break
		}
		if err != nil {
			t.Fatalf("GetNextQueuedJob: %v", err)
		}
		_ = db.CompleteJob(ctx, job.ID, "", models.JobStatusCancelled)
	}
	_, err := db.GetNextQueuedJob(ctx)
	if err != ErrNotFound {
		t.Errorf("GetNextQueuedJob: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CompleteJobRoundTrip(t *testing.T) {
	db, ctx := integrationDB(t)
	task, _ := db.CreateTask(ctx, nil, "p")
	job, _ := db.CreateJob(ctx, task.ID, "payload")
	result := `{"status":"ok"}`
	if err := db.CompleteJob(ctx, job.ID, result, models.JobStatusCompleted); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	got, err := db.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if got.Result.Ptr() == nil || *got.Result.Ptr() != result {
		t.Errorf("result not round-tripped: got %v", got.Result.Ptr())
	}
}

func TestIntegration_CreateJobCompleted(t *testing.T) {
	db, ctx := integrationDB(t)
	task, _ := db.CreateTask(ctx, nil, "prompt-task")
	jobID := uuid.New()
	result := `{"version":1,"task_id":"` + task.ID.String() + `","job_id":"` + jobID.String() + `","status":"completed","stdout":"hi"}`
	job, err := db.CreateJobCompleted(ctx, task.ID, jobID, result)
	if err != nil {
		t.Fatalf("CreateJobCompleted: %v", err)
	}
	if job.Status != models.JobStatusCompleted {
		t.Errorf("status want completed got %s", job.Status)
	}
	if job.Result.Ptr() == nil || *job.Result.Ptr() != result {
		t.Errorf("result not stored: got %v", job.Result.Ptr())
	}
	jobs, _ := db.GetJobsByTaskID(ctx, task.ID)
	if len(jobs) != 1 || jobs[0].ID != jobID {
		t.Errorf("GetJobsByTaskID: got %v", jobs)
	}
}

func storeRoundTripCredentials(t *testing.T, db Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	_, err := db.CreatePasswordCredential(ctx, userID, []byte("hash"), "argon2")
	if err != nil {
		t.Fatalf("CreatePasswordCredential: %v", err)
	}
	if _, err := db.GetPasswordCredentialByUserID(ctx, userID); err != nil {
		t.Fatalf("GetPasswordCredentialByUserID: %v", err)
	}
}

func storeRoundTripSessions(t *testing.T, db Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	expires := time.Now().Add(time.Hour)
	session, err := db.CreateRefreshSession(ctx, userID, []byte("tokenhash"), expires)
	if err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}
	if _, err := db.GetActiveRefreshSession(ctx, []byte("tokenhash")); err != nil {
		t.Fatalf("GetActiveRefreshSession: %v", err)
	}
	if err := db.InvalidateRefreshSession(ctx, session.ID); err != nil {
		t.Fatalf("InvalidateRefreshSession: %v", err)
	}
}

func storeRoundTripNode(t *testing.T, db Store, ctx context.Context) *models.Node {
	t.Helper()
	node, err := db.CreateNode(ctx, "inttest-roundtrip-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if _, err := db.GetNodeByID(ctx, node.ID); err != nil {
		t.Fatalf("GetNodeByID: %v", err)
	}
	if _, err := db.GetNodeBySlug(ctx, "inttest-roundtrip-node"); err != nil {
		t.Fatalf("GetNodeBySlug: %v", err)
	}
	return node
}

func storeRoundTripTaskJobNode(t *testing.T, db Store, ctx context.Context, userID uuid.UUID, node *models.Node) {
	t.Helper()
	task, err := db.CreateTask(ctx, &userID, "roundtrip-prompt")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}
	if err := db.UpdateTaskSummary(ctx, task.ID, "summary"); err != nil {
		t.Fatalf("UpdateTaskSummary: %v", err)
	}
	list, err := db.ListTasksByUser(ctx, userID, 10, 0)
	if err != nil || len(list) < 1 {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	job, err := db.CreateJob(ctx, task.ID, `{"r":1}`)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if _, err := db.GetJobsByTaskID(ctx, task.ID); err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if err := db.UpdateJobStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}
	if err := db.AssignJobToNode(ctx, job.ID, node.ID); err != nil {
		t.Fatalf("AssignJobToNode: %v", err)
	}
	_ = db.CompleteJob(ctx, job.ID, "{}", models.JobStatusCompleted)
}

// TestIntegration_StoreRoundTrip exercises more Store methods for coverage.
func TestIntegration_StoreRoundTrip(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	if _, err := db.GetUserByID(ctx, user.ID); err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	storeRoundTripCredentials(t, db, ctx, user.ID)
	storeRoundTripSessions(t, db, ctx, user.ID)
	node := storeRoundTripNode(t, db, ctx)
	storeRoundTripTaskJobNode(t, db, ctx, user.ID, node)
	if err := db.InvalidateAllUserSessions(ctx, user.ID); err != nil {
		t.Fatalf("InvalidateAllUserSessions: %v", err)
	}
	_ = db.UpdateNodeLastSeen(ctx, node.ID)
	_ = db.UpdateNodeCapability(ctx, node.ID, "hash")
	_ = db.SaveNodeCapabilitySnapshot(ctx, node.ID, `{}`)
}

func TestIntegration_GetUserByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetUserByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetUserByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetUserByHandle_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetUserByHandle(ctx, "nonexistent-handle-"+uuid.New().String())
	if err != ErrNotFound {
		t.Errorf("GetUserByHandle: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetNodeBySlug_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetNodeBySlug(ctx, "nonexistent-slug-"+uuid.New().String())
	if err != ErrNotFound {
		t.Errorf("GetNodeBySlug: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetNodeByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetNodeByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetNodeByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CreateUser_DuplicateHandle(t *testing.T) {
	db, ctx := integrationDB(t)
	handle := "dup-handle-" + uuid.New().String()
	_, err := db.CreateUser(ctx, handle, nil)
	if err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err = db.CreateUser(ctx, handle, nil)
	if err == nil {
		t.Error("second CreateUser with same handle should fail")
	}
}

func TestIntegration_GetTaskByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetTaskByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetTaskByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetJobByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetJobByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetJobByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetPasswordCredentialByUserID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "nocred-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.GetPasswordCredentialByUserID(ctx, user.ID)
	if err != ErrNotFound {
		t.Errorf("GetPasswordCredentialByUserID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetActiveRefreshSession_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetActiveRefreshSession(ctx, []byte("nonexistent-token-hash"))
	if err != ErrNotFound {
		t.Errorf("GetActiveRefreshSession: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_ListTasksByUser_Empty(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "notasks-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	list, err := db.ListTasksByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListTasksByUser: expected empty, got %d", len(list))
	}
}

func TestIntegration_GetJobsByTaskID_Empty(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "no-jobs-task")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobs, err := db.GetJobsByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("GetJobsByTaskID: expected empty, got %d", len(jobs))
	}
}

func TestIntegration_ChatThreadsAndMessages(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "chat-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	thread, err := db.GetOrCreateActiveChatThread(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread: %v", err)
	}
	if thread.ID == uuid.Nil || thread.UserID != user.ID {
		t.Errorf("thread: id or user_id mismatch")
	}
	msg, err := db.AppendChatMessage(ctx, thread.ID, "user", "hello", nil)
	if err != nil {
		t.Fatalf("AppendChatMessage: %v", err)
	}
	if msg.Role != "user" || msg.Content != "hello" {
		t.Errorf("message: role or content mismatch")
	}
	msg2, err := db.AppendChatMessage(ctx, thread.ID, "assistant", "hi back", nil)
	if err != nil {
		t.Fatalf("AppendChatMessage assistant: %v", err)
	}
	if msg2.Content != "hi back" {
		t.Errorf("assistant message content: got %q", msg2.Content)
	}
	rec := &models.ChatAuditLog{
		UserID:           &user.ID,
		Outcome:          "success",
		RedactionApplied: false,
	}
	if err := db.CreateChatAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateChatAuditLog: %v", err)
	}
	if rec.ID == uuid.Nil || rec.CreatedAt.IsZero() {
		t.Error("CreateChatAuditLog: expected ID and CreatedAt set")
	}
}
