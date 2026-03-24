package artifacts

import (
	"context"
	"strings"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

// NewServiceFromConfig builds an artifacts Service when ARTIFACTS_S3_ENDPOINT is set; otherwise returns nil, nil.
func NewServiceFromConfig(ctx context.Context, db *database.DB, cfg *config.OrchestratorConfig) (*Service, error) {
	if cfg == nil || db == nil {
		return nil, nil
	}
	ep := strings.TrimSpace(cfg.ArtifactsS3Endpoint)
	if ep == "" {
		return nil, nil
	}
	bucket := cfg.ArtifactsS3Bucket
	if bucket == "" {
		bucket = "cynodeai-artifacts"
	}
	region := cfg.ArtifactsS3Region
	if region == "" {
		region = "us-east-1"
	}
	blob, err := s3blob.New(ctx, &s3blob.Config{
		Endpoint:  ep,
		Region:    region,
		AccessKey: cfg.ArtifactsS3AccessKey,
		SecretKey: cfg.ArtifactsS3SecretKey,
		Bucket:    bucket,
	})
	if err != nil {
		return nil, err
	}
	maxBytes := cfg.ArtifactHashInlineMaxBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}
	return &Service{
		DB:                 db,
		Blob:               blob,
		HashInlineMaxBytes: maxBytes,
	}, nil
}

// NewServiceWithBlob builds a Service with an explicit blob backend (tests, BDD).
func NewServiceWithBlob(db *database.DB, blob s3blob.BlobStore, hashInlineMaxBytes int64) *Service {
	if db == nil || blob == nil {
		return nil
	}
	if hashInlineMaxBytes <= 0 {
		hashInlineMaxBytes = 1024 * 1024
	}
	return &Service{
		DB:                 db,
		Blob:               blob,
		HashInlineMaxBytes: hashInlineMaxBytes,
	}
}
