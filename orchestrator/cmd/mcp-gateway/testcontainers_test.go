// Package main: run integration test with testcontainers Postgres to cover run() real-DB branch.
// Requires Podman. Set SKIP_TESTCONTAINERS=1 to skip container setup; set DATABASE_URL to use existing DB.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	mcpGatewayDBEnv             = "DATABASE_URL"
	testcontainersSetupTimeout  = 90 * time.Second
	testcontainersWaitForDB     = 60 * time.Second
	testcontainersRealDBListen  = "127.0.0.1:19084"
)

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

func runMCPGatewayTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, testcontainersSetupTimeout)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		writeTCErr(setupCtx, "postgres.Run failed: "+err.Error())
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		writeTCErr(setupCtx, "ConnectionString failed: "+err.Error())
		return container, false
	}
	connStr = dsnForceIPv4(connStr)
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(mcpGatewayDBEnv, connStr)
	if err := waitForPostgres(setupCtx, connStr); err != nil {
		writeTCErr(setupCtx, "postgres not ready: "+err.Error())
		return container, false
	}
	return container, true
}

func writeTCErr(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		_, _ = os.Stderr.WriteString("[mcp-gateway/testcontainers] setup timed out after " + testcontainersSetupTimeout.String() + "\n")
		return
	}
	_, _ = os.Stderr.WriteString("[mcp-gateway/testcontainers] " + msg + "\n")
}

func waitForPostgres(ctx context.Context, dsn string) error {
	deadline := time.Now().Add(testcontainersWaitForDB)
	for time.Now().Before(deadline) {
		db, err := database.Open(ctx, dsn)
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
	return fmt.Errorf("postgres not ready within %v", testcontainersWaitForDB)
}

type tcResult struct {
	container *postgres.PostgresContainer
	ok        bool
}

func TestMain(m *testing.M) {
	if os.Getenv(mcpGatewayDBEnv) != "" {
		os.Exit(m.Run())
		return
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
			_, _ = os.Stderr.WriteString("[mcp-gateway/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	hardTimeout := testcontainersSetupTimeout + 15*time.Second
	resultCh := make(chan tcResult, 1)
	go func() {
		c, ok := runMCPGatewayTestcontainersSetup(context.Background())
		resultCh <- tcResult{container: c, ok: ok}
	}()
	var ok bool
	select {
	case res := <-resultCh:
		container = res.container
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[mcp-gateway/testcontainers] setup did not complete within " + hardTimeout.String() + "\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

// TestRun_WithRealDatabase covers run() when DATABASE_URL is set and no test hooks: database.Open, RunSchema, server start.
// Requires testcontainers Postgres (or DATABASE_URL). Skips when DATABASE_URL is unset (e.g. SKIP_TESTCONTAINERS=1).
func TestRun_WithRealDatabase(t *testing.T) {
	dsn := os.Getenv(mcpGatewayDBEnv)
	if dsn == "" {
		t.Skip("DATABASE_URL not set (testcontainers skipped or not available)")
	}
	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", testcontainersRealDBListen)
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, slog.Default()) }()

	// Wait for server to be up
	baseURL := "http://" + testcontainersRealDBListen
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if i == 49 {
			cancel()
			<-done
			t.Fatalf("healthz did not become ready")
		}
	}

	// Hit tool-call endpoint to exercise audit path with real DB
	resp, err := http.Post(baseURL+"/v1/mcp/tools/call", "application/json", bytes.NewReader([]byte(`{"tool_name":"db.preference.get"}`)))
	if err != nil {
		cancel()
		<-done
		t.Fatalf("POST tool call: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("POST tool call: got status %d (expected 501)", resp.StatusCode)
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}
