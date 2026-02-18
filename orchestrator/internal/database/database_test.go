package database

import (
	"context"
	"errors"
	"testing"
)

// DB behavior is tested via integration tests. Set POSTGRES_TEST_DSN and run:
//   go test -v -run Integration ./internal/database

func TestOpen_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	_, err := Open(ctx, "postgres://invalid:invalid@127.0.0.1:1/nonexistent?connect_timeout=1")
	if err == nil {
		t.Fatal("Open with invalid DSN should fail")
	}
}

func TestErrNotFound(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("ErrNotFound should be errors.Is(self)")
	}
	err := wrapErr(errors.New("other"), "op")
	if errors.Is(err, ErrNotFound) {
		t.Error("wrapErr of non-ErrRecordNotFound should not be ErrNotFound")
	}
}
