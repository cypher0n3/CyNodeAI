// Package main provides the orchestrator control-plane API.
// See docs/tech_specs/orchestrator.md.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/nodetelemetry"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/pmasubprocess"
)

// testShutdownTimeout, when set by tests, overrides the server shutdown timeout.
var testShutdownTimeout *time.Duration

// testShutdownHook, when set by tests, is called instead of server.Shutdown so tests can force a shutdown error.
var testShutdownHook func(*http.Server, context.Context) error

// testOpenStore, when set by tests, is used instead of database.Open when store is nil so store==nil path can be covered without a real DB.
var testOpenStore func(context.Context, string) (database.Store, error)

// testDatabaseOpen, when set by tests, is used instead of database.Open when both store and testOpenStore are nil (allows covering open-success path without a real DB).
var testDatabaseOpen func(context.Context, string) (database.Store, error)

// testPMAStart, when set by tests, is used instead of pmasubprocess.Start so tests can cover the "inference path available" branch without running the real binary.
var testPMAStart func(*config.OrchestratorConfig, *slog.Logger) (*exec.Cmd, error)

func main() {
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

// runMain runs the control-plane and returns an exit code. Used by main and tests.
func runMain() int {
	return runMainWithContext(context.Background(), nil)
}

// resolveStore opens the DB when store is nil (using testOpenStore or database.Open). Returns (store, nil), (nil, nil) when migrateOnly after open, or (nil, err).
//
//nolint:gocognit,dupl // test hooks and real open share the same migrateOnly handling by design
func resolveStore(ctx context.Context, store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger, migrateOnly bool) (database.Store, error) {
	if store != nil {
		return store, nil
	}
	if testOpenStore != nil {
		var err error
		store, err = testOpenStore(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			return nil, err
		}
		if store != nil && migrateOnly {
			logger.Info("schema applied (migrate-only)")
			return nil, nil
		}
		return store, nil
	}
	if testDatabaseOpen != nil {
		var err error
		store, err = testDatabaseOpen(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			return nil, err
		}
		if store != nil && migrateOnly {
			logger.Info("schema applied (migrate-only)")
			return nil, nil
		}
		return store, nil
	}
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, err
	}
	if err := db.RunSchema(ctx, logger); err != nil {
		logger.Error("failed to run schema", "error", err)
		_ = db.Close()
		return nil, err
	}
	if migrateOnly {
		logger.Info("schema applied (migrate-only)")
		_ = db.Close()
		return nil, nil
	}
	return db, nil
}

// runMainWithContext runs the control-plane with an optional store (for tests). When store is nil, opens DB from config.
// Used by tests to exercise the full success path without a real database.
func runMainWithContext(ctx context.Context, store database.Store) int {
	fs := flag.NewFlagSet("control-plane", flag.ContinueOnError)
	var migrateOnly bool
	fs.BoolVar(&migrateOnly, "migrate-only", false, "run database migrations and exit")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadOrchestratorConfig()

	var err error
	store, err = resolveStore(ctx, store, cfg, logger, migrateOnly)
	if err != nil {
		return 1
	}
	if store == nil {
		return 0
	}
	if migrateOnly {
		logger.Info("schema applied (migrate-only)")
		return 0
	}

	if err := run(ctx, store, cfg, logger); err != nil {
		logger.Error("run failed", "error", err)
		return 1
	}
	return 0
}

// run bootstraps admin, starts the HTTP server and dispatcher until ctx is canceled. Used by main and tests.
func run(ctx context.Context, store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger) error {
	if err := bootstrapAdminUser(ctx, store, cfg.BootstrapAdminPassword, logger); err != nil {
		return err
	}

	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		cfg.JWTAccessDuration,
		cfg.JWTRefreshDuration,
		cfg.JWTNodeDuration,
	)

	nodeHandler := handlers.NewNodeHandler(store, jwtManager, cfg.NodeRegistrationPSK, cfg.OrchestratorPublicURL, cfg.WorkerAPIBearerToken, cfg.WorkerAPITargetURL, cfg.WorkerInternalAgentToken, logger)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, logger)
	workflowHandler := handlers.NewWorkflowHandler(store, logger)
	workflowAuth := middleware.RequireWorkflowRunnerAuth(cfg.WorkflowRunnerBearerToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.HandleFunc("GET /readyz", readyzHandler(store, cfg, logger))

	mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)
	mux.Handle("GET /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.GetConfig)))
	mux.Handle("POST /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ConfigAck)))
	mux.Handle("POST /v1/nodes/capability", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ReportCapability)))

	mux.Handle("POST /v1/workflow/start", workflowAuth(http.HandlerFunc(workflowHandler.Start)))
	mux.Handle("POST /v1/workflow/resume", workflowAuth(http.HandlerFunc(workflowHandler.Resume)))
	mux.Handle("POST /v1/workflow/checkpoint", workflowAuth(http.HandlerFunc(workflowHandler.SaveCheckpoint)))
	mux.Handle("POST /v1/workflow/release", workflowAuth(http.HandlerFunc(workflowHandler.Release)))

	handler := middleware.Recovery(logger)(middleware.Logging(logger)(mux))

	addr := getEnv("CONTROL_PLANE_LISTEN_ADDR", getEnv("LISTEN_ADDR", ":8082"))
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	go startDispatcher(ctx, store, logger)

	// REQ-ORCHES-0150: start PMA only when first inference path is available (worker ready and inference-capable, or API Egress key for PMA).
	var pmaCmdMu sync.Mutex
	var pmaCmd *exec.Cmd
	defer func() {
		pmaCmdMu.Lock()
		c := pmaCmd
		pmaCmdMu.Unlock()
		if c != nil && c.Process != nil {
			_ = c.Process.Signal(syscall.SIGTERM)
			_ = c.Wait()
			logger.Info("cynode-pma stopped")
		}
	}()
	if cfg.PMAEnabled {
		go startPMAWhenInferencePathReady(ctx, store, cfg, logger, &pmaCmdMu, &pmaCmd)
	}
	go runTelemetryPullLoop(ctx, store, logger)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting control-plane", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	case err := <-serverErr:
		return err
	}

	logger.Info("shutting down...")
	shutdownTimeout := 30 * time.Second
	if testShutdownTimeout != nil {
		shutdownTimeout = *testShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	var shutdownErr error
	if testShutdownHook != nil {
		shutdownErr = testShutdownHook(server, shutdownCtx)
	} else {
		shutdownErr = server.Shutdown(shutdownCtx)
	}
	if shutdownErr != nil {
		logger.Error("shutdown error", "error", shutdownErr)
		return shutdownErr
	}
	logger.Info("server stopped")
	return nil
}

// testPMAPollInterval, when set by tests, shortens the poll interval in startPMAWhenInferencePathReady.
var testPMAPollInterval time.Duration

// inferencePathAvailable returns true when at least one inference path exists: dispatchable node or external API credential (REQ-ORCHES-0150, orchestrator_bootstrap.md).
func inferencePathAvailable(ctx context.Context, store database.Store) (bool, error) {
	nodes, err := store.ListDispatchableNodes(ctx)
	if err != nil {
		return false, err
	}
	if len(nodes) > 0 {
		return true, nil
	}
	hasCred, err := store.HasAnyActiveApiCredential(ctx)
	if err != nil {
		return false, err
	}
	return hasCred, nil
}

// waitForInferencePath polls until at least one inference path is available (node or external key) or ctx is done.
// Returns true when inference path is available, false when ctx canceled. Used by startPMAWhenInferencePathReady and tests.
func waitForInferencePath(ctx context.Context, store database.Store, logger *slog.Logger) bool {
	pollInterval := 2 * time.Second
	if testPMAPollInterval > 0 {
		pollInterval = testPMAPollInterval
	}
	for {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		ok, err := inferencePathAvailable(ctx, store)
		if err != nil {
			if logger != nil {
				logger.Warn("PMA startup check: inference path check failed", "error", err)
			}
			time.Sleep(pollInterval)
			continue
		}
		if ok {
			return true
		}
		time.Sleep(pollInterval)
	}
}

// startPMAWhenInferencePathReady starts cynode-pma when the first inference path is available (REQ-ORCHES-0150).
func startPMAWhenInferencePathReady(ctx context.Context, store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger, pmaCmdMu *sync.Mutex, pmaCmdPtr **exec.Cmd) {
	if !waitForInferencePath(ctx, store, logger) {
		return
	}
	startFn := pmasubprocess.Start
	if testPMAStart != nil {
		startFn = testPMAStart
	}
	cmd, err := startFn(cfg, logger)
	if err != nil {
		logger.Error("failed to start cynode-pma", "error", err)
		return
	}
	if cmd != nil {
		pmaCmdMu.Lock()
		*pmaCmdPtr = cmd
		pmaCmdMu.Unlock()
	}
}

// telemetryPullInterval is the period for pulling node telemetry; tests may override.
var telemetryPullInterval = 60 * time.Second

// pullNodeTelemetry pulls node:info and node:stats for one node; logs errors only.
func pullNodeTelemetry(ctx context.Context, client *nodetelemetry.Client, baseURL, bearer, nodeID string, logger *slog.Logger) {
	pullCtx, cancel := context.WithTimeout(ctx, nodetelemetry.DefaultTimeout+time.Second)
	defer cancel()
	if _, err := client.PullNodeInfo(pullCtx, baseURL, bearer); err != nil {
		logger.Debug("telemetry pull node:info failed", "node_id", nodeID, "error", err)
	}
	if _, err := client.PullNodeStats(pullCtx, baseURL, bearer); err != nil {
		logger.Debug("telemetry pull node:stats failed", "node_id", nodeID, "error", err)
	}
}

// runTelemetryPullLoop periodically pulls node:info and node:stats from dispatchable nodes (REQ-ORCHES-0141--0143).
// Failures are logged and do not affect control-plane stability.
func runTelemetryPullLoop(ctx context.Context, store database.Store, logger *slog.Logger) {
	client := nodetelemetry.NewClient()
	ticker := time.NewTicker(telemetryPullInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
		nodes, err := store.ListDispatchableNodes(listCtx)
		listCancel()
		if err != nil {
			logger.Debug("telemetry pull: list nodes failed", "error", err)
			continue
		}
		for _, n := range nodes {
			if n.WorkerAPITargetURL == nil || *n.WorkerAPITargetURL == "" {
				continue
			}
			bearer := ""
			if n.WorkerAPIBearerToken != nil {
				bearer = *n.WorkerAPIBearerToken
			}
			pullNodeTelemetry(ctx, client, *n.WorkerAPITargetURL, bearer, n.ID.String(), logger)
		}
	}
}

// healthzHandler responds to GET /healthz with 200 and plain text body "ok" per spec.
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// pmaReady checks whether the PMA (cynode-pma) is reachable at the configured listen address.
// It performs a GET to /healthz with a short timeout. Returns true only on HTTP 200.
func pmaReady(ctx context.Context, listenAddr string) bool {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil || port == "" {
		return false
	}
	url := "http://127.0.0.1:" + port + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// readyzHandler returns a handler for GET /readyz. Returns 200 with body "ready" when at least one
// inference-capable path exists (dispatchable node) and, when PMA is enabled, PMA is ready; otherwise 503 with an actionable reason.
// See REQ-ORCHES-0119, REQ-ORCHES-0120, REQ-ORCHES-0129 and CYNAI.ORCHES.Rule.HealthEndpoints.
func readyzHandler(store database.Store, cfg *config.OrchestratorConfig, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		hasPath, err := inferencePathAvailable(ctx, store)
		if err != nil {
			if logger != nil {
				logger.Error("readyz check failed", "error", err)
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("readiness check failed (database error)"))
			return
		}
		// REQ-ORCHES-0120: require at least one inference path (dispatchable node or external API credential) before reporting ready.
		if !hasPath {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("no inference path available (no dispatchable nodes; register and configure a worker node or configure external provider keys)"))
			return
		}
		if cfg != nil && cfg.PMAEnabled {
			if !pmaReady(ctx, cfg.PMAListenAddr) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("PMA not ready (cynode-pma not reachable or not yet started)"))
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func bootstrapAdminUser(ctx context.Context, db database.Store, password string, logger *slog.Logger) error {
	_, err := db.GetUserByHandle(ctx, "admin")
	if err == nil {
		logger.Info("admin user already exists")
		return nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return err
	}

	user, err := db.CreateUser(ctx, "admin", nil)
	if err != nil {
		return err
	}

	passwordHash, err := auth.HashPassword(password, nil)
	if err != nil {
		return err
	}

	_, err = db.CreatePasswordCredential(ctx, user.ID, passwordHash, "argon2id")
	if err != nil {
		return err
	}

	logger.Info("admin user created", "handle", "admin")
	return nil
}
