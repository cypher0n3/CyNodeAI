package artifacts

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

func TestSystemDeleteArtifact_nilOrUnconfigured(t *testing.T) {
	ctx := context.Background()
	var s *Service
	if err := s.SystemDeleteArtifact(ctx, uuid.New()); err == nil {
		t.Fatal("nil service: want error")
	}
	s = &Service{}
	if err := s.SystemDeleteArtifact(ctx, uuid.New()); err == nil {
		t.Fatal("missing db/blob: want error")
	}
}

func TestBackfillMissingHashesOnce_nilOrUnconfigured(t *testing.T) {
	ctx := context.Background()
	var s *Service
	if _, err := s.BackfillMissingHashesOnce(ctx, 10); err == nil {
		t.Fatal("nil service: want error")
	}
	s = &Service{}
	if _, err := s.BackfillMissingHashesOnce(ctx, 10); err == nil {
		t.Fatal("missing db/blob: want error")
	}
}

func TestPruneStaleByMaxAgeOnce_nilService(t *testing.T) {
	ctx := context.Background()
	var s *Service
	if _, err := s.PruneStaleByMaxAgeOnce(ctx, time.Hour, 10); err == nil {
		t.Fatal("nil service: want error")
	}
}

func TestPruneStaleByMaxAgeOnce_zeroMaxAge(t *testing.T) {
	s := &Service{DB: nil}
	if n, err := s.PruneStaleByMaxAgeOnce(context.Background(), 0, 10); err != nil || n != 0 {
		t.Fatalf("zero max age: n=%d err=%v", n, err)
	}
}

func TestBackfillMissingHashesOnce_zeroBatchUsesDefault(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024)
	if _, err := svc.BackfillMissingHashesOnce(ctx, 0); err != nil {
		t.Fatal(err)
	}
}
