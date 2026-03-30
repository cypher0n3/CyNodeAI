// Package main: integration test with testcontainers Postgres to cover runMain() Open + ApplyWorkerBearerEncryptionAtStartup + server start.
// Requires Podman or Docker. Set SKIP_TESTCONTAINERS=1 to skip container setup; set DATABASE_URL to use an existing DB.
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
	userGatewayDBEnv                = "DATABASE_URL"
	testcontainersSetupTimeout      = 90 * time.Second
	testcontainersWaitForDB         = 60 * time.Second
	testcontainersUserGatewayListen = "127.0.0.1:18086"
)

func setupRootlessPodmanHostUserGW() {
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

func dsnForceIPv4UserGW(dsn string) string {
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

func runUserGatewayTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
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
		writeTCErrUserGW(setupCtx, "postgres.Run failed: "+err.Error())
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		writeTCErrUserGW(setupCtx, "ConnectionString failed: "+err.Error())
		return container, false
	}
	connStr = dsnForceIPv4UserGW(connStr)
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(userGatewayDBEnv, connStr)
	if err := waitForPostgresUserGW(setupCtx, connStr); err != nil {
		writeTCErrUserGW(setupCtx, "postgres not ready: "+err.Error())
		return container, false
	}
	return container, true
}

func writeTCErrUserGW(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		_, _ = os.Stderr.WriteString("[user-gateway/testcontainers] setup timed out after " + testcontainersSetupTimeout.String() + "\n")
		return
	}
	_, _ = os.Stderr.WriteString("[user-gateway/testcontainers] " + msg + "\n")
}

func waitForPostgresUserGW(ctx context.Context, dsn string) error {
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

type tcResultUserGW struct {
	container *postgres.PostgresContainer
	ok        bool
}

func TestMain(m *testing.M) {
	if os.Getenv(userGatewayDBEnv) != "" {
		os.Exit(m.Run())
		return
	}
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHostUserGW()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[user-gateway/testcontainers] panic: " + fmt.Sprint(r) + "\n")
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
	resultCh := make(chan tcResultUserGW, 1)
	go func() {
		var c *postgres.PostgresContainer
		var ok bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					_, _ = os.Stderr.WriteString("[user-gateway/testcontainers] setup panic: " + fmt.Sprint(r) + "\n")
					c, ok = nil, false
				}
			}()
			c, ok = runUserGatewayTestcontainersSetup(context.Background())
		}()
		resultCh <- tcResultUserGW{container: c, ok: ok}
	}()
	var ok bool
	select {
	case res := <-resultCh:
		container = res.container
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[user-gateway/testcontainers] setup did not complete within " + hardTimeout.String() + "\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

func pollHealthzReadyUserGW(t *testing.T, baseURL string) bool {
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

// TestRunMain_WithRealDatabase covers runMain when DATABASE_URL is set: database.Open, ApplyWorkerBearerEncryptionAtStartup, server start.
// Requires testcontainers Postgres (or DATABASE_URL). Skips when DATABASE_URL is unset (e.g. SKIP_TESTCONTAINERS=1).
func TestRunMain_WithRealDatabase(t *testing.T) {
	dsn := os.Getenv(userGatewayDBEnv)
	if dsn == "" {
		t.Skip("DATABASE_URL not set (testcontainers skipped or not available)")
	}
	testDatabaseOpen = func(ctx context.Context, openDSN string) (*database.DB, error) {
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
	defer func() { testDatabaseOpen = nil }()

	oldAddr := os.Getenv("LISTEN_ADDR")
	_ = os.Setenv("LISTEN_ADDR", testcontainersUserGatewayListen)
	defer func() {
		if oldAddr != "" {
			_ = os.Setenv("LISTEN_ADDR", oldAddr)
		} else {
			_ = os.Unsetenv("LISTEN_ADDR")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runMain(ctx) }()

	baseURL := "http://" + testcontainersUserGatewayListen
	if !pollHealthzReadyUserGW(t, baseURL) {
		cancel()
		<-done
		t.Fatalf("healthz did not become ready")
	}

	cancel()
	code := <-done
	if code != 0 {
		t.Errorf("runMain exit code %d (want 0)", code)
	}
}
