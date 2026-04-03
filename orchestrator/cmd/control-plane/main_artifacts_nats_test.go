package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/nats-io/nats-server/v2/test"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestWireArtifactsService_NonDBStore covers wireArtifactsService when the store is not *database.DB (MCP artifact tools cleared).
func TestWireArtifactsService_NonDBStore(t *testing.T) {
	got := wireArtifactsService(context.Background(), testutil.NewMockDB(), config.LoadOrchestratorConfig(), slog.Default())
	if got != nil {
		t.Fatalf("expected nil when store is not *database.DB, got %v", got)
	}
}

func TestResolveStore_ReturnsInjectedStore(t *testing.T) {
	mock := testutil.NewMockDB()
	cfg := config.LoadOrchestratorConfig()
	got, err := resolveStore(context.Background(), mock, cfg, slog.Default(), false)
	if err != nil || got != mock {
		t.Fatalf("resolveStore: err=%v got=%v", err, got)
	}
}

func TestResolveStore_TestOpenStoreError(t *testing.T) {
	testOpenStore = func(context.Context, string) (database.Store, error) {
		return nil, errors.New("open failed")
	}
	defer func() { testOpenStore = nil }()
	_, err := resolveStore(context.Background(), nil, config.LoadOrchestratorConfig(), slog.Default(), false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConnectControlPlaneNATS_NoIssuer(t *testing.T) {
	cfg := &config.OrchestratorConfig{NATSClientURL: "nats://127.0.0.1:4222"}
	if nc := connectControlPlaneNATS(context.Background(), testutil.NewMockDB(), cfg, nil, slog.Default()); nc != nil {
		t.Fatal("expected nil conn when issuer nil")
	}
}

func TestConnectControlPlaneNATS_ConnectFails(t *testing.T) {
	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.OrchestratorConfig{NATSClientURL: "nats://127.0.0.1:1"}
	if nc := connectControlPlaneNATS(context.Background(), testutil.NewMockDB(), cfg, iss, slog.Default()); nc != nil {
		t.Fatal("expected nil conn on connect failure")
	}
}

func TestWireArtifactsService_RealDBNoArtifactsEndpoint(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	cfg := config.LoadOrchestratorConfig()
	cfg.ArtifactsS3Endpoint = ""
	got := wireArtifactsService(ctx, db, cfg, slog.Default())
	if got != nil {
		t.Fatalf("expected nil without S3 endpoint, got %v", got)
	}
}

func TestWireArtifactsService_RealDBS3BlobError(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	cfg := config.LoadOrchestratorConfig()
	cfg.ArtifactsS3Endpoint = "http://127.0.0.1:1"
	cfg.ArtifactsS3AccessKey = "x"
	cfg.ArtifactsS3SecretKey = "y"
	got := wireArtifactsService(ctx, db, cfg, slog.Default())
	if got != nil {
		t.Fatalf("expected nil when S3 init fails, got %v", got)
	}
}

func TestResolveStore_RealPostgresMigrateOnly(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	ctx := context.Background()
	cfg := config.LoadOrchestratorConfig()
	cfg.DatabaseURL = dsn
	store, err := resolveStore(ctx, nil, cfg, slog.Default(), true)
	if err != nil {
		t.Fatalf("resolveStore: %v", err)
	}
	if store != nil {
		t.Fatal("expected nil store after migrate-only")
	}
}

func TestConnectControlPlaneNATS_Success(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	defer s.Shutdown()

	iss, err := natsjwt.NewDevIssuer()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.OrchestratorConfig{NATSClientURL: s.ClientURL()}
	nc := connectControlPlaneNATS(context.Background(), testutil.NewMockDB(), cfg, iss, slog.Default())
	if nc == nil {
		t.Fatal("expected NATS connection")
	}
	t.Cleanup(func() { nc.Close() })
}
