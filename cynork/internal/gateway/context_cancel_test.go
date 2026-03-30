package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContextCancel_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Health(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Health: want context.Canceled, got %v", err)
	}
}
