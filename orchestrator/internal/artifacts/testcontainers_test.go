// Package artifacts: Postgres via testcontainers for integration tests (mirrors database package TestMain).
package artifacts

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

const integrationEnv = "POSTGRES_TEST_DSN"

const tcSetupTimeoutArtifacts = 90 * time.Second

func runArtifactsTestcontainersSetup(ctx context.Context) (*postgres.PostgresContainer, bool) {
	setupCtx, cancel := context.WithTimeout(ctx, tcSetupTimeoutArtifacts)
	defer cancel()

	container, err := postgres.Run(setupCtx, "pgvector/pgvector:pg16",
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		postgres.WithDatabase("cynodeai"),
		postgres.WithUsername("cynodeai"),
		postgres.WithPassword("cynodeai-test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		_, _ = os.Stderr.WriteString("[artifacts/testcontainers] postgres.Run failed: " + err.Error() + "\n")
		return nil, false
	}
	connStr, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		_, _ = os.Stderr.WriteString("[artifacts/testcontainers] ConnectionString failed: " + err.Error() + "\n")
		return container, false
	}
		connStr = urlForceIPv4Localhost(connStr, "5432")
	select {
	case <-setupCtx.Done():
		return container, false
	case <-time.After(3 * time.Second):
	}
	_ = os.Setenv(integrationEnv, connStr)
	if err := waitForPostgresArtifacts(setupCtx, connStr, 60*time.Second); err != nil {
		_, _ = os.Stderr.WriteString("[artifacts/testcontainers] postgres not ready: " + err.Error() + "\n")
		return container, false
	}
	return container, true
}

func waitForPostgresArtifacts(ctx context.Context, dsn string, timeout time.Duration) error {
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

type tcResultArtifacts struct {
	container *postgres.PostgresContainer
	ok        bool
}

func TestMain(m *testing.M) {
	if dsn := os.Getenv(integrationEnv); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err := database.Open(ctx, dsn)
		cancel()
		if err == nil {
			_ = db.Close()
			os.Exit(m.Run())
			return
		}
		_ = os.Unsetenv(integrationEnv)
	}
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		os.Exit(m.Run())
		return
	}
	setupRootlessPodmanHostForTests()
	var code int
	var container *postgres.PostgresContainer
	defer func() {
		if r := recover(); r != nil {
			_, _ = os.Stderr.WriteString("[artifacts/testcontainers] panic: " + fmt.Sprint(r) + "\n")
			code = m.Run()
		}
		if container != nil {
			termCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_ = container.Terminate(termCtx)
			cancel()
		}
		os.Exit(code)
	}()

	hardTimeout := tcSetupTimeoutArtifacts + 15*time.Second
	ch := make(chan tcResultArtifacts, 1)
	go func() {
		c, ok := runArtifactsTestcontainersSetup(context.Background())
		ch <- tcResultArtifacts{container: c, ok: ok}
	}()
	var ok bool
	select {
	case res := <-ch:
		container = res.container
		ok = res.ok
	case <-time.After(hardTimeout):
		_, _ = os.Stderr.WriteString("[artifacts/testcontainers] setup timeout\n")
		container = nil
		ok = false
	}
	if !ok {
		code = m.Run()
		return
	}
	code = m.Run()
}

func tcArtifactsDB(t *testing.T, ctx context.Context) *database.DB {
	t.Helper()
	if os.Getenv(integrationEnv) == "" {
		t.Skip("postgres not available (set POSTGRES_TEST_DSN or run testcontainers)")
	}
	db, err := database.Open(ctx, os.Getenv(integrationEnv))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	return db
}
