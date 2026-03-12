package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// runWrapTest runs the given handler and asserts the response code; used by Logging and Recovery tests.
func runWrapTest(t *testing.T, handler http.Handler, wantCode int) {
	t.Helper()
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != wantCode {
		t.Errorf("status code = %d, want %d", w.Code, wantCode)
	}
}

// okHandler is a shared handler that returns 200 for middleware tests.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

// runMiddlewareTest builds a logger, wraps okHandler with the given middleware, and asserts the response code.
func runMiddlewareTest(t *testing.T, wrap func(*slog.Logger) func(http.Handler) http.Handler, wantCode int) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := wrap(logger)(okHandler)
	runWrapTest(t, handler, wantCode)
}

func TestLogging(t *testing.T) {
	runMiddlewareTest(t, Logging, http.StatusOK)
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rw := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:         http.StatusOK,
	}

	rw.WriteHeader(http.StatusNotFound)

	if rw.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rw.status, http.StatusNotFound)
	}
}
