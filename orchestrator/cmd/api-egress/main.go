// Package main provides the API egress server.
// REQ-APIEGR-0001, REQ-APIEGR-0110--0113, REQ-APIEGR-0119: access control and audit when API_EGRESS_DSN is set.
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

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
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

// run sets up and runs the server until ctx is canceled. Used by main and tests.
func run(ctx context.Context, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	bearer := getEnv("API_EGRESS_BEARER_TOKEN", "")
	allowlist := getEnv("API_EGRESS_ALLOWED", "openai,github")
	dsn := getEnv("API_EGRESS_DSN", "")
	if dsn != "" {
		db, err := database.Open(ctx, dsn)
		if err != nil {
			return err
		}
		if err := db.RunSchema(ctx, logger); err != nil {
			_ = db.Close()
			return err
		}
		mux.Handle("POST /v1/call", newCallHandlerWithStore(logger, bearer, allowlist, db))
	} else {
		mux.Handle("POST /v1/call", newCallHandler(logger, bearer, allowlist))
	}

	handler := middleware.Logging(logger)(mux)
	srv := &http.Server{
		Addr:              getEnv("LISTEN_ADDR", ":8084"),
		Handler:           handler,
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
	logger.Info("shutting down api-egress")
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
	logger  *slog.Logger
	token   string
	allowed map[string]bool
	store   database.Store
}

func newCallHandler(logger *slog.Logger, bearerToken, allowlist string) *callHandler {
	return &callHandler{logger: logger, token: bearerToken, allowed: parseAllowlist(allowlist), store: nil}
}

func newCallHandlerWithStore(logger *slog.Logger, bearerToken, allowlist string, store database.Store) *callHandler {
	return &callHandler{logger: logger, token: bearerToken, allowed: parseAllowlist(allowlist), store: store}
}

func parseAllowlist(allowlist string) map[string]bool {
	allowed := make(map[string]bool)
	for _, p := range strings.Split(allowlist, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			allowed[p] = true
		}
	}
	return allowed
}

func (h *callHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "")
		return
	}
	if h.token != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.token {
			h.writeProblem(w, http.StatusUnauthorized, "Unauthorized", "")
			return
		}
	}
	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "invalid JSON")
		return
	}
	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	operation := strings.TrimSpace(strings.ToLower(req.Operation))

	if h.store != nil {
		subjectID, decision, reason := h.evaluateWithStore(r.Context(), &req, provider, operation)
		var taskIDPtr *uuid.UUID
		if req.TaskID != "" {
			if tid, err := uuid.Parse(req.TaskID); err == nil {
				taskIDPtr = &tid
			}
		}
		h.auditLog(r.Context(), subjectID, decision, reason, provider, operation, taskIDPtr)
		if decision == decisionDeny {
			h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", provider, "operation", operation, "decision", decision, "reason", reason)
			h.writeProblem(w, http.StatusForbidden, "Forbidden", reason)
			return
		}
		h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", provider, "operation", operation, "decision", "allow")
		h.writeProblem(w, http.StatusNotImplemented, "Not Implemented", "operation not implemented")
		return
	}

	if !h.allowed[provider] {
		h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", req.Provider, "operation", req.Operation, "allowed", false)
		h.writeProblem(w, http.StatusForbidden, "Forbidden", "provider not allowed")
		return
	}
	h.logger.Info("api_egress_audit", "task_id", req.TaskID, "provider", req.Provider, "operation", req.Operation, "allowed", true)
	h.writeProblem(w, http.StatusNotImplemented, "Not Implemented", "operation not implemented")
}

func (h *callHandler) writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	body := map[string]interface{}{"title": title, "status": status}
	if detail != "" {
		body["detail"] = detail
	}
	_ = json.NewEncoder(w).Encode(body)
}

const decisionDeny = "deny"
const decisionAllow = "allow"

// evaluateWithStore resolves subject from task_id, checks policy and credential; returns subjectID (may be nil on deny), decision, reason.
func (h *callHandler) evaluateWithStore(ctx context.Context, req *callRequest, provider, operation string) (subjectID *uuid.UUID, decision, reason string) {
	subjectID, reason = h.resolveSubjectFromTask(ctx, req)
	if subjectID == nil {
		return nil, decisionDeny, reason
	}
	rules, err := h.store.ListAccessControlRulesForApiCall(ctx, "user", subjectID, database.ActionApiCall, database.ResourceTypeProviderOperation)
	if err != nil {
		return subjectID, decisionDeny, "failed to load policy"
	}
	if reason := h.evaluatePolicy(rules, provider, operation); reason != "" {
		return subjectID, decisionDeny, reason
	}
	hasCred, err := h.store.HasActiveApiCredentialForUserAndProvider(ctx, *subjectID, provider)
	if err != nil {
		return subjectID, decisionDeny, "failed to check credential"
	}
	if !hasCred {
		return subjectID, decisionDeny, "no active credential for provider"
	}
	return subjectID, decisionAllow, ""
}

func (h *callHandler) resolveSubjectFromTask(ctx context.Context, req *callRequest) (subjectID *uuid.UUID, reason string) {
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, "task_id required"
	}
	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return nil, "invalid task_id"
	}
	task, err := h.store.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, "task not found"
		}
		return nil, "failed to resolve task"
	}
	if task.CreatedBy == nil {
		return nil, "task has no user context"
	}
	return task.CreatedBy, ""
}

func (h *callHandler) evaluatePolicy(rules []*models.AccessControlRule, provider, operation string) string {
	resource := provider + "/" + operation
	var allowMatch, denyMatch bool
	for _, rule := range rules {
		if rule.ResourcePattern != resource {
			continue
		}
		switch rule.Effect {
		case decisionDeny:
			denyMatch = true
		case decisionAllow:
			allowMatch = true
		}
	}
	if denyMatch {
		return "policy denies provider/operation"
	}
	if !allowMatch {
		return "provider/operation not allowed by policy"
	}
	return ""
}

func (h *callHandler) auditLog(ctx context.Context, subjectID *uuid.UUID, decision, reason, provider, operation string, taskID *uuid.UUID) {
	resource := provider + "/" + operation
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	rec := &models.AccessControlAuditLog{
		SubjectType:  "user",
		SubjectID:    subjectID,
		Action:       database.ActionApiCall,
		ResourceType: database.ResourceTypeProviderOperation,
		Resource:     resource,
		Decision:     decision,
		Reason:       reasonPtr,
		TaskID:       taskID,
	}
	if err := h.store.CreateAccessControlAuditLog(ctx, rec); err != nil {
		h.logger.Warn("access_control_audit_log failed", "error", err)
	}
}
