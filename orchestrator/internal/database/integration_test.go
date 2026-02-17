// Integration tests for the database package. Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
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
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	return db, ctx
}

func TestIntegration_GetNextQueuedJob_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
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
