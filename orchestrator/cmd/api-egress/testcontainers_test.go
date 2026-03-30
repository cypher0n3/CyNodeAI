// Package main: integration test with testcontainers Postgres to cover run() when API_EGRESS_DSN is set (ApplyWorkerBearerEncryptionAtStartup).
// Requires Podman. Set SKIP_TESTCONTAINERS=1 to skip; set API_EGRESS_DSN to use an existing DB.
package main

import (
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
	apiEgressDBEnv                = "API_EGRESS_DSN"
	testcontainersSetupTimeoutAE  = 90 * time.Second
	testcontainersWaitForDBAE     = 60 * time.Second
	testcontainersAPIEgressListen = "127.0.0.1:18087"
)

func setupRootlessPodmanHostAE() {
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

func dsnForceIPv4AE(dsn string) string {
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

func runAPITestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, testcontainersSetupTimeoutAE)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		writeTCErrAE(setupCtx, "postgres.Run failed: "+err.Error())
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		writeTCErrAE(setupCtx, "ConnectionString failed: "+err.Error())
		return container, false
	}
	connStr = dsnForceIPv4AE(connStr)
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(apiEgressDBEnv, connStr)
	if err := waitForPostgresAE(setupCtx, connStr); err != nil {
		writeTCErrAE(setupCtx, "postgres not ready: "+err.Error())
		return container, false
	}
	return container, true
}

func writeTCErrAE(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		_, _ = os.Stderr.WriteString("[api-egress/testcontainers] setup timed out after " + testcontainersSetupTimeoutAE.String() + "\n")
		return
	}
	_, _ = os.Stderr.WriteString("[api-egress/testcontainers] " + msg + "\n")
}

func waitForPostgresAE(ctx context.Context, dsn string) error {
	deadline := time.Now().Add(testcontainersWaitForDBAE)
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
	return fmt.Errorf("postgres not ready within %v", testcontainersWaitForDBAE)
}

type tcResultAE struct {
	container *postgres.PostgresContainer
	ok        bool
}

func TestMain(m *testing.M) {
	if os.Getenv(apiEgressDBEnv) != "" {
		os.Exit(m.Run())
		return
	}
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHostAE()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[api-egress/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	hardTimeout := testcontainersSetupTimeoutAE + 15*time.Second
	resultCh := make(chan tcResultAE, 1)
	go func() {
		var c *postgres.PostgresContainer
		var ok bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					_, _ = os.Stderr.WriteString("[api-egress/testcontainers] setup panic: " + fmt.Sprint(r) + "\n")
					c, ok = nil, false
				}
			}()
			c, ok = runAPITestcontainersSetup(context.Background())
		}()
		resultCh <- tcResultAE{container: c, ok: ok}
	}()
	var ok bool
	select {
	case res := <-resultCh:
		container = res.container
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[api-egress/testcontainers] setup did not complete within " + hardTimeout.String() + "\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

func pollHealthzReadyAE(t *testing.T, baseURL string) bool {
	t.Helper()
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
	}
	return false
}

// TestRun_WithRealDatabase covers run when API_EGRESS_DSN is set: Open, RunSchema, ApplyWorkerBearerEncryptionAtStartup.
func TestRun_WithRealDatabase(t *testing.T) {
	dsn := os.Getenv(apiEgressDBEnv)
	if dsn == "" {
		t.Skip("API_EGRESS_DSN not set (testcontainers skipped or not available)")
	}
	testAPIEgressDatabaseOpen = func(ctx context.Context, openDSN string) (*database.DB, error) {
		db, err := database.Open(ctx, openDSN)
		if err != nil {
			return nil, err
		}
		if err := db.RunSchema(ctx, slog.Default()); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}
	defer func() { testAPIEgressDatabaseOpen = nil }()

	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", testcontainersAPIEgressListen)
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

	baseURL := "http://" + testcontainersAPIEgressListen
	if !pollHealthzReadyAE(t, baseURL) {
		cancel()
		<-done
		t.Fatalf("healthz did not become ready")
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("run: %v", err)
	}
}
