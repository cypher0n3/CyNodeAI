package artifacts

import (
	"context"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

func TestNewServiceFromConfig_nilInputs(t *testing.T) {
	ctx := context.Background()
	svc, err := NewServiceFromConfig(ctx, nil, &config.OrchestratorConfig{ArtifactsS3Endpoint: "http://127.0.0.1:9000"})
	if err != nil || svc != nil {
		t.Fatalf("nil db: svc=%v err=%v", svc, err)
	}
	svc, err = NewServiceFromConfig(ctx, nil, nil)
	if err != nil || svc != nil {
		t.Fatalf("nil cfg: svc=%v err=%v", svc, err)
	}
}

func TestNewServiceWithBlob_nil(t *testing.T) {
	if NewServiceWithBlob(nil, s3blob.NewMemStore(), 1024) != nil {
		t.Fatal("expected nil when db nil")
	}
	if NewServiceWithBlob(nil, nil, 1024) != nil {
		t.Fatal("expected nil when blob nil")
	}
}

func TestIntegration_NewServiceFromConfig_unreachableEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	db := tcArtifactsDB(t, context.Background())
	_, err := NewServiceFromConfig(ctx, db, &config.OrchestratorConfig{
		ArtifactsS3Endpoint: "http://127.0.0.1:1",
		ArtifactsS3Bucket:   "test-bucket-wire",
	})
	if err == nil {
		t.Fatal("expected error from S3 client")
	}
}

func TestIntegration_NewServiceWithBlob_zeroMaxUsesDefault(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 0)
	if svc == nil {
		t.Fatal("nil service")
	}
	if svc.HashInlineMaxBytes != 1024*1024 {
		t.Fatalf("max bytes: %d", svc.HashInlineMaxBytes)
	}
}
