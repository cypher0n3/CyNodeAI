package s3blob

import (
	"context"
	"testing"
)

func TestMemStore_PutGetDeleteRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	ct := "application/octet-stream"
	if err := m.PutObject(ctx, "a/b", []byte("data"), &ct); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetObject(ctx, "a/b")
	if err != nil || string(got) != "data" {
		t.Fatalf("GetObject: %v %q", err, got)
	}
	if err := m.DeleteObject(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetObject(ctx, "a/b"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMemStore_PutNilContentType(t *testing.T) {
	m := NewMemStore()
	if err := m.PutObject(context.Background(), "k", []byte("x"), nil); err != nil {
		t.Fatal(err)
	}
}

func TestMemStore_nilReceiver(t *testing.T) {
	var m *MemStore
	if err := m.PutObject(context.Background(), "k", nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := m.GetObject(context.Background(), "k"); err == nil {
		t.Fatal("expected error")
	}
	if err := m.DeleteObject(context.Background(), "k"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemStore_lazyMapInit(t *testing.T) {
	m := &MemStore{}
	if err := m.PutObject(context.Background(), "k", []byte("v"), nil); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetObject(context.Background(), "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("got %v %q", err, got)
	}
}

func TestMemStore_GetNotFound(t *testing.T) {
	m := NewMemStore()
	_, err := m.GetObject(context.Background(), "missing")
	if err == nil || err.Error() != "s3blob: not found" {
		t.Fatalf("got %v", err)
	}
}
