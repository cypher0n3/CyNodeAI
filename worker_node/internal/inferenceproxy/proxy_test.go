package inferenceproxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewProxy_forwards_to_upstream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:11434/", http.NoBody)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("got body %q, want \"ok\"", rec.Body.String())
	}
}

func TestNewProxy_rejects_large_body(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("backend should not be called for oversized body")
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	// Body larger than limit (use 1 byte over to avoid allocating 10 MiB in test)
	body := make([]byte, MaxRequestBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "http://localhost:11434/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("got status %d, want 413", rec.Code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }
func (errReader) Close() error             { return nil }

func errReadCloser() io.ReadCloser { return errReader{} }

func TestNewProxy_read_error_returns_500(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("backend should not be called")
	}))
	defer backend.Close()

	u, _ := url.Parse(backend.URL)
	proxy := NewProxy(u)

	req := httptest.NewRequest(http.MethodPost, "http://localhost:11434/", http.NoBody)
	req.Body = errReadCloser()
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want 500", rec.Code)
	}
}
