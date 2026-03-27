// Integration tests: workflow leases, jobs, store round-trips, ErrNotFound paths, chat threads.
// Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestIntegration_WorkflowLeaseAndCheckpoint(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-lease-test", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID := uuid.New()
	holderID := "runner-1"
	expiresAt := time.Now().UTC().Add(time.Hour)
	lease, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, holderID, expiresAt)
	if err != nil {
		t.Fatalf("AcquireTaskWorkflowLease: %v", err)
	}
	if lease.LeaseID != leaseID {
		t.Errorf("lease_id: got %s", lease.LeaseID)
	}
	gotLease, err := db.GetTaskWorkflowLease(ctx, task.ID)
	if err != nil || gotLease.LeaseID != leaseID {
		t.Fatalf("GetTaskWorkflowLease: %v, got %+v", err, gotLease)
	}
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, uuid.New(), "runner-2", expiresAt)
	if !errors.Is(err, ErrLeaseHeld) {
		t.Errorf("duplicate acquire: want ErrLeaseHeld, got %v", err)
	}
	if err := db.ReleaseTaskWorkflowLease(ctx, task.ID, leaseID); err != nil {
		t.Fatalf("ReleaseTaskWorkflowLease: %v", err)
	}
	state := `{"task_id":"` + task.ID.String() + `"}`
	cp := &models.WorkflowCheckpoint{
		WorkflowCheckpointBase: models.WorkflowCheckpointBase{
			TaskID:     task.ID,
			State:      &state,
			LastNodeID: "plan_steps",
		},
	}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint: %v", err)
	}
	got, err := db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWorkflowCheckpoint: %v", err)
	}
	if got.LastNodeID != "plan_steps" {
		t.Errorf("GetWorkflowCheckpoint: last_node_id %q", got.LastNodeID)
	}
	// Update path: upsert again with new last_node_id.
	cp2 := &models.WorkflowCheckpoint{
		WorkflowCheckpointBase: models.WorkflowCheckpointBase{
			TaskID:     task.ID,
			State:      &state,
			LastNodeID: "dispatch_step",
		},
	}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp2); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint update: %v", err)
	}
	got2, _ := db.GetWorkflowCheckpoint(ctx, task.ID)
	if got2.LastNodeID != "dispatch_step" {
		t.Errorf("after update: last_node_id %q", got2.LastNodeID)
	}
}

func TestIntegration_WorkflowLease_ExpiredReacquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-lease-expiry", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID1 := uuid.New()
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID1, "runner-1", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("AcquireTaskWorkflowLease: %v", err)
	}
	// Expire the lease in DB so a different holder can re-acquire.
	past := time.Now().UTC().Add(-time.Minute)
	if err := db.GORM().WithContext(ctx).Model(&TaskWorkflowLeaseRecord{}).Where("task_id = ?", task.ID).Update("expires_at", past).Error; err != nil {
		t.Fatalf("update expires_at: %v", err)
	}
	leaseID2 := uuid.New()
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID2, "runner-2", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Re-acquire after expiry: %v", err)
	}
	if lease2.HolderID == nil || *lease2.HolderID != "runner-2" || lease2.LeaseID != leaseID2 {
		t.Errorf("re-acquired lease: got %+v", lease2)
	}
}

func TestIntegration_WorkflowLease_IdempotentAcquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-idempotent", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID := uuid.New()
	expiresAt := time.Now().UTC().Add(time.Hour)
	lease1, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, "runner-1", expiresAt)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, "runner-1", expiresAt)
	if err != nil {
		t.Fatalf("second acquire (idempotent): %v", err)
	}
	if lease2.LeaseID != lease1.LeaseID || lease2.ID != lease1.ID {
		t.Errorf("idempotent acquire: got %+v, want same as %+v", lease2, lease1)
	}
}

func TestIntegration_WorkflowLease_ReleaseThenReacquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-release-reacquire", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID1 := uuid.New()
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID1, "runner-1", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := db.ReleaseTaskWorkflowLease(ctx, task.ID, leaseID1); err != nil {
		t.Fatalf("Release: %v", err)
	}
	leaseID2 := uuid.New()
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID2, "runner-2", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Re-acquire after release: %v", err)
	}
	if lease2.LeaseID != leaseID2 || lease2.HolderID == nil || *lease2.HolderID != "runner-2" {
		t.Errorf("re-acquired lease: got %+v", lease2)
	}
}

func TestIntegration_WorkflowErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-err-not-found", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	_, err = db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != ErrNotFound {
		t.Errorf("GetWorkflowCheckpoint: want ErrNotFound, got %v", err)
	}
	_, err = db.GetTaskWorkflowLease(ctx, task.ID)
	if err != ErrNotFound {
		t.Errorf("GetTaskWorkflowLease: want ErrNotFound, got %v", err)
	}
}

func TestIntegration_UpsertWorkflowCheckpoint_WithPreSetID(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-checkpoint-preset-id", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	state := integrationTestPayloadX1
	cp := &models.WorkflowCheckpoint{
		WorkflowCheckpointBase: models.WorkflowCheckpointBase{
			TaskID:     task.ID,
			State:      &state,
			LastNodeID: "step1",
		},
		ID: uuid.New(),
	}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint: %v", err)
	}
	got, err := db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWorkflowCheckpoint: %v", err)
	}
	if got.ID != cp.ID || got.LastNodeID != "step1" {
		t.Errorf("checkpoint: got %+v", got)
	}
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
		_ = db.CompleteJob(ctx, job.ID, "", models.JobStatusCanceled)
	}
	_, err := db.GetNextQueuedJob(ctx)
	if err != ErrNotFound {
		t.Errorf("GetNextQueuedJob: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CompleteJobRoundTrip(t *testing.T) {
	db, ctx := integrationDB(t)
	task, _ := db.CreateTask(ctx, nil, "p", nil, nil)
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
	task, _ := db.CreateTask(ctx, nil, "prompt-task", nil, nil)
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
	task, err := db.CreateTask(ctx, &userID, "roundtrip-prompt", nil, nil)
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
	user := integrationEnsureInttestUser(t, db, ctx)
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

func TestIntegration_CreatePasswordCredential_DuplicateUser(t *testing.T) {
	db, ctx := integrationDB(t)
	handle := "pwd-dup-" + uuid.New().String()
	user, err := db.CreateUser(ctx, handle, nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := db.CreatePasswordCredential(ctx, user.ID, []byte("first"), "argon2"); err != nil {
		t.Fatalf("first CreatePasswordCredential: %v", err)
	}
	if _, err := db.CreatePasswordCredential(ctx, user.ID, []byte("second"), "argon2"); err == nil {
		t.Fatal("second CreatePasswordCredential: expected error for duplicate user row")
	}
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

func TestIntegration_CreateJobWithID_DuplicateID_ReturnsError(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "job-dup-id", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobID := uuid.New()
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, integrationTestPayloadX1)
	if err != nil {
		t.Fatalf("first CreateJobWithID: %v", err)
	}
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, integrationTestPayloadX1)
	if err == nil {
		t.Error("second CreateJobWithID with same ID: expected error")
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
	task, err := db.CreateTask(ctx, nil, "no-jobs-task", nil, nil)
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
	assertListChatMessages(t, db, ctx, thread.ID)
	rec := &models.ChatAuditLog{
		ChatAuditLogBase: models.ChatAuditLogBase{
			UserID:           &user.ID,
			Outcome:          "success",
			RedactionApplied: false,
		},
	}
	if err := db.CreateChatAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateChatAuditLog: %v", err)
	}
	if rec.ID == uuid.Nil || rec.CreatedAt.IsZero() {
		t.Error("CreateChatAuditLog: expected ID and CreatedAt set")
	}
}
