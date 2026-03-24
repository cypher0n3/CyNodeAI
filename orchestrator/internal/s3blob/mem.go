package s3blob

import (
	"context"
	"errors"
	"sync"
)

// MemStore is an in-memory BlobStore for unit tests and BDD (no MinIO required).
type MemStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

// NewMemStore returns an empty memory-backed store.
func NewMemStore() *MemStore {
	return &MemStore{m: make(map[string][]byte)}
}

// PutObject stores body under key.
func (m *MemStore) PutObject(ctx context.Context, key string, body []byte, _ *string) error {
	_ = ctx
	if m == nil {
		return errors.New("s3blob: nil MemStore")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = make(map[string][]byte)
	}
	cp := make([]byte, len(body))
	copy(cp, body)
	m.m[key] = cp
	return nil
}

// GetObject returns bytes for key.
func (m *MemStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	_ = ctx
	if m == nil {
		return nil, errors.New("s3blob: nil MemStore")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.m[key]
	if !ok {
		return nil, errors.New("s3blob: not found")
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}

// DeleteObject removes key.
func (m *MemStore) DeleteObject(ctx context.Context, key string) error {
	_ = ctx
	if m == nil {
		return errors.New("s3blob: nil MemStore")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
	return nil
}

var _ BlobStore = (*MemStore)(nil)
