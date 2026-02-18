// Integration tests for the database package. Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

const integrationEnv = "POSTGRES_TEST_DSN"

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
