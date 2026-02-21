// Package bdd: TestMain starts Postgres via testcontainers when POSTGRES_TEST_DSN is unset,
// so BDD scenarios run against a real DB (same behavior as database package integration tests).
// Set SKIP_TESTCONTAINERS=1 to run without a DB; scenarios that need the DB will skip.
package bdd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresTestDSNEnv    = "POSTGRES_TEST_DSN"
	skipTestcontainersEnv = "SKIP_TESTCONTAINERS"
	bddTCSetupTimeout     = 90 * time.Second
	bddTCWaitTimeout      = 60 * time.Second
	bddTCHardTimeout      = bddTCSetupTimeout + 15*time.Second
	bddTCTerminateTimeout = 15 * time.Second
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

func waitForPostgres(ctx context.Context, dsn string, timeout time.Duration) error {
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

// runBDDTestcontainersSetup starts Postgres via testcontainers for the BDD suite.
// On success sets POSTGRES_TEST_DSN and returns (container, true). On failure returns (nil, false).
func runBDDTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, bddTCSetupTimeout)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
	)
	if err != nil {
		_, _ = os.Stderr.WriteString("[bdd/testcontainers] postgres.Run failed: " + err.Error() + "\n")
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		_, _ = os.Stderr.WriteString("[bdd/testcontainers] ConnectionString failed: " + err.Error() + "\n")
		return container, false
	}
	_ = os.Setenv(postgresTestDSNEnv, connStr)
	if err := waitForPostgres(setupCtx, connStr, bddTCWaitTimeout); err != nil {
		_, _ = os.Stderr.WriteString("[bdd/testcontainers] " + err.Error() + "\n")
		return container, false
	}
	return container, true
}

func TestMain(m *testing.M) {
	if os.Getenv(postgresTestDSNEnv) != "" {
		os.Exit(m.Run())
		return
	}
	if os.Getenv(skipTestcontainersEnv) != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHost()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[bdd/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), bddTCTerminateTimeout)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	hardTimeout := bddTCHardTimeout
	resultCh := make(chan struct {
		c  *postgres.PostgresContainer
		ok bool
	}, 1)
	go func() {
		c, ok := runBDDTestcontainersSetup(context.Background())
		resultCh <- struct {
			c  *postgres.PostgresContainer
			ok bool
		}{c, ok}
	}()
	var ok bool
	select {
	case res := <-resultCh:
		container = res.c
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[bdd/testcontainers] setup did not complete within " + hardTimeout.String() + "\n")
		container = nil
		ok = false
	}
	if !ok {
		os.Exit(1)
	}
	code = m.Run()
}
