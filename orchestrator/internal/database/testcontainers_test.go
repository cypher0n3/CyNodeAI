// Package database: run integration tests with testcontainers using Podman.
// Requires Podman (rootful or rootless). Rootless: systemctl --user start podman.socket.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMain(m *testing.M) {
	if os.Getenv(integrationEnv) != "" {
		os.Exit(m.Run())
		return
	}
	ctx := context.Background()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[database/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			_ = container.Terminate(ctx)
		}
		os.Exit(code)
	}()
	var err error
	container, err = postgres.Run(ctx, "postgres:16-alpine",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
	)
	if err != nil {
		_, _ = os.Stderr.WriteString("[database/testcontainers] postgres.Run failed: " + err.Error() + "\n")
		code = m.Run()
		return
	}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_, _ = os.Stderr.WriteString("[database/testcontainers] ConnectionString failed: " + err.Error() + "\n")
		code = m.Run()
		return
	}
	_ = os.Setenv(integrationEnv, connStr)
	code = m.Run()
}

func tcOpenDB(t *testing.T, ctx context.Context) Store {
	t.Helper()
	if os.Getenv(integrationEnv) == "" {
		t.Skip("postgres not started by TestMain (testcontainers skipped)")
	}
	db, err := Open(ctx, os.Getenv(integrationEnv))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	return db
}

func tcCreateUserAndVerify(t *testing.T, db Store, ctx context.Context) *models.User {
	t.Helper()
	user, err := db.CreateUser(ctx, "tc-user", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := db.GetUserByHandle(ctx, "tc-user")
	if err != nil || got.ID != user.ID {
		t.Fatalf("GetUserByHandle: %v", err)
	}
	return user
}

func tcCreateTaskJobAndVerifyPayload(t *testing.T, db Store, ctx context.Context, user *models.User) (*models.Task, *models.Job) {
	t.Helper()
	task, err := db.CreateTask(ctx, &user.ID, "prompt")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	job, err := db.CreateJob(ctx, task.ID, `{"x":1}`)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Payload.Ptr() == nil || *job.Payload.Ptr() != `{"x":1}` {
		t.Error("CreateJob payload round-trip")
	}
	return task, job
}

func tcCreateNodeAndListActive(t *testing.T, db Store, ctx context.Context) *models.Node {
	t.Helper()
	node, err := db.CreateNode(ctx, "tc-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	list, err := db.ListActiveNodes(ctx)
	if err != nil || len(list) < 1 {
		t.Fatalf("ListActiveNodes: %v", err)
	}
	return node
}

func tcCompleteJobAndVerifyResult(t *testing.T, db Store, ctx context.Context, job *models.Job, result string) {
	t.Helper()
	if err := db.CompleteJob(ctx, job.ID, result, models.JobStatusCompleted); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	gotJob, err := db.GetJobByID(ctx, job.ID)
	if err != nil || gotJob.Result.Ptr() == nil || *gotJob.Result.Ptr() != result {
		t.Fatalf("CompleteJob round-trip: %v", err)
	}
}

func TestWithTestcontainers_Integration(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	user := tcCreateUserAndVerify(t, db, ctx)
	_, job := tcCreateTaskJobAndVerifyPayload(t, db, ctx, user)
	_ = tcCreateNodeAndListActive(t, db, ctx)
	tcCompleteJobAndVerifyResult(t, db, ctx, job, `{"status":"ok"}`)
}
