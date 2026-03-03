// Package main provides the API egress server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain sets up logger and runs the server. Returns 0 on success, 1 on failure. Used by main and tests.
func runMain(ctx context.Context) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	if err := run(ctx, logger); err != nil {
		logger.Error("run failed", "error", err)
		return 1
	}
	return 0
}

// run sets up and runs the server until ctx is cancelled. Used by main and tests.
func run(ctx context.Context, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// REQ-APIEGR-0001, REQ-APIEGR-0110--0119: minimal callable endpoint with authz and audit.
	callHandler := newCallHandler(logger, getEnv("API_EGRESS_BEARER_TOKEN", ""), getEnv("API_EGRESS_ALLOWED", "openai,github"))
	mux.Handle("POST /v1/call", callHandler)

	srv := &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8084"),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting api-egress", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout())
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// shutdownTimeout returns server shutdown timeout from env or default. Used by run and tests.
func shutdownTimeout() time.Duration {
	const defaultSec = 10
	if s := os.Getenv("API_EGRESS_SHUTDOWN_SEC"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultSec * time.Second
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// callRequest is the minimal body for POST /v1/call (REQ-APIEGR-0110--0119).
type callRequest struct {
	Provider  string          `json:"provider"`
	Operation string          `json:"operation"`
	Params    json.RawMessage `json:"params,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
}

// callHandler implements POST /v1/call with authz and audit; returns 501 for unimplemented operations.
type callHandler struct {
	logger   *slog.Logger
	token    string
	allowed  map[string]bool
}

func newCallHandler(logger *slog.Logger, bearerToken, allowlist string) *callHandler {
	allowed := make(map[string]bool)
	for _, p := range strings.Split(allowlist, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			allowed[p] = true
		}
	}
	return &callHandler{logger: logger, token: bearerToken, allowed: allowed}
}

func (h *callHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "https://cynode.ai/specs/method-not-allowed", "title": "Method Not Allowed", "status": 405})
		return
	}
	if h.token != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.token {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "https://cynode.ai/specs/unauthorized", "title": "Unauthorized", "status": 401})
			return
		}
	}
	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "https://cynode.ai/specs/validation", "title": "Bad Request", "status": 400, "detail": "invalid JSON"})
		return
	}
	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	if !h.allowed[provider] {
		h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", req.Provider, "operation", req.Operation, "allowed", false)
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "https://cynode.ai/specs/forbidden", "title": "Forbidden", "status": 403, "detail": "provider not allowed"})
		return
	}
	h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", req.Provider, "operation", req.Operation, "allowed", true)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "https://cynode.ai/specs/not-implemented", "title": "Not Implemented", "status": 501, "detail": "operation not implemented"})
}
