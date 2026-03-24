package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// readyzErrorStore implements database.Store by delegating to MockDB but returning an error from ListDispatchableNodes.
type readyzErrorStore struct {
	*testutil.MockDB
	listErr error
}

func (s *readyzErrorStore) ListDispatchableNodes(ctx context.Context) ([]*models.Node, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.MockDB.ListDispatchableNodes(ctx)
}

func TestGetEnv(t *testing.T) {
	_ = os.Unsetenv("TEST_UG_ENV")
	if getEnv("TEST_UG_ENV", "def") != "def" {
		t.Error("default")
	}
	_ = os.Setenv("TEST_UG_ENV", "val")
	defer func() { _ = os.Unsetenv("TEST_UG_ENV") }()
	if getEnv("TEST_UG_ENV", "def") != "val" {
		t.Error("from env")
	}
}

func TestLimitBody(t *testing.T) {
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}
	wrapped := limitBody(100, next)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("small"))
	rec := httptest.NewRecorder()
	wrapped(rec, req)
	if !called {
		t.Error("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d", rec.Code)
	}
}

func TestGetEnv_USER_GATEWAY_LISTEN_ADDR(t *testing.T) {
	// run() uses getEnv("USER_GATEWAY_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8080"))
	_ = os.Unsetenv("USER_GATEWAY_LISTEN_ADDR")
	_ = os.Unsetenv("LISTEN_ADDR")
	if getEnv("LISTEN_ADDR", ":8080") != ":8080" {
		t.Error("LISTEN_ADDR default")
	}
	_ = os.Setenv("LISTEN_ADDR", ":9090")
	defer func() { _ = os.Unsetenv("LISTEN_ADDR") }()
	if getEnv("USER_GATEWAY_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8080")) != ":9090" {
		t.Error("USER_GATEWAY_LISTEN_ADDR should fall back to LISTEN_ADDR")
	}
	_ = os.Setenv("USER_GATEWAY_LISTEN_ADDR", ":7070")
	defer func() { _ = os.Unsetenv("USER_GATEWAY_LISTEN_ADDR") }()
	if getEnv("USER_GATEWAY_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8080")) != ":7070" {
		t.Error("USER_GATEWAY_LISTEN_ADDR should take precedence")
	}
}

func TestRun_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, cfg, mockDB, logger)
	if err != nil {
		t.Errorf("run with canceled context: %v", err)
	}
}

func TestRun_EnsureDefaultSkillErrorStillStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	mockDB.EnsureDefaultSkillErr = errors.New("skill unavailable")
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, mockDB, logger) }()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not exit")
	}
}

func TestRun_StartAndShutdown(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":18080")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, mockDB, logger) }()
	time.Sleep(80 * time.Millisecond)
	resp, err := http.Get("http://127.0.0.1:18080/healthz")
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("healthz: got %d", resp.StatusCode)
		}
	}
	readyResp, readyErr := http.Get("http://127.0.0.1:18080/readyz")
	if readyErr == nil {
		_ = readyResp.Body.Close()
		if readyResp.StatusCode != http.StatusOK && readyResp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("readyz: got %d", readyResp.StatusCode)
		}
	}
	authResp, authErr := http.Get("http://127.0.0.1:18080/v1/models")
	if authErr == nil {
		_ = authResp.Body.Close()
		if authResp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET /v1/models without auth: got %d", authResp.StatusCode)
		}
	}
	cancel()
	time.Sleep(150 * time.Millisecond)
	select {
	case <-done:
	default:
		t.Log("run may still be shutting down")
	}
}

func TestRun_ShutdownHookReturnsError(t *testing.T) {
	testShutdownHook = func(_ *http.Server, _ context.Context) error {
		return errors.New("shutdown failed")
	}
	defer func() { testShutdownHook = nil }()
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":18081")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, mockDB, logger) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected shutdown error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return")
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	err := run(ctx, cfg, mockDB, logger)
	if err == nil {
		t.Error("expected error when ListenAndServe fails (invalid port)")
	}
}

func TestRunMain_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	code := runMainWithStore(ctx, cfg, mockDB, logger)
	if code != 0 {
		t.Errorf("runMainWithStore success path: got %d", code)
	}
}

func TestRunMain_DBOpenFails(t *testing.T) {
	oldDSN := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/none?sslmode=disable")
	defer func() {
		if oldDSN != "" {
			_ = os.Setenv("DATABASE_URL", oldDSN)
		} else {
			_ = os.Unsetenv("DATABASE_URL")
		}
	}()
	code := runMain(context.Background())
	if code != 1 {
		t.Errorf("runMain with bad DB: got %d", code)
	}
}

func TestRunMain_RunFails(t *testing.T) {
	if pg := os.Getenv("POSTGRES_TEST_DSN"); pg != "" {
		oldDSN := os.Getenv("DATABASE_URL")
		_ = os.Setenv("DATABASE_URL", pg)
		defer func() {
			if oldDSN != "" {
				_ = os.Setenv("DATABASE_URL", oldDSN)
			} else {
				_ = os.Unsetenv("DATABASE_URL")
			}
		}()
	}
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code := runMain(ctx)
	if code != 1 {
		t.Errorf("runMain when run fails: got %d", code)
	}
}

func TestRunMainWithStore_RunFails(t *testing.T) {
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", ":99999")
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.LoadOrchestratorConfig()
	mockDB := testutil.NewMockDB()
	logger := slog.Default()
	code := runMainWithStore(ctx, cfg, mockDB, logger)
	if code != 1 {
		t.Errorf("runMainWithStore when run fails: got %d", code)
	}
}

func TestGatewayReadyzHandler_NoInferencePath_Returns503(t *testing.T) {
	mock := testutil.NewMockDB()
	handler := gatewayReadyzHandler(mock, nil, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when no inference path, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGatewayReadyzHandler_WithNodeAndPMAReady_Returns200(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n1")
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://worker:12090", "token")
	ack := time.Now().UTC()
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ack, nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	capWithPMAReady := `{"version":1,"managed_services_status":{"services":[{"service_id":"pma","service_type":"pma","state":"ready","endpoints":["http://127.0.0.1:8090"]}]}}`
	_ = mock.SaveNodeCapabilitySnapshot(ctx, node.ID, capWithPMAReady)

	handler := gatewayReadyzHandler(mock, nil, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when node+PMA ready, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGatewayReadyzHandler_InferencePathButNoPMA_Returns503(t *testing.T) {
	mock := testutil.NewMockDB()
	ctx := context.Background()
	node, _ := mock.CreateNode(ctx, "n2")
	_ = mock.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://worker:12091", "token")
	ack := time.Now().UTC()
	_ = mock.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ack, nil)
	_ = mock.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive)
	// No PMA in capability snapshot

	handler := gatewayReadyzHandler(mock, nil, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when inference path but PMA not ready, got %d", w.Code)
	}
	if w.Body.String() != "PMA not ready (no local PMA and no worker has reported PMA ready)" {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestGatewayReadyzHandler_StoreError_Returns503(t *testing.T) {
	store := &readyzErrorStore{MockDB: testutil.NewMockDB(), listErr: errors.New("db error")}
	handler := gatewayReadyzHandler(store, nil, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when store errors, got %d", w.Code)
	}
	if w.Body.String() != "readiness check failed (database error)" {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}
