// Package httplimits provides shared HTTP body size limits for APIs and HTTP clients.
package httplimits

import (
	"io"
	"net/http"
)

const (
	// DefaultMaxAPIRequestBodyBytes is the default maximum JSON/API request body size (10 MiB).
	DefaultMaxAPIRequestBodyBytes int64 = 10 * 1024 * 1024
	// DefaultMaxArtifactUploadBytes is the default maximum artifact blob upload size (100 MiB).
	DefaultMaxArtifactUploadBytes int64 = 100 * 1024 * 1024
	// DefaultMaxHTTPResponseBytes is the default maximum response body read size for outbound HTTP clients (100 MiB).
	DefaultMaxHTTPResponseBytes int64 = 100 * 1024 * 1024
)

// WrapRequestBody replaces r.Body with http.MaxBytesReader. Call before json.NewDecoder(r.Body) or io.ReadAll(r.Body).
func WrapRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}

// LimitBody returns middleware that limits the request body before invoking next.
func LimitBody(maxBytes int64, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WrapRequestBody(w, r, maxBytes)
		next(w, r)
	}
}

// LimitResponseReader wraps resp.Body with io.LimitReader. Caller must still Close resp.Body.
func LimitResponseReader(resp *http.Response, maxBytes int64) io.Reader {
	return io.LimitReader(resp.Body, maxBytes)
}
