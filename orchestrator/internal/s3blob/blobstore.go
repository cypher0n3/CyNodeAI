package s3blob

import "context"

// BlobStore is the minimal object storage API used by the artifacts service (S3, MinIO, or tests).
type BlobStore interface {
	PutObject(ctx context.Context, key string, body []byte, contentType *string) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
}

var _ BlobStore = (*Client)(nil)
