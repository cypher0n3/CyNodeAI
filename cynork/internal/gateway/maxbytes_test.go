package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
)

func TestMaxBytes_ResponseBodyCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 11*1024*1024))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL)
	b, err := c.GetBytes("/any")
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(b)) > httplimits.DefaultMaxHTTPResponseBytes {
		t.Fatalf("len %d > max %d", len(b), httplimits.DefaultMaxHTTPResponseBytes)
	}
}
