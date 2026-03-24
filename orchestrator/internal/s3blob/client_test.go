package s3blob

import (
	"context"
	"testing"
)

func TestNew_MissingEndpoint(t *testing.T) {
	_, err := New(context.Background(), &Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_NilConfig(t *testing.T) {
	_, err := New(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
