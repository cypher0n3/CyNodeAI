// Package mcpgateway: Postgres via testcontainers for integration tests (mirrors internal/database TestMain).
package mcpgateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

const mcpIntegrationEnv = "POSTGRES_TEST_DSN"

const mcpTcSetupTimeout = 90 * time.Second

func setupRootlessPodmanHostMCP() {
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

func dsnForceIPv4MCP(dsn string) string {
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

func runMCPTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, mcpTcSetupTimeout)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		_, _ = os.Stderr.WriteString("[mcpgateway/testcontainers] postgres.Run failed: " + err.Error() + "\n")
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		_, _ = os.Stderr.WriteString("[mcpgateway/testcontainers] ConnectionString failed: " + err.Error() + "\n")
		return container, false
	}
	connStr = dsnForceIPv4MCP(connStr)
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(mcpIntegrationEnv, connStr)
	if err := waitForPostgresMCP(setupCtx, connStr, 60*time.Second); err != nil {
		_, _ = os.Stderr.WriteString("[mcpgateway/testcontainers] postgres not ready: " + err.Error() + "\n")
		return container, false
	}
	return container, true
}

func waitForPostgresMCP(ctx context.Context, dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
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
	return fmt.Errorf("postgres not ready within %v", timeout)
}

func TestMain(m *testing.M) {
	if dsn := os.Getenv(mcpIntegrationEnv); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := database.Open(ctx, dsn)
		cancel()
		if err == nil {
			_ = db.Close()
			os.Exit(m.Run())
			return
		}
		_ = os.Unsetenv(mcpIntegrationEnv)
	}
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHostMCP()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[mcpgateway/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	hardTimeout := mcpTcSetupTimeout + 15*time.Second
	ch := make(chan struct {
		c  *postgres.PostgresContainer
		ok bool
	}, 1)
	go func() {
		c, ok := runMCPTestcontainersSetup(context.Background())
		ch <- struct {
			c  *postgres.PostgresContainer
			ok bool
		}{c, ok}
	}()
	var ok bool
	select {
	case res := <-ch:
		container = res.c
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[mcpgateway/testcontainers] setup timeout\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

func tcMCPIntegrationDB(t *testing.T, ctx context.Context) *database.DB {
	t.Helper()
	if os.Getenv(mcpIntegrationEnv) == "" {
		t.Skip("postgres not available (set POSTGRES_TEST_DSN or run testcontainers)")
	}
	db, err := database.Open(ctx, os.Getenv(mcpIntegrationEnv))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	return db
}
