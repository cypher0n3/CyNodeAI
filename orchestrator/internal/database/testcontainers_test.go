// Package database: run integration tests with testcontainers using Podman.
// Requires Podman (rootful or rootless). Rootless: systemctl --user start podman.socket.
//
// Rootless Podman: testcontainers-go discovers the daemon via DOCKER_HOST. It does not
// check the rootless Podman socket path ($XDG_RUNTIME_DIR/podman/podman.sock) by default.
// We set DOCKER_HOST here when unset so "go test ./..." works without exporting it in the shell.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// setupRootlessPodmanHost sets DOCKER_HOST to the rootless Podman socket if DOCKER_HOST
// is unset and the socket exists. Required for testcontainers-go to find rootless Podman.
func setupRootlessPodmanHost() {
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return
	}
	sock := filepath.Join(runtimeDir, "podman", "podman.sock")
	if _, err := os.Stat(sock); err != nil {
		return
	}
	_ = os.Setenv("DOCKER_HOST", "unix://"+sock)
}

// testcontainersSetupTimeout bounds how long TestMain waits for container start.
// Prevents CI from hanging indefinitely if Podman/testcontainers blocks.
const testcontainersSetupTimeout = 90 * time.Second

const wantDSNIPv4 = "postgres://127.0.0.1:5432/db"

// TestDsnForceIPv4 covers the DSN rewrite helper (localhost -> 127.0.0.1; parse error returns as-is).
func TestDsnForceIPv4(t *testing.T) {
	if got := dsnForceIPv4("://bad"); got != "://bad" {
		t.Errorf("parse error: got %q", got)
	}
	if got := dsnForceIPv4("postgres://localhost:5432/db"); got != wantDSNIPv4 {
		t.Errorf("localhost: got %q", got)
	}
	if got := dsnForceIPv4("postgres://localhost/db"); got != wantDSNIPv4 {
		t.Errorf("localhost no port: got %q", got)
	}
	if got := dsnForceIPv4("postgres://example.com:5432/db"); got != "postgres://example.com:5432/db" {
		t.Errorf("other host: got %q", got)
	}
	if got := dsnForceIPv4("postgres://[::1]:5432/db"); got != wantDSNIPv4 {
		t.Errorf("::1: got %q", got)
	}
}

// dsnForceIPv4 rewrites a postgres DSN so the host is 127.0.0.1 instead of localhost,
// avoiding "connection reset by peer" when Podman only forwards IPv4.
func dsnForceIPv4(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	host := u.Hostname()
	if host == "localhost" || host == "::1" {
		port := u.Port()
		if port == "" {
			port = "5432"
		}
		u.Host = "127.0.0.1:" + port
		return u.String()
	}
	return dsn
}

// runTestcontainersSetup starts Postgres via testcontainers and waits for it.
// On success returns (container, true) with integrationEnv set. On failure logs to stderr and returns (container or nil, false).
func runTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, testcontainersSetupTimeout)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
		postgres.BasicWaitStrategies(), // wait for "ready to accept connections" (2x) and port before considering ready
	)
	if err != nil {
		writeTestcontainersErr(setupCtx, "postgres.Run failed: "+err.Error())
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		writeTestcontainersErr(setupCtx, "ConnectionString failed: "+err.Error())
		return container, false
	}
	connStr = dsnForceIPv4(connStr)
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(integrationEnv, connStr)
	_ = os.Setenv("DATABASE_URL", connStr) // so mcp-gateway can use same container when it runs after database
	if err := waitForPostgres(setupCtx, connStr, 60*time.Second); err != nil {
		writeTestcontainersErr(setupCtx, "postgres not ready: "+err.Error())
		return container, false
	}
	return container, true
}

func writeTestcontainersErr(ctx context.Context, fallback string) {
	if ctx.Err() != nil {
		_, _ = os.Stderr.WriteString("[database/testcontainers] setup timed out after " + testcontainersSetupTimeout.String() + "; running tests without DB\n")
		return
	}
	_, _ = os.Stderr.WriteString("[database/testcontainers] " + fallback + "\n")
}

// testcontainersResult carries the outcome of runTestcontainersSetup from a goroutine.
type testcontainersResult struct {
	container *postgres.PostgresContainer
	ok        bool
}

func TestMain(m *testing.M) {
	if dsn := os.Getenv(integrationEnv); dsn != "" {
		// Use existing DSN only if it actually connects (avoids stale/bad env and forces testcontainers when needed).
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := Open(ctx, dsn)
		cancel()
		if err == nil {
			_ = db.Close()
			os.Exit(m.Run())
			return
		}
		_ = os.Unsetenv(integrationEnv)
		_ = os.Unsetenv("DATABASE_URL")
	}
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHost()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[database/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	// Run setup in a goroutine so we can enforce a hard timeout; some testcontainers
	// operations (e.g. image pull) may not respect context and would otherwise hang CI.
	hardTimeout := testcontainersSetupTimeout + 15*time.Second
	resultCh := make(chan testcontainersResult, 1)
	go func() {
		c, ok := runTestcontainersSetup(context.Background())
		resultCh <- testcontainersResult{container: c, ok: ok}
	}()
	var ok bool
	select {
	case res := <-resultCh:
		container = res.container
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[database/testcontainers] setup did not complete within " + hardTimeout.String() + "; running tests without DB\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

// waitForPostgres polls the database until it accepts connections or timeout. Checks once per second.
func waitForPostgres(ctx context.Context, dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		db, err := Open(ctx, dsn)
		if err == nil {
			_ = db.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("postgres not ready within %v", timeout)
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
	task, err := db.CreateTask(ctx, &user.ID, "prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	meta := `{"tc":1}`
	if err := db.UpdateTaskMetadata(ctx, task.ID, &meta); err != nil {
		t.Fatalf("UpdateTaskMetadata: %v", err)
	}
	reloaded, err := db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID after metadata: %v", err)
	}
	if reloaded.Metadata == nil || strings.TrimSpace(*reloaded.Metadata) == "" {
		t.Errorf("metadata not stored: %v", reloaded.Metadata)
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

func tcCreateTaskAndJobWithID(t *testing.T, db Store, ctx context.Context, user *models.User, jobID uuid.UUID, payload string) (*models.Task, *models.Job) {
	t.Helper()
	task, err := db.CreateTask(ctx, &user.ID, "sba-prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	job, err := db.CreateJobWithID(ctx, task.ID, jobID, payload)
	if err != nil {
		t.Fatalf("CreateJobWithID: %v", err)
	}
	if job.ID != jobID {
		t.Errorf("CreateJobWithID: job.ID = %v, want %v", job.ID, jobID)
	}
	got, err := db.GetJobByID(ctx, jobID)
	if err != nil || got.Payload.Ptr() == nil || *got.Payload.Ptr() != payload {
		t.Fatalf("CreateJobWithID round-trip: %v", err)
	}
	return task, job
}

func TestWithTestcontainers_CreateTask_WithTaskName(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	user, err := db.CreateUser(ctx, "tc-user-taskname-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	name1 := "My Task  Name"
	task1, err := db.CreateTask(ctx, &user.ID, "prompt1", &name1, nil)
	if err != nil {
		t.Fatalf("CreateTask with task name: %v", err)
	}
	if task1.Summary == nil || *task1.Summary != "my-task-name" {
		t.Errorf("CreateTask task name normalized: got %q, want my-task-name", ptrVal(task1.Summary))
	}
	name2 := "My Task  Name"
	task2, err := db.CreateTask(ctx, &user.ID, "prompt2", &name2, nil)
	if err != nil {
		t.Fatalf("second CreateTask same name: %v", err)
	}
	if task2.Summary == nil || *task2.Summary != "my-task-name-2" {
		t.Errorf("CreateTask uniqueness: got %q, want my-task-name-2", ptrVal(task2.Summary))
	}
}

func tcAssertTaskMatchesUserAndSummary(t *testing.T, got *models.Task, userID uuid.UUID, summary string) {
	t.Helper()
	if got.CreatedBy == nil || *got.CreatedBy != userID || got.Summary == nil || *got.Summary != summary {
		t.Errorf("task: got created_by=%v summary=%v, want user %s summary %q", got.CreatedBy, got.Summary, userID, summary)
	}
}

func TestWithTestcontainers_GetTaskBySummary(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	user, err := db.CreateUser(ctx, "tc-user-getsummary-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	summary := "my-summary"
	task1, err := db.CreateTask(ctx, &user.ID, "p1", &summary, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	got, err := db.GetTaskBySummary(ctx, user.ID, summary)
	if err != nil {
		t.Fatalf("GetTaskBySummary: %v", err)
	}
	if got.ID != task1.ID {
		t.Errorf("GetTaskBySummary: got id %s, want %s", got.ID, task1.ID)
	}
	tcAssertTaskMatchesUserAndSummary(t, got, user.ID, summary)
	_, err = db.CreateTask(ctx, &user.ID, "p2", &summary, nil)
	if err != nil {
		t.Fatalf("CreateTask 2: %v", err)
	}
	got2, err := db.GetTaskBySummary(ctx, user.ID, summary)
	if err != nil {
		t.Fatalf("GetTaskBySummary second: %v", err)
	}
	tcAssertTaskMatchesUserAndSummary(t, got2, user.ID, summary)
	_, err = db.GetTaskBySummary(ctx, user.ID, "nonexistent")
	if err == nil {
		t.Error("GetTaskBySummary nonexistent: expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTaskBySummary: want ErrNotFound, got %v", err)
	}
}

func testcontainersProjectReadSetup(t *testing.T) (context.Context, Store, uuid.UUID, *models.Project) {
	t.Helper()
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	user, err := db.CreateUser(ctx, "tc-user-projread-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	def, err := db.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateDefaultProjectForUser: %v", err)
	}
	return ctx, db, user.ID, def
}

func TestWithTestcontainers_ProjectRead_GetByIDAndSlug(t *testing.T) {
	ctx, db, _, def := testcontainersProjectReadSetup(t)
	byID, err := db.GetProjectByID(ctx, def.ID)
	if err != nil || byID == nil || byID.ID != def.ID {
		t.Fatalf("GetProjectByID: %v got=%v", err, byID)
	}
	bySlug, err := db.GetProjectBySlug(ctx, def.Slug)
	if err != nil || bySlug == nil || bySlug.ID != def.ID {
		t.Fatalf("GetProjectBySlug: %v got=%v", err, bySlug)
	}
}

func TestWithTestcontainers_ProjectRead_ListAuthorized(t *testing.T) {
	ctx, db, uid, def := testcontainersProjectReadSetup(t)
	cases := []struct {
		name    string
		q       string
		limit   int
		offset  int
		wantLen int
		wantID  bool
	}{
		{"empty q", "", 50, 0, 1, true},
		{"match slug", def.Slug, 50, 0, 1, false},
		{"miss filter", "zzzz-not-in-project", 50, 0, 0, false},
		{"offset past row", "", 50, 99, 0, false},
		{"limit zero", "", 0, 0, 1, false},
		{"limit 500 mvp", "", 500, 0, 1, false},
		{"negative offset", "", 50, -3, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list, err := db.ListAuthorizedProjectsForUser(ctx, uid, tc.q, tc.limit, tc.offset)
			if err != nil || len(list) != tc.wantLen {
				t.Fatalf("len: err=%v got=%d want=%d", err, len(list), tc.wantLen)
			}
			if tc.wantID && tc.wantLen == 1 && list[0].ID != def.ID {
				t.Fatalf("wrong project id")
			}
		})
	}
}

func TestWithTestcontainers_ProjectRead_NotFound(t *testing.T) {
	ctx, db, _, _ := testcontainersProjectReadSetup(t)
	_, err := db.GetProjectByID(ctx, uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectByID missing: %v", err)
	}
	_, err = db.GetProjectBySlug(ctx, "no-such-slug-"+uuid.New().String())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProjectBySlug missing: %v", err)
	}
}

func TestWithTestcontainers_ListAuthorizedProjectsForUser_MatchesDescription(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	dbImpl := store.(*DB)
	user, err := dbImpl.CreateUser(ctx, "tc-proj-desc-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	def, err := dbImpl.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateDefaultProjectForUser: %v", err)
	}
	desc := "findme-in-description"
	if err := dbImpl.db.WithContext(ctx).Model(&ProjectRecord{}).Where("id = ?", def.ID).Update("description", desc).Error; err != nil {
		t.Fatalf("update description: %v", err)
	}
	list, err := dbImpl.ListAuthorizedProjectsForUser(ctx, user.ID, "findme", 50, 0)
	if err != nil || len(list) != 1 || list[0].ID != def.ID {
		t.Fatalf("filter by description: err=%v len=%d", err, len(list))
	}
}

func TestWithTestcontainers_DefaultProjectAndTaskProjectID(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	user, err := db.CreateUser(ctx, "tc-user-proj-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	defaultProj, err := db.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateDefaultProjectForUser: %v", err)
	}
	if defaultProj == nil || defaultProj.ID == uuid.Nil {
		t.Fatal("expected default project")
	}
	sameProj, err := db.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("second GetOrCreateDefaultProjectForUser: %v", err)
	}
	if sameProj.ID != defaultProj.ID {
		t.Fatalf("default project should be stable: %s vs %s", sameProj.ID, defaultProj.ID)
	}
	task, err := db.CreateTask(ctx, &user.ID, "prompt", nil, &defaultProj.ID)
	if err != nil {
		t.Fatalf("CreateTask with project id: %v", err)
	}
	if task.ProjectID == nil || *task.ProjectID != defaultProj.ID {
		t.Fatalf("task.ProjectID = %v, want %s", task.ProjectID, defaultProj.ID)
	}
}

func ptrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestWithTestcontainers_CreateJobWithID_DuplicateIDReturnsError(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	// Use unique handle so we do not conflict with tcCreateUserAndVerify("tc-user") in other tests.
	user, err := db.CreateUser(ctx, "tc-user-dup-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	task, err := db.CreateTask(ctx, &user.ID, "prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobID := uuid.New()
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, `{"job_spec_json":"{}"}`)
	if err != nil {
		t.Fatalf("first CreateJobWithID: %v", err)
	}
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, `{"x":2}`)
	if err == nil {
		t.Error("second CreateJobWithID with same jobID should return error")
	}
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
	// Exercise CreateJobWithID (SBA job path).
	sbaJobID := uuid.New()
	tcCreateTaskAndJobWithID(t, db, ctx, user, sbaJobID, `{"job_spec_json":"{}","image":"cynodeai-cynode-sba:dev"}`)
	node := tcCreateNodeAndListActive(t, db, ctx)
	if err := db.UpdateNodeConfigVersion(ctx, node.ID, "1"); err != nil {
		t.Fatalf("UpdateNodeConfigVersion: %v", err)
	}
	if err := db.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://worker:12090", "token"); err != nil {
		t.Fatalf("UpdateNodeWorkerAPIConfig: %v", err)
	}
	ackAt := time.Now().UTC()
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck: %v", err)
	}
	dispatchable, err := db.ListDispatchableNodes(ctx)
	if err != nil {
		t.Fatalf("ListDispatchableNodes: %v", err)
	}
	if len(dispatchable) < 1 {
		t.Errorf("ListDispatchableNodes: expected at least one node, got %d", len(dispatchable))
	} else {
		found := false
		for _, n := range dispatchable {
			if n.ID == node.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("ListDispatchableNodes: expected our node in the list")
		}
	}
	tcCompleteJobAndVerifyResult(t, db, ctx, job, `{"status":"ok"}`)
}

// TestWithTestcontainers_GetLatestNodeCapabilitySnapshot exercises Save and GetLatest of capability snapshot.
func TestWithTestcontainers_GetLatestNodeCapabilitySnapshot(t *testing.T) {
	ctx := context.Background()
	db := tcOpenDB(t, ctx)
	node, err := db.CreateNode(ctx, "tc-cap-node-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	capJSON := `{"version":1,"reported_at":"2026-02-28T12:00:00Z","node":{"node_slug":"tc-cap-node"},"inference":{"supported":true,"existing_service":false}}`
	if err := db.SaveNodeCapabilitySnapshot(ctx, node.ID, capJSON); err != nil {
		t.Fatalf("SaveNodeCapabilitySnapshot: %v", err)
	}
	got, err := db.GetLatestNodeCapabilitySnapshot(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetLatestNodeCapabilitySnapshot: %v", err)
	}
	// Postgres JSONB may reorder keys and spacing; check equivalent content.
	if got == "" || !strings.Contains(got, "tc-cap-node") || !strings.Contains(got, "supported") {
		t.Errorf("GetLatestNodeCapabilitySnapshot: got %q", got)
	}
	_, err = db.GetLatestNodeCapabilitySnapshot(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetLatestNodeCapabilitySnapshot on unknown node: want ErrNotFound, got %v", err)
	}
}

// TestWithTestcontainers_Preferences exercises preference store with the testcontainers DB.
func TestWithTestcontainers_Preferences(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	db, ok := store.(*DB)
	if !ok {
		t.Fatal("tcOpenDB did not return *DB")
	}
	val := integrationTestPreferenceValue
	ent := &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			ScopeID:   nil,
			Key:       "tc.pref.key",
			Value:     &val,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference: %v", err)
	}
	got, err := store.GetPreference(ctx, "system", nil, "tc.pref.key")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got.Key != "tc.pref.key" {
		t.Errorf("GetPreference: got key %q", got.Key)
	}
	list, next, err := store.ListPreferences(ctx, "system", nil, "", 10, "")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("ListPreferences: got %d entries", len(list))
	}
	_ = next
	task, err := store.CreateTask(ctx, nil, "tc-pref-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	effective, err := store.GetEffectivePreferencesForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if effective["tc.pref.key"] == nil {
		t.Errorf("effective missing tc.pref.key: %v", effective)
	}
}

// TestWithTestcontainers_PreferenceCRUDAndArtifact exercises CreatePreference, UpdatePreference, DeletePreference and GetArtifactByTaskIDAndPath.
func TestWithTestcontainers_PreferenceCRUDAndArtifact(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	key := "tc.crud." + uuid.New().String()
	ent, err := store.CreatePreference(ctx, "system", nil, key, `"v1"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	if ent.Version != 1 {
		t.Errorf("CreatePreference: want version 1, got %d", ent.Version)
	}
	_, err = store.CreatePreference(ctx, "system", nil, key, `"v2"`, "string", nil, nil)
	if err != ErrExists {
		t.Errorf("CreatePreference duplicate: want ErrExists, got %v", err)
	}
	ev := ent.Version
	ent2, err := store.UpdatePreference(ctx, "system", nil, key, `"updated"`, "string", &ev, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePreference: %v", err)
	}
	if ent2.Version <= ent.Version {
		t.Errorf("UpdatePreference: version should increase, got %d then %d", ent.Version, ent2.Version)
	}
	cur, err := store.GetPreference(ctx, "system", nil, key)
	if err != nil {
		t.Fatalf("GetPreference before delete: %v", err)
	}
	evDel := cur.Version
	err = store.DeletePreference(ctx, "system", nil, key, &evDel, nil)
	if err != nil {
		t.Fatalf("DeletePreference: %v", err)
	}
	_, err = store.GetPreference(ctx, "system", nil, key)
	if err != ErrNotFound {
		t.Errorf("after DeletePreference: want ErrNotFound, got %v", err)
	}
	task, err := store.CreateTask(ctx, nil, "tc-artifact-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	tcAssertTaskArtifacts(t, store, ctx, task)
}

func tcAssertTaskArtifacts(t *testing.T, store Store, ctx context.Context, task *models.Task) {
	t.Helper()
	db := store.(*DB)
	art := &models.TaskArtifact{
		TaskArtifactBase: models.TaskArtifactBase{
			TaskID:     task.ID,
			Path:       "tc/out.txt",
			StorageRef: "ref:xyz",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(art).Error; err != nil {
		t.Fatalf("create task artifact: %v", err)
	}
	got, err := store.GetArtifactByTaskIDAndPath(ctx, task.ID, "tc/out.txt")
	if err != nil {
		t.Fatalf("GetArtifactByTaskIDAndPath: %v", err)
	}
	if got.StorageRef != "ref:xyz" {
		t.Errorf("GetArtifactByTaskIDAndPath: got storage_ref %q", got.StorageRef)
	}
	_, err = store.GetArtifactByTaskIDAndPath(ctx, task.ID, "missing")
	if err != ErrNotFound {
		t.Errorf("GetArtifactByTaskIDAndPath missing: want ErrNotFound, got %v", err)
	}
	art2, errArt := store.CreateTaskArtifact(ctx, task.ID, "upload/a.txt", "", nil)
	if errArt != nil {
		t.Fatalf("CreateTaskArtifact: %v", errArt)
	}
	if art2.Path != "upload/a.txt" || art2.TaskID != task.ID {
		t.Errorf("CreateTaskArtifact: got %+v", art2)
	}
	paths, err := store.ListArtifactPathsByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListArtifactPathsByTaskID: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("ListArtifactPathsByTaskID: want 2 paths, got %d: %v", len(paths), paths)
	}
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}
	if !pathSet["tc/out.txt"] || !pathSet["upload/a.txt"] {
		t.Errorf("ListArtifactPathsByTaskID: want tc/out.txt and upload/a.txt, got %v", paths)
	}
	_, errDup := store.CreateTaskArtifact(ctx, task.ID, "upload/a.txt", "", nil)
	if errDup == nil {
		t.Error("CreateTaskArtifact duplicate path: expected error")
	}
}

func tcAssertHasAnyActiveApiCredential(t *testing.T, store Store, ctx context.Context, want bool) {
	t.Helper()
	got, err := store.HasAnyActiveApiCredential(ctx)
	if err != nil {
		t.Fatalf("HasAnyActiveApiCredential: %v", err)
	}
	if got != want {
		t.Errorf("HasAnyActiveApiCredential: got %v want %v", got, want)
	}
}

func TestWithTestcontainers_AccessControlAndApiCredential(t *testing.T) {
	ctx := context.Background()
	store := tcOpenDB(t, ctx)
	user, err := store.CreateUser(ctx, "tc-ac-user", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	task, err := store.CreateTask(ctx, &user.ID, "tc-ac-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	tcAssertHasAnyActiveApiCredential(t, store, ctx, false)
	db := store.(*DB)
	now := time.Now().UTC()
	rule := &models.AccessControlRule{
		AccessControlRuleBase: models.AccessControlRuleBase{
			SubjectType:     "user",
			SubjectID:       &user.ID,
			Action:          ActionApiCall,
			ResourceType:    ResourceTypeProviderOperation,
			ResourcePattern: "openai/chat",
			Effect:          "allow",
			Priority:        10,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(rule).Error; err != nil {
		t.Fatalf("create access_control_rule: %v", err)
	}
	cred := &models.ApiCredential{
		ApiCredentialBase: models.ApiCredentialBase{
			OwnerType:      "user",
			OwnerID:        user.ID,
			Provider:       "openai",
			CredentialType: "api_key",
			CredentialName: "default",
			IsActive:       true,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(cred).Error; err != nil {
		t.Fatalf("create api_credential: %v", err)
	}
	rules, err := store.ListAccessControlRulesForApiCall(ctx, "user", &user.ID, ActionApiCall, ResourceTypeProviderOperation)
	if err != nil {
		t.Fatalf("ListAccessControlRulesForApiCall: %v", err)
	}
	if len(rules) < 1 {
		t.Errorf("ListAccessControlRulesForApiCall: want at least one rule, got %d", len(rules))
	}
	hasCred, err := store.HasActiveApiCredentialForUserAndProvider(ctx, user.ID, "openai")
	if err != nil {
		t.Fatalf("HasActiveApiCredentialForUserAndProvider: %v", err)
	}
	if !hasCred {
		t.Error("HasActiveApiCredentialForUserAndProvider: want true")
	}
	tcAssertHasAnyActiveApiCredential(t, store, ctx, true)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.HasAnyActiveApiCredential(cancelCtx); err == nil {
		t.Error("HasAnyActiveApiCredential with canceled context: expected error")
	}
	auditRec := &models.AccessControlAuditLog{
		AccessControlAuditLogBase: models.AccessControlAuditLogBase{
			SubjectType:  "user",
			SubjectID:    &user.ID,
			Action:       ActionApiCall,
			ResourceType: ResourceTypeProviderOperation,
			Resource:     "openai/chat",
			Decision:     "allow",
			TaskID:       &task.ID,
		},
	}
	if err := store.CreateAccessControlAuditLog(ctx, auditRec); err != nil {
		t.Fatalf("CreateAccessControlAuditLog: %v", err)
	}
	if auditRec.ID == uuid.Nil || auditRec.CreatedAt.IsZero() {
		t.Error("CreateAccessControlAuditLog: expected ID and CreatedAt set")
	}
}

func TestRecord_ToDomainConversions_Smoke(t *testing.T) {
	t.Parallel()
	const recSubjectUser = "user"
	id := uuid.New()
	now := time.Now().UTC()
	uid := id

	pp := &ProjectPlanRecord{}
	pp.ID = id
	pp.CreatedAt = now
	pp.UpdatedAt = now
	pp.ProjectID = id
	pp.State = "active"
	pp.Archived = false
	if pp.ToProjectPlan() == nil {
		t.Fatal("ToProjectPlan")
	}

	mcp := &McpToolCallAuditLogRecord{}
	mcp.ID = id
	mcp.CreatedAt = now
	mcp.ToolName = "t"
	if mcp.ToMcpToolCallAuditLog() == nil {
		t.Fatal("ToMcpToolCallAuditLog")
	}

	pref := &PreferenceAuditLogRecord{}
	pref.ID = id
	pref.EntryID = id
	if pref.ToPreferenceAuditLog() == nil {
		t.Fatal("ToPreferenceAuditLog")
	}

	chat := &ChatAuditLogRecord{}
	chat.ID = id
	chat.CreatedAt = now
	chat.UpdatedAt = now
	chat.UserID = &uid
	chat.Outcome = "ok"
	if chat.ToChatAuditLog() == nil {
		t.Fatal("ToChatAuditLog")
	}

	ac := &AccessControlAuditLogRecord{}
	ac.ID = id
	ac.CreatedAt = now
	ac.SubjectType = recSubjectUser
	ac.Action = ActionApiCall
	if ac.ToAccessControlAuditLog() == nil {
		t.Fatal("ToAccessControlAuditLog")
	}
}
