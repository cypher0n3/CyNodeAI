package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	readinesscheck "github.com/cypher0n3/cynodeai/orchestrator/internal/readiness"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

const testJobPayload = `{"command":["echo","hi"]}`

const testWorkerAPIURL = "http://localhost:9190"
const testWorkerAPIToken = "tok"
const expectedReadyBody = "ready"

func TestLoadDispatcherConfig(t *testing.T) {
	_ = os.Unsetenv("DISPATCHER_ENABLED")
	_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	_ = os.Unsetenv("WORKER_API_URL")
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	_ = os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
		_ = os.Unsetenv("WORKER_API_URL")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_HTTP_TIMEOUT")
	}()

	cfg := loadDispatcherConfig()
	if !cfg.Enabled {
		t.Error("default Enabled should be true")
	}
	if cfg.PollInterval != 1*time.Second {
		t.Errorf("default PollInterval: %v", cfg.PollInterval)
	}

	_ = os.Setenv("DISPATCHER_ENABLED", "false")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "2s")
	cfg2 := loadDispatcherConfig()
	if cfg2.Enabled {
		t.Error("DISPATCHER_ENABLED=false should set Enabled false")
	}
	if cfg2.PollInterval != 2*time.Second {
		t.Errorf("DISPATCH_POLL_INTERVAL: %v", cfg2.PollInterval)
	}
}

func TestGetDurationEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_DISPATCH_DURATION")
	if getDurationEnv("TEST_DISPATCH_DURATION", 10*time.Second) != 10*time.Second {
		t.Error("default")
	}
	_ = os.Setenv("TEST_DISPATCH_DURATION", "3m")
	defer func() { _ = os.Unsetenv("TEST_DISPATCH_DURATION") }()
	if getDurationEnv("TEST_DISPATCH_DURATION", time.Second) != 3*time.Minute {
		t.Error("parse 3m")
	}
}

func TestGetDurationEnv_InvalidValue(t *testing.T) {
	_ = os.Setenv("TEST_DISPATCH_DURATION_BAD", "not-a-duration")
	defer func() { _ = os.Unsetenv("TEST_DISPATCH_DURATION_BAD") }()
	if getDurationEnv("TEST_DISPATCH_DURATION_BAD", 7*time.Second) != 7*time.Second {
		t.Error("invalid duration should return default")
	}
}

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_CP_ENV")
	if getEnv("TEST_CP_ENV", "default") != "default" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_CP_ENV", "value")
	defer func() { _ = os.Unsetenv("TEST_CP_ENV") }()
	if getEnv("TEST_CP_ENV", "default") != "value" {
		t.Error("from env")
	}
}

func TestRegisterMCPToolRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerMCPToolRoute(mux, testutil.NewMockDB(), slog.Default(), config.LoadOrchestratorConfig())
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", strings.NewReader(`{"tool_name":"help.list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Fatal("expected POST /v1/mcp/tools/call to be registered on the control-plane mux")
	}
}

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	healthzHandler(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("healthz: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("healthz Content-Type: got %q", ct)
	}
	if strings.TrimSpace(w.Body.String()) != "ok" {
		t.Errorf("healthz body: got %q", w.Body.String())
	}
}

func TestReadyzHandler_NoNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	cfg := &config.OrchestratorConfig{PMAEnabled: false}
	handler := readyzHandler(mock, cfg, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz (no nodes): got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("no inference path")) {
		t.Errorf("readyz body should contain 'no inference path', got %s", w.Body.String())
	}
}

func TestReadyzHandler_WithNode(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	url, token := testWorkerAPIURL, testWorkerAPIToken
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, url, token)
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "01HXYZ", "applied", time.Now().UTC(), nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, "active")
	// Worker-reported PMA ready so readyz can return 200 (prescribed path).
	capWithPMAReady := `{"version":1,"managed_services_status":{"services":[{"service_id":"pma-main","service_type":"pma","state":"ready","endpoints":["http://127.0.0.1:8090"]}]}}`
	_ = mock.SaveNodeCapabilitySnapshot(ctx, node.ID, capWithPMAReady)
	cfg := &config.OrchestratorConfig{PMAEnabled: false}
	handler := readyzHandler(mock, cfg, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("readyz (with node): got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != expectedReadyBody {
		t.Errorf("readyz body want 'ready', got %s", w.Body.String())
	}
}

func TestReadyzHandler_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	cfg := &config.OrchestratorConfig{PMAEnabled: false}
	handler := readyzHandler(mock, cfg, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz (db error): got %d", w.Code)
	}
}

func TestReadyzHandler_PMANotReady(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	url, token := testWorkerAPIURL, testWorkerAPIToken
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, url, token)
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "01HXYZ", "applied", time.Now().UTC(), nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, "active")
	cfg := &config.OrchestratorConfig{PMAEnabled: true, PMAListenAddr: ":19999"}
	handler := readyzHandler(mock, cfg, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz (PMA not ready): got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("PMA not ready")) {
		t.Errorf("readyz body should contain 'PMA not ready', got %s", w.Body.String())
	}
}

func TestPmaReady_InvalidListenAddr(t *testing.T) {
	ctx := context.Background()
	if readinesscheck.PMASubprocessReady(ctx, "") {
		t.Error("PMASubprocessReady with empty addr should return false")
	}
	if readinesscheck.PMASubprocessReady(ctx, "no-port") {
		t.Error("PMASubprocessReady with invalid addr should return false")
	}
}

func TestReadyzHandler_WithNode_NilConfig(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, testWorkerAPIURL, testWorkerAPIToken)
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "01HXYZ", "applied", time.Now().UTC(), nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, "active")
	// Worker-reported PMA ready so readyz can return 200.
	capWithPMAReady := `{"version":1,"managed_services_status":{"services":[{"service_id":"pma-main","service_type":"pma","state":"ready","endpoints":["http://127.0.0.1:8090"]}]}}`
	_ = mock.SaveNodeCapabilitySnapshot(ctx, node.ID, capWithPMAReady)
	handler := readyzHandler(mock, nil, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("readyz (nil cfg, with node): got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != expectedReadyBody {
		t.Errorf("readyz body want 'ready', got %s", w.Body.String())
	}
}

func TestReadyzHandler_PMAReady(t *testing.T) {
	pmaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer pmaServer.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(pmaServer.URL, "http://"))
	if err != nil {
		t.Fatalf("parse PMA server URL: %v", err)
	}
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	workerURL, token := testWorkerAPIURL, testWorkerAPIToken
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerURL, token)
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "01HXYZ", "applied", time.Now().UTC(), nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, "active")
	cfg := &config.OrchestratorConfig{PMAEnabled: true, PMAListenAddr: ":" + port}
	handler := readyzHandler(mock, cfg, slog.Default())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("readyz (PMA ready): got %d %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != expectedReadyBody {
		t.Errorf("readyz body want 'ready', got %s", w.Body.String())
	}
}

// newWorkerServerOK returns a test server that responds 200 with the given worker response. Caller must defer server.Close().
func newWorkerServerOK(workerResp *workerapi.RunJobResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(workerResp)
	}))
}

// makeDispatchableNode sets the node as active with config ack applied and worker API URL/token (for ListDispatchableNodes).
func makeDispatchableNode(t *testing.T, mock *testutil.MockDB, ctx context.Context, node *models.Node, workerURL, bearerToken string) {
	t.Helper()
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerURL, bearerToken)
	ackAt := time.Now().UTC()
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
}

func TestDispatchOnce_Success(t *testing.T) {
	workerResp := workerapi.RunJobResponse{
		Version: 1, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: workerapi.ExitCodePtr(0),
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	}
	server := newWorkerServerOK(&workerResp)
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "prompt", nil)
	payload := testJobPayload
	job, _ := mock.CreateJob(ctx, task.ID, payload)
	node, _ := mock.CreateNode(ctx, "node-1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "token")

	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce: %v", err)
	}

	// Job should be completed
	j, _ := mock.GetJobByID(ctx, job.ID)
	if j.Status != models.JobStatusCompleted {
		t.Errorf("job status %s", j.Status)
	}
}

func TestDispatchOnce_NoDispatchableNodes(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	// No worker API config or config ack -> not dispatchable

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected error when no dispatchable nodes")
	}
}

func TestDispatchOnce_ErrNotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDispatchOnce_NoNodes(t *testing.T) {
	payload := testJobPayload
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, payload)
	// No active node

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()

	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err == nil {
		t.Fatal("expected no active nodes error")
	}
}

func TestStartDispatcher_Disabled(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "false")
	defer func() { _ = os.Unsetenv("DISPATCHER_ENABLED") }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	startDispatcher(ctx, mock, logger)
}

func TestStartDispatcher_NoToken(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mock := testutil.NewMockDB()
	logger := slog.Default()
	// Dispatcher no longer exits early when token unset (uses per-node token); run in goroutine and cancel.
	go startDispatcher(ctx, mock, logger)
	<-time.After(25 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

func TestStartDispatcher_EnabledOneTick(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "test-token")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "1ms")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	}()
	ctx, cancel := context.WithCancel(context.Background())
	mock := testutil.NewMockDB()
	logger := slog.Default()
	go startDispatcher(ctx, mock, logger)
	// Allow one tick (dispatchOnce returns ErrNotFound when queue empty)
	<-time.After(20 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

// listDispatchableNodesErrorStore fails ListDispatchableNodes so dispatchOnce returns non-ErrNotFound error.
type listDispatchableNodesErrorStore struct {
	*testutil.MockDB
}

func (m *listDispatchableNodesErrorStore) ListDispatchableNodes(_ context.Context) ([]*models.Node, error) {
	return nil, errors.New("list nodes error")
}

func (m *listDispatchableNodesErrorStore) ListActiveNodes(ctx context.Context) ([]*models.Node, error) {
	return m.MockDB.ListActiveNodes(ctx)
}

// listDispatchableNodesFlakyStore fails ListDispatchableNodes until setListError(false) is called.
// It avoids unsafely mutating MockDB.ForceError from another goroutine in race tests.
type listDispatchableNodesFlakyStore struct {
	*testutil.MockDB
	mu        sync.RWMutex
	listError bool
}

func newListDispatchableNodesFlakyStore() *listDispatchableNodesFlakyStore {
	return &listDispatchableNodesFlakyStore{
		MockDB:    testutil.NewMockDB(),
		listError: true,
	}
}

func (m *listDispatchableNodesFlakyStore) setListError(enabled bool) {
	m.mu.Lock()
	m.listError = enabled
	m.mu.Unlock()
}

func (m *listDispatchableNodesFlakyStore) ListDispatchableNodes(ctx context.Context) ([]*models.Node, error) {
	m.mu.RLock()
	listError := m.listError
	m.mu.RUnlock()
	if listError {
		return nil, errors.New("list failed")
	}
	return m.MockDB.ListDispatchableNodes(ctx)
}

func TestStartDispatcher_DispatchOnceReturnsError(t *testing.T) {
	_ = os.Setenv("DISPATCHER_ENABLED", "true")
	_ = os.Setenv("WORKER_API_BEARER_TOKEN", "token")
	_ = os.Setenv("DISPATCH_POLL_INTERVAL", "1ms")
	defer func() {
		_ = os.Unsetenv("DISPATCHER_ENABLED")
		_ = os.Unsetenv("WORKER_API_BEARER_TOKEN")
		_ = os.Unsetenv("DISPATCH_POLL_INTERVAL")
	}()
	base := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := base.CreateTask(ctx, nil, "p", nil)
	_, _ = base.CreateJob(ctx, task.ID, `{"command":["x"]}`)
	mock := &listDispatchableNodesErrorStore{MockDB: base}
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.Default()
	go startDispatcher(ctx, mock, logger)
	<-time.After(25 * time.Millisecond)
	cancel()
	<-time.After(10 * time.Millisecond)
}

func TestBootstrapAdminUser_AlreadyExists(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	_, _ = mock.CreateUser(ctx, "admin", nil)
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "password", logger)
	if err != nil {
		t.Fatalf("bootstrapAdminUser: %v", err)
	}
}

func TestBootstrapAdminUser_Create(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "adminpass", logger)
	if err != nil {
		t.Fatalf("bootstrapAdminUser: %v", err)
	}
	u, err := mock.GetUserByHandle(ctx, "admin")
	if err != nil || u == nil {
		t.Fatalf("admin user not created: %v", err)
	}
}

func TestBootstrapAdminUser_GetUserError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, mock, "adminpass", logger)
	if err == nil {
		t.Fatal("expected error when GetUserByHandle fails with non-ErrNotFound")
	}
}

// createUserErrorStore fails CreateUser (bootstrapAdminUser path).
type createUserErrorStore struct {
	*testutil.MockDB
}

func (m *createUserErrorStore) CreateUser(_ context.Context, _ string, _ *string) (*models.User, error) {
	return nil, errors.New("create user error")
}

// createPasswordCredErrorStore fails CreatePasswordCredential (bootstrapAdminUser path).
type createPasswordCredErrorStore struct {
	*testutil.MockDB
}

func (m *createPasswordCredErrorStore) CreatePasswordCredential(_ context.Context, _ uuid.UUID, _ []byte, _ string) (*models.PasswordCredential, error) {
	return nil, errors.New("create credential error")
}

func TestBootstrapAdminUser_CreateUserFails(t *testing.T) {
	testBootstrapAdminUserFails(t, &createUserErrorStore{MockDB: testutil.NewMockDB()}, "CreateUser")
}

func TestBootstrapAdminUser_CreatePasswordCredentialFails(t *testing.T) {
	testBootstrapAdminUserFails(t, &createPasswordCredErrorStore{MockDB: testutil.NewMockDB()}, "CreatePasswordCredential")
}

func testBootstrapAdminUserFails(t *testing.T, store database.Store, which string) {
	t.Helper()
	ctx := context.Background()
	logger := slog.Default()
	err := bootstrapAdminUser(ctx, store, "adminpass", logger)
	if err == nil {
		t.Fatalf("expected error when %s fails", which)
	}
}

func TestRun_BootstrapAdminUserFails(t *testing.T) {
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	mock := &createPasswordCredErrorStore{MockDB: testutil.NewMockDB()}
	logger := slog.Default()
	err := run(ctx, mock, cfg, logger)
	if err == nil {
		t.Fatal("run should return error when bootstrapAdminUser fails")
	}
}

func TestRun_ListenAndServeFails(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = false
	mock := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, mock, cfg, logger)
	if err == nil {
		t.Fatal("run should return error when ListenAndServe fails (invalid port)")
	}
}

func TestDispatchOnce_InvalidPayload(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, "not-valid-json")
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, testWorkerAPIURL, "token")

	cfg := dispatcherConfig{}
	client := &http.Client{}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce with bad payload should complete job as failed: %v", err)
	}
}

func runDispatchOnceWithWorkerStatus(t *testing.T, statusCode int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	defer server.Close()
	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "t")
	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed: %v", err)
	}
}

func TestDispatchOnce_WorkerAPIError(t *testing.T) {
	runDispatchOnceWithWorkerStatus(t, http.StatusInternalServerError)
}

func TestDispatchOnce_WorkerAPIBadVersion(t *testing.T) {
	server := newWorkerServerOK(&workerapi.RunJobResponse{
		Version: 0, TaskID: "t1", JobID: "j1",
		Status: workerapi.StatusCompleted, ExitCode: workerapi.ExitCodePtr(0),
		StartedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:01Z",
	})
	defer server.Close()

	mock := testutil.NewMockDB()
	ctx := context.Background()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	_, _ = mock.CreateJob(ctx, task.ID, testJobPayload)
	node, _ := mock.CreateNode(ctx, "n1")
	makeDispatchableNode(t, mock, ctx, node, server.URL, "t")

	cfg := dispatcherConfig{HTTPTimeout: 5 * time.Second}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logger := slog.Default()
	err := dispatcher.RunOnce(ctx, mock, client, cfg.HTTPTimeout, logger)
	if err != nil {
		t.Fatalf("dispatchOnce should complete job as failed on bad version: %v", err)
	}
}

func TestRun_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = false
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, mockDB, cfg, logger)
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

// TestRun_ShutdownSucceeds covers the shutdown success path (server started, then ctx canceled, shutdown succeeds).
func TestRun_ShutdownSucceeds(t *testing.T) {
	oldListen := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() {
		if oldListen != "" {
			_ = os.Setenv("LISTEN_ADDR", oldListen)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = false
	mockDB := testutil.NewMockDB()
	logger := slog.Default()

	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()

	// Allow server to start and dispatcher to tick once.
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-done
	if err != nil {
		t.Errorf("run after cancel: %v", err)
	}
}

// TestRun_PMAStartedAndStopped runs with PMA enabled and a quick-exit binary so the start and defer stop path is covered.
// Adds a dispatchable node so startPMAWhenInferencePathReady (REQ-ORCHES-0150) sees an inference path and starts PMA.
func TestRun_PMAStartedAndStopped(t *testing.T) {
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not in PATH, skipping PMA test")
	}
	oldListen := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() {
		if oldListen != "" {
			_ = os.Setenv("LISTEN_ADDR", oldListen)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	cfg.PMABinaryPath = path
	mockDB := testutil.NewMockDB()
	node, err := mockDB.CreateNode(ctx, "pma-test-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
	_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
	ackAt := time.Now().UTC()
	_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	logger := slog.Default()

	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()

	time.Sleep(150 * time.Millisecond)
	cancel()

	err = <-done
	if err != nil {
		t.Errorf("run with PMA after cancel: %v", err)
	}
}

// TestRun_PMAStartWhenInferencePathReady_StartFails covers startPMAWhenInferencePathReady when Start returns an error.
func TestRun_PMAStartWhenInferencePathReady_StartFails(t *testing.T) {
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	testPMAStart = func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error) {
		return nil, errors.New("start failed")
	}
	defer func() { testPMAStart = nil }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	mockDB := testutil.NewMockDB()
	node, _ := mockDB.CreateNode(ctx, "n1")
	_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
	_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
	ackAt := time.Now().UTC()
	_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
}

// TestRun_PMAStartWhenInferencePathReady_WithRealCmd covers the cmd != nil path by returning a started process from testPMAStart.
func TestRun_PMAStartWhenInferencePathReady_WithRealCmd(t *testing.T) {
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not in PATH")
	}
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	testPMAStart = func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error) {
		c := exec.Command(path)
		if err := c.Start(); err != nil {
			return nil, err
		}
		return c, nil
	}
	defer func() { testPMAStart = nil }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	mockDB := testutil.NewMockDB()
	node, _ := mockDB.CreateNode(ctx, "n1")
	_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
	_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
	ackAt := time.Now().UTC()
	_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()
	time.Sleep(500 * time.Millisecond)
	cancel()
	err = <-done
	if err != nil {
		t.Errorf("run: %v", err)
	}
}

// TestRun_PMAStartWhenInferencePathReady covers startPMAWhenInferencePathReady when store returns dispatchable nodes (REQ-ORCHES-0150).
func TestRun_PMAStartWhenInferencePathReady(t *testing.T) {
	oldListen := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() {
		if oldListen != "" {
			_ = os.Setenv("LISTEN_ADDR", oldListen)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	testPMAStart = func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error) {
		return nil, nil
	}
	defer func() { testPMAStart = nil }()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	node, err := mockDB.CreateNode(ctx, "dispatchable-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	if err := mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1"); err != nil {
		t.Fatalf("UpdateNodeConfigVersion: %v", err)
	}
	if err := mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok"); err != nil {
		t.Fatalf("UpdateNodeWorkerAPIConfig: %v", err)
	}
	ackAt := time.Now().UTC()
	if err := mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	err = <-done
	if err != nil {
		t.Errorf("run with PMA when inference path ready: %v", err)
	}
}

// TestStartPMAWhenInferencePathReady_Direct covers startPMAWhenInferencePathReady synchronously so the cmd != nil path is hit.
func TestStartPMAWhenInferencePathReady_Direct(t *testing.T) {
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not in PATH")
	}
	testPMAStart = func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error) {
		c := exec.Command(path)
		if err := c.Start(); err != nil {
			return nil, err
		}
		return c, nil
	}
	defer func() { testPMAStart = nil }()
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	mockDB := testutil.NewMockDB()
	node, _ := mockDB.CreateNode(ctx, "n1")
	_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
	_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
	ackAt := time.Now().UTC()
	_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	logger := slog.Default()
	var mu sync.Mutex
	var pmaCmd *exec.Cmd
	startPMAWhenInferencePathReady(ctx, mockDB, cfg, logger, &mu, &pmaCmd)
	mu.Lock()
	c := pmaCmd
	mu.Unlock()
	if c == nil || c.Process == nil {
		t.Error("expected pmaCmd to be set with a started process")
	}
	if c != nil && c.Process != nil {
		_ = c.Process.Kill()
		_ = c.Wait()
	}
}

// TestWaitForInferencePath_ContextCanceled covers waitForInferencePath returning false when ctx is canceled.
func TestWaitForInferencePath_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mockDB := testutil.NewMockDB()
	got := waitForInferencePath(ctx, mockDB, slog.Default())
	if got {
		t.Error("waitForInferencePath with canceled ctx should return false")
	}
}

// TestWaitForInferencePath_NodeAvailable covers waitForInferencePath returning true when store returns a node.
func TestWaitForInferencePath_NodeAvailable(t *testing.T) {
	testPMAPollInterval = 1 * time.Millisecond
	defer func() { testPMAPollInterval = 0 }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockDB := testutil.NewMockDB()
	node, _ := mockDB.CreateNode(ctx, "n1")
	_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
	_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
	ackAt := time.Now().UTC()
	_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	got := waitForInferencePath(ctx, mockDB, slog.Default())
	if !got {
		t.Error("waitForInferencePath with dispatchable node should return true")
	}
}

// TestWaitForInferencePath_ListFailsThenNode covers the error path and then success.
func TestWaitForInferencePath_ListFailsThenNode(t *testing.T) {
	testPMAPollInterval = 1 * time.Millisecond
	defer func() { testPMAPollInterval = 0 }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockDB := newListDispatchableNodesFlakyStore()
	go func() {
		time.Sleep(5 * time.Millisecond)
		mockDB.setListError(false)
		node, _ := mockDB.CreateNode(ctx, "n1")
		_ = mockDB.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
		_ = mockDB.UpdateNodeConfigVersion(ctx, node.ID, "1")
		_ = mockDB.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:1", "tok")
		ackAt := time.Now().UTC()
		_ = mockDB.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	}()
	got := waitForInferencePath(ctx, mockDB, slog.Default())
	if !got {
		t.Error("waitForInferencePath should eventually return true after node added")
	}
}

// TestRun_PMAStartWhenInferencePathReady_ListNodesFails covers the branch where ListDispatchableNodes returns an error.
func TestRun_PMAStartWhenInferencePathReady_ListNodesFails(t *testing.T) {
	_ = os.Setenv("LISTEN_ADDR", ":0")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	testPMAStart = func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error) { return nil, nil }
	testPMAPollInterval = 5 * time.Millisecond
	defer func() { testPMAStart = nil; testPMAPollInterval = 0 }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	cfg.PMAEnabled = true
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("list nodes failed")
	defer func() { mockDB.ForceError = nil }()
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, mockDB, cfg, logger) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done
}
