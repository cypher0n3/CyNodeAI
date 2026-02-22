// Package inferenceproxy provides a minimal HTTP reverse proxy to Ollama.
// Per docs/tech_specs/worker_node.md: enforces request size (10 MiB) and per-request timeout (120s);
// MUST NOT expose credentials.
package inferenceproxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// MaxRequestBodyBytes is the maximum request body size (10 MiB per worker_node.md).
const MaxRequestBodyBytes = 10 * 1024 * 1024

// PerRequestTimeout is the per-request timeout (120s per worker_node.md).
const PerRequestTimeout = 120 * time.Second

// NewProxy returns an http.Handler that forwards requests to upstream with size and timeout limits.
func NewProxy(upstream *url.URL) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.Transport = &http.Transport{ResponseHeaderTimeout: PerRequestTimeout}
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
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
