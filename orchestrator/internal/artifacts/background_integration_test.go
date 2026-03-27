package artifacts

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

func Test_runHashBackfillLoop_immediateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Long interval so we never hit the tick branch with a nil service.
	runHashBackfillLoop(ctx, nil, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func Test_runStaleCleanupLoop_immediateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runStaleCleanupLoop(ctx, nil, time.Hour, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestIntegration_backgroundLoopsTick(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024)

	parent, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go runHashBackfillLoop(parent, svc, 5*time.Millisecond, logger)
	go runStaleCleanupLoop(parent, svc, 5*time.Millisecond, time.Hour, logger)
	time.Sleep(25 * time.Millisecond)
}

func TestIntegration_StartBackgroundJobs(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024)
	cfg := &config.OrchestratorConfig{
		ArtifactHashBackfillEnabled:     true,
		ArtifactHashBackfillInterval:    5 * time.Millisecond,
		ArtifactStaleCleanupEnabled:     true,
		ArtifactStaleCleanupMaxAgeHours: 1,
		ArtifactStaleCleanupInterval:    5 * time.Millisecond,
	}
	parent, cancel := context.WithCancel(ctx)
	StartBackgroundJobs(parent, svc, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(25 * time.Millisecond)
	cancel()
}

func TestStartBackgroundJobs_nilOrDisabled(t *testing.T) {
	StartBackgroundJobs(context.Background(), nil, &config.OrchestratorConfig{ArtifactHashBackfillEnabled: true}, nil)
	StartBackgroundJobs(context.Background(), &Service{}, nil, nil)
	cfg := &config.OrchestratorConfig{}
	StartBackgroundJobs(context.Background(), &Service{}, cfg, nil)
}

func TestIntegration_StartBackgroundJobs_hashOnlyThenStaleOnly(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024)
	parent, cancel := context.WithCancel(ctx)
	StartBackgroundJobs(parent, svc, &config.OrchestratorConfig{
		ArtifactHashBackfillEnabled:  true,
		ArtifactHashBackfillInterval: 10 * time.Millisecond,
		ArtifactStaleCleanupEnabled:  false,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(25 * time.Millisecond)
	cancel()
	parent2, cancel2 := context.WithCancel(ctx)
	StartBackgroundJobs(parent2, svc, &config.OrchestratorConfig{
		ArtifactHashBackfillEnabled:     false,
		ArtifactStaleCleanupEnabled:     true,
		ArtifactStaleCleanupMaxAgeHours: 1,
		ArtifactStaleCleanupInterval:    10 * time.Millisecond,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(25 * time.Millisecond)
	cancel2()
}

func TestIntegration_StartBackgroundJobs_defaultIntervals(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024)
	cfg := &config.OrchestratorConfig{
		ArtifactHashBackfillEnabled:     true,
		ArtifactHashBackfillInterval:    0,
		ArtifactStaleCleanupEnabled:     true,
		ArtifactStaleCleanupMaxAgeHours: 1,
		ArtifactStaleCleanupInterval:    0,
	}
	parent, cancel := context.WithCancel(ctx)
	StartBackgroundJobs(parent, svc, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(5 * time.Millisecond)
	cancel()
}
