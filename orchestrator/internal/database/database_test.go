package database

import (
	"errors"
	"testing"
)

// DB behavior is tested via integration tests. Set POSTGRES_TEST_DSN and run:
//   go test -v -run Integration ./internal/database

func TestErrNotFound(t *testing.T) {
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("ErrNotFound should be errors.Is(self)")
	}
	err := wrapErr(errors.New("other"), "op")
	if errors.Is(err, ErrNotFound) {
		t.Error("wrapErr of non-ErrRecordNotFound should not be ErrNotFound")
	}
}
