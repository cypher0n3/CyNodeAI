package s3blob

import (
	"context"
	"testing"
)

func TestNew_EmptyEndpoint(t *testing.T) {
	_, err := New(context.Background(), &Config{
		Endpoint: "",
		Bucket:   "b",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_EmptyBucket(t *testing.T) {
	_, err := New(context.Background(), &Config{
		Endpoint: "http://localhost:9000",
		Bucket:   "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
