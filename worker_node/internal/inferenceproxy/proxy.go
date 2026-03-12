// Package inferenceproxy provides a minimal HTTP reverse proxy to Ollama.
// Per docs/tech_specs/worker_node.md: enforces request size (10 MiB) and per-request timeout (120s);
// MUST NOT expose credentials.
package inferenceproxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

// MaxRequestBodyBytes is the maximum request body size (10 MiB per worker_node.md).
const MaxRequestBodyBytes = 10 * 1024 * 1024

// PerRequestTimeout is the per-request timeout (120s per worker_node.md).
const PerRequestTimeout = 120 * time.Second

// DefaultUpstream is the fallback Ollama URL when OLLAMA_UPSTREAM_URL is unset.
const DefaultUpstream = "http://host.containers.internal:11434"

// RunUDS starts the inference proxy on a Unix domain socket at sockPath.
// REQ-WORKER-0260: UDS mode — TCP port 11434 is NOT bound.
// Returns 0 on clean shutdown, 1 on error.
func RunUDS(ctx context.Context, sockPath string) int {
	upstream := os.Getenv("OLLAMA_UPSTREAM_URL")
	if upstream == "" {
		upstream = DefaultUpstream
	}
	return RunUDSWithUpstream(ctx, sockPath, upstream)
}

// RunUDSWithUpstream starts the inference proxy on a Unix domain socket using
// the given upstream URL directly, without reading from the environment.
// Used by callers (e.g. worker-api) that manage the upstream URL themselves.
func RunUDSWithUpstream(ctx context.Context, sockPath, upstream string) int {
	u, err := url.Parse(upstream)
	if err != nil {
		slog.Error("invalid OLLAMA_UPSTREAM_URL", "error", err)
		return 1
	}
	_ = os.Remove(sockPath) // clean up stale socket
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		slog.Error("UDS listen failed", "path", sockPath, "error", err)
		return 1
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", NewProxy(u))
	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  PerRequestTimeout + 10*time.Second,
		WriteTimeout: PerRequestTimeout + 10*time.Second,
	}
	done := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = srv.Serve(listener)
		close(done)
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		<-done
		return 0
	case <-done:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("UDS server failed", "error", serveErr)
			return 1
		}
		return 0
	}
}

// NewProxy returns an http.Handler that forwards requests to upstream with size and timeout limits.
func NewProxy(upstream *url.URL) http.Handler {
	rp := &httputil.ReverseProxy{
		Transport: &http.Transport{ResponseHeaderTimeout: PerRequestTimeout},
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(upstream)
			req.Out.Host = upstream.Host
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodyBytes+1))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(body) > MaxRequestBodyBytes {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		ctx, cancel := context.WithTimeout(r.Context(), PerRequestTimeout)
		defer cancel()
		rp.ServeHTTP(w, r.WithContext(ctx))
	})
}
