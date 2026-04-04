// Package bdd provides Godog step definitions for the orchestrator suite.
// Feature files live under repo features/orchestrator/.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/mcpgateway"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

type ctxKey int

const stateKey ctxKey = 0

// testState holds shared state for BDD steps.
type testState struct {
	server                 *httptest.Server
	db                     *database.DB
	accessToken            string
	refreshToken           string
	taskID                 string
	nodeJWT                string
	nodeSlug               string
	advertisedWorkerAPIURL string // optional; when set, registration/capability include worker_api.base_url
	lastConfigBody         []byte
	lastConfigVersion      string
	lastStatusCode         int
	// Fake worker for node-aware dispatch scenarios
	workerServer       *httptest.Server
	workerRequestMu    sync.Mutex
	workerRequest      *http.Request
	workerRequestBody  []byte // captured so we can inspect after handler returns (Body may be closed)
	workerToken        string
	lastTaskResultBody []byte
	// Mock inference server for Chat scenario (POST /api/generate); closed in After
	inferenceServer *httptest.Server
	// Mock PMA server for Chat via PMA path (worker-reported endpoint); closed in After
	pmaMockServer    *httptest.Server
	pmaMockServerURL string
	// Workflow: last start response body and stored lease_id for release step
	workflowStartBody []byte
	storedLeaseID     string
	// API egress stub config (per-scenario) and last response body
	egressBearer     string
	egressAllowlist  string
	lastResponseBody []byte
	// GPU devices for registration (when set, registration includes gpu in capability)
	registrationGPU *nodepayloads.GPUInfo
	// Artifacts API (user-scoped CRUD / RBAC scenarios)
	artifactID string
	// Scope-partition BDD anchors (group / project UUID strings)
	bddGroupID   string
	bddProjectID string
	// MCP gateway test tokens (must match ToolCallAuth in test mux)
	pmMCPAgentToken      string
	sandboxMCPAgentToken string
}

func getState(ctx context.Context) *testState {
	s, _ := ctx.Value(stateKey).(*testState)
	return s
}

func bddGetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func bddPostArtifact(st *testState, query, body string) error {
	if st == nil || st.server == nil {
		return godog.ErrSkip
	}
	req, err := http.NewRequest(http.MethodPost, st.server.URL+"/v1/artifacts"+query, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+st.accessToken)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	st.lastStatusCode = resp.StatusCode
	st.lastResponseBody, _ = io.ReadAll(resp.Body)
	return nil
}

// bddTruncatePublicTables removes all rows from every public table so scenarios do not
// observe data left by earlier scenarios in the same suite (same Postgres volume).
func bddDeleteNodeBySlug(ctx context.Context, db *database.DB, slug string) error {
	// GORM placeholder for PostgreSQL
	return db.GORM().WithContext(ctx).Exec(`DELETE FROM nodes WHERE node_slug = ?`, slug).Error
}

func bddTruncatePublicTables(ctx context.Context, db *database.DB) error {
	sqlDB, err := db.GORM().DB()
	if err != nil {
		return err
	}
	rows, err := sqlDB.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var quoted []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		quoted = append(quoted, `"`+name+`"`)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(quoted) == 0 {
		return nil
	}
	q := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE"
	_, err = sqlDB.ExecContext(ctx, q)
	return err
}

// apiEgressStub returns a handler that mimics api-egress POST /v1/call: bearer auth, allowlist, 403/501.
func apiEgressStub(state *testState) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"title": "Method Not Allowed", "status": 405})
			return
		}
		if state.egressBearer != "" {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != state.egressBearer {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"title": "Unauthorized", "status": 401})
				return
			}
		}
		var req struct {
			Provider  string `json:"provider"`
			Operation string `json:"operation"`
			TaskID    string `json:"task_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"title": "Bad Request", "status": 400, "detail": "invalid JSON"})
			return
		}
		allowed := make(map[string]bool)
		for _, p := range strings.Split(state.egressAllowlist, ",") {
			p = strings.TrimSpace(strings.ToLower(p))
			if p != "" {
				allowed[p] = true
			}
		}
		provider := strings.TrimSpace(strings.ToLower(req.Provider))
		if !allowed[provider] {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"title": "Forbidden", "status": 403, "detail": "provider not allowed"})
			return
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"title": "Not Implemented", "status": 501, "detail": "operation not implemented"})
	})
}

// InitializeOrchestratorSuite sets up the godog suite with a test server and DB.
// POSTGRES_TEST_DSN is set by TestMain via testcontainers when unset; scenarios that need the DB skip only when SKIP_TESTCONTAINERS=1.
func InitializeOrchestratorSuite(sc *godog.ScenarioContext, state *testState) {
	sc.Before(func(ctx context.Context, s *godog.Scenario) (context.Context, error) {
		dsn := os.Getenv("POSTGRES_TEST_DSN")
		if dsn == "" {
			return ctx, nil
		}
		db, err := database.Open(ctx, dsn)
		if err != nil {
			return ctx, err
		}
		if err := db.RunSchema(ctx, slog.Default()); err != nil {
			_ = db.Close()
			return ctx, err
		}
		cfg := config.LoadOrchestratorConfig()
		if err := database.ApplyWorkerBearerEncryptionAtStartup(ctx, db, cfg.JWTSecret); err != nil {
			_ = db.Close()
			return ctx, err
		}
		// orchestrator_startup.feature expects /readyz to be 503 when no dispatchable nodes exist.
		// Earlier scenarios in the suite leave nodes in the shared Postgres; clear data only for this scenario.
		if s.Name == "Orchestrator remains not ready when no inference path is available" {
			if err := bddTruncatePublicTables(ctx, db); err != nil {
				_ = db.Close()
				return ctx, err
			}
		}
		// PMA routing scenarios derive service_id from the active refresh session; stale session_bindings
		// or nodes from earlier scenarios in the shared DB cause flaky 503 (model_unavailable).
		if strings.Contains(s.Name, "cynodeai.pm") && strings.Contains(s.Name, "worker-reported") {
			if err := bddTruncatePublicTables(ctx, db); err != nil {
				_ = db.Close()
				return ctx, err
			}
		}
		// orchestrator_startup.feature registers "ready-node-01" for readiness; task lifecycle
		// expects a single dispatcher target (test-node-01). Remove the leftover node so dispatch
		// does not hit the wrong worker mock.
		if strings.Contains(s.Uri, "orchestrator_task_lifecycle.feature") {
			if err := bddDeleteNodeBySlug(ctx, db, "ready-node-01"); err != nil {
				_ = db.Close()
				return ctx, err
			}
		}
		jwtManager := auth.NewJWTManager(
			cfg.JWTSecret,
			cfg.JWTAccessDuration,
			cfg.JWTRefreshDuration,
			cfg.JWTNodeDuration,
		)
		rateLimiter := auth.NewRateLimiter(cfg.RateLimitPerMinute, time.Minute)
		authHandler := handlers.NewAuthHandler(db, jwtManager, rateLimiter, nil, "", "", nil)
		userHandler := handlers.NewUserHandler(db, nil)
		// Enable mock inference for Chat scenarios so Chat returns immediately; other scenarios get queued tasks.
		var inferenceURL, inferenceModel string
		if s.Name == "Chat returns model response" || s.Name == "Chat completion returns 200 or acceptable error status" {
			inferenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/generate" || r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": "Hello from model", "done": true})
			}))
			state.inferenceServer = inferenceServer
			inferenceURL = inferenceServer.URL
			inferenceModel = "qwen3.5:0.8b"
		}
		taskHandler := handlers.NewTaskHandler(db, nil, inferenceURL, inferenceModel)
		// Do not pass cfg.WorkerAPITargetURL: host env (e.g. after setup-dev) would override node-reported
		// worker_api.base_url and break scenarios that assert registration persists the node's URL.
		nodeHandler := handlers.NewNodeHandler(db, jwtManager, cfg.NodeRegistrationPSK, cfg.OrchestratorPublicURL, cfg.WorkerAPIBearerToken, "", cfg.WorkerInternalAgentToken, nil, "", "", nil)
		authMiddleware := middleware.NewAuthMiddleware(jwtManager, nil)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
			list, err := db.ListDispatchableNodes(r.Context())
			if err != nil || len(list) == 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("no inference path available"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
		})
		mux.HandleFunc("POST /v1/auth/login", authHandler.Login)
		mux.HandleFunc("POST /v1/auth/refresh", authHandler.Refresh)
		mux.Handle("POST /v1/auth/logout", authMiddleware.RequireUserAuth(http.HandlerFunc(authHandler.Logout)))
		mux.Handle("GET /v1/users/me", authMiddleware.RequireUserAuth(http.HandlerFunc(userHandler.GetMe)))
		mux.Handle("POST /v1/users/{id}/revoke_sessions", authMiddleware.RequireAdminAuth(http.HandlerFunc(userHandler.RevokeSessions)))
		mux.Handle("POST /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CreateTask)))
		mux.Handle("GET /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.ListTasks)))
		mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
		mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))
		mux.Handle("POST /v1/tasks/{id}/cancel", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CancelTask)))
		mux.Handle("POST /v1/tasks/{id}/ready", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.PostTaskReady)))
		mux.Handle("GET /v1/tasks/{id}/logs", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskLogs)))
		openAIChatHandler := handlers.NewOpenAIChatHandler(db, slog.Default(), inferenceURL, inferenceModel, bddGetEnv("WORKER_API_BEARER_TOKEN", ""))
		mux.Handle("GET /v1/models", authMiddleware.RequireUserAuth(http.HandlerFunc(openAIChatHandler.ListModels)))
		mux.Handle("POST /v1/chat/completions", authMiddleware.RequireUserAuth(http.HandlerFunc(openAIChatHandler.ChatCompletions)))
		mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)
		mux.Handle("DELETE /v1/nodes/self", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.UnregisterSelf)))
		mux.Handle("GET /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.GetConfig)))
		mux.Handle("POST /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ConfigAck)))
		mux.Handle("POST /v1/nodes/capability", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ReportCapability)))
		workflowHandler := handlers.NewWorkflowHandler(db, nil)
		workflowAuth := middleware.RequireWorkflowRunnerAuth("")
		mux.Handle("POST /v1/workflow/start", workflowAuth(http.HandlerFunc(workflowHandler.Start)))
		mux.Handle("POST /v1/workflow/resume", workflowAuth(http.HandlerFunc(workflowHandler.Resume)))
		mux.Handle("POST /v1/workflow/checkpoint", workflowAuth(http.HandlerFunc(workflowHandler.SaveCheckpoint)))
		mux.Handle("POST /v1/workflow/release", workflowAuth(http.HandlerFunc(workflowHandler.Release)))
		blob := s3blob.NewMemStore()
		artSvc := artifacts.NewServiceWithBlob(db, blob, 1024*1024)
		mcpgateway.SetArtifactToolService(artSvc)
		state.pmMCPAgentToken = bddGetEnv("BDD_MCP_PM_TOKEN", "bdd-mcp-pm-token")
		state.sandboxMCPAgentToken = bddGetEnv("BDD_MCP_SANDBOX_TOKEN", "bdd-mcp-sandbox-token")
		mcpAuth := &mcpgateway.ToolCallAuth{PMToken: state.pmMCPAgentToken, SandboxToken: state.sandboxMCPAgentToken}
		mux.HandleFunc("POST /v1/mcp/tools/call", mcpgateway.ToolCallHandler(db, slog.Default(), mcpAuth))
		artifactsHandler := handlers.NewArtifactsHandler(artSvc, slog.Default())
		mux.Handle("POST /v1/artifacts", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Create)))
		mux.Handle("GET /v1/artifacts", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Find)))
		mux.Handle("GET /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Read)))
		mux.Handle("PUT /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Update)))
		mux.Handle("DELETE /v1/artifacts/{artifact_id}", authMiddleware.RequireUserAuth(http.HandlerFunc(artifactsHandler.Delete)))
		state.egressBearer = bddGetEnv("API_EGRESS_BEARER_TOKEN", "egress-bearer")
		state.egressAllowlist = bddGetEnv("API_EGRESS_ALLOWED", "openai,github")
		mux.Handle("POST /v1/call", apiEgressStub(state))

		state.server = httptest.NewServer(mux)
		state.db = db
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, s *godog.Scenario, err error) (context.Context, error) {
		if state.inferenceServer != nil {
			state.inferenceServer.Close()
			state.inferenceServer = nil
		}
		if state.pmaMockServer != nil {
			state.pmaMockServer.Close()
			state.pmaMockServer = nil
			state.pmaMockServerURL = ""
		}
		if state.workerServer != nil {
			state.workerServer.Close()
			state.workerServer = nil
		}
		state.workerRequestMu.Lock()
		state.workerRequest = nil
		state.workerRequestBody = nil
		state.workerRequestMu.Unlock()
		state.workerToken = ""
		if state.server != nil {
			state.server.Close()
		}
		if state.db != nil {
			_ = state.db.Close()
		}
		state.server = nil
		state.db = nil
		state.accessToken = ""
		state.refreshToken = ""
		state.taskID = ""
		state.nodeJWT = ""
		state.nodeSlug = ""
		state.lastConfigBody = nil
		state.lastConfigVersion = ""
		state.lastStatusCode = 0
		state.lastTaskResultBody = nil
		state.workflowStartBody = nil
		state.storedLeaseID = ""
		state.lastResponseBody = nil
		state.registrationGPU = nil
		state.artifactID = ""
		state.bddGroupID = ""
		state.bddProjectID = ""
		state.pmMCPAgentToken = ""
		state.sandboxMCPAgentToken = ""
		mcpgateway.SetArtifactToolService(nil)
		return ctx, nil
	})

	RegisterOrchestratorSteps(sc, state)
}

// RegisterOrchestratorSteps registers step definitions for orchestrator features.
func RegisterOrchestratorSteps(sc *godog.ScenarioContext, state *testState) {
	registerOrchestratorBootstrapTasks(sc, state)
	registerOrchestratorWorkflowEgressArtifacts(sc, state)
	registerOrchestratorWorkflowEgressArtifactsTaskCRUD(sc, state)
	registerOrchestratorTasksDispatchChat(sc, state)
}

func orchestratorSelectsNodeForDispatch(ctx context.Context) error {
	if err := postTaskReadyHTTP(ctx); err != nil {
		return err
	}
	st := getState(ctx)
	if st == nil || st.workerServer == nil || st.db == nil {
		return godog.ErrSkip
	}
	client := &http.Client{Timeout: 30 * time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return err
		}
		st.workerRequestMu.Lock()
		got := st.workerRequest != nil
		st.workerRequestMu.Unlock()
		if got {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("dispatcher did not call worker within 5s")
}

func taskCompletes(ctx context.Context) error {
	if err := postTaskReadyHTTP(ctx); err != nil {
		return err
	}
	st := getState(ctx)
	if st == nil || st.db == nil || st.taskID == "" {
		return godog.ErrSkip
	}
	client := &http.Client{Timeout: 30 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return err
		}
		taskID, err := uuid.Parse(st.taskID)
		if err != nil {
			return err
		}
		task, err := st.db.GetTaskByID(ctx, taskID)
		if err != nil {
			return err
		}
		if task.Status == models.TaskStatusCompleted || task.Status == models.TaskStatusFailed {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("task did not complete within 15s")
}

// postTaskReadyHTTP calls POST /v1/tasks/{id}/ready so draft tasks can run jobs or workflows (REQ-ORCHES-0179).
func postTaskReadyHTTP(ctx context.Context) error {
	st := getState(ctx)
	if st == nil || st.server == nil || st.taskID == "" {
		return godog.ErrSkip
	}
	req, err := http.NewRequest("POST", st.server.URL+"/v1/tasks/"+st.taskID+"/ready", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+st.accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post task ready returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func getTaskResult(ctx context.Context) error {
	st := getState(ctx)
	if st == nil || st.server == nil || st.taskID == "" {
		return godog.ErrSkip
	}
	req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID+"/result", nil)
	req.Header.Set("Authorization", "Bearer "+st.accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	st.lastStatusCode = resp.StatusCode
	st.lastTaskResultBody, err = io.ReadAll(resp.Body)
	return err
}

func sendChatMessage(ctx context.Context, message, model string) error {
	st := getState(ctx)
	if st == nil || st.server == nil {
		return godog.ErrSkip
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": message}},
	})
	req, _ := http.NewRequest("POST", st.server.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+st.accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	st.lastStatusCode = resp.StatusCode
	st.lastTaskResultBody, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chat/completions returned %d", resp.StatusCode)
	}
	return nil
}

// normalizeWorkerAPIBaseURL strips optional Gherkin-style angle brackets around URLs in feature steps.
func normalizeWorkerAPIBaseURL(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

func nodeRegisterStep(ctx context.Context, slug, advertisedWorkerAPIURL string) error {
	st := getState(ctx)
	if st == nil || st.server == nil {
		return godog.ErrSkip
	}
	cfg := config.LoadOrchestratorConfig()
	capability := map[string]interface{}{
		"version":     1,
		"reported_at": time.Now().UTC().Format(time.RFC3339),
		"node":        map[string]interface{}{"node_slug": slug},
		"platform":    map[string]interface{}{"os": "linux", "arch": "amd64"},
		"compute":     map[string]interface{}{"cpu_cores": 2, "ram_mb": 4096},
	}
	if u := normalizeWorkerAPIBaseURL(advertisedWorkerAPIURL); u != "" {
		capability["worker_api"] = map[string]interface{}{"base_url": u}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"psk":        cfg.NodeRegistrationPSK,
		"capability": capability,
	})
	resp, err := http.Post(st.server.URL+"/v1/nodes/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register returned %d", resp.StatusCode)
	}
	var out struct {
		Auth struct {
			NodeJWT string `json:"node_jwt"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	st.nodeJWT = out.Auth.NodeJWT
	st.nodeSlug = slug
	return nil
}

// nodeRegisterStepWithInference registers the node with capability that includes inference (supported, existing_service).
func nodeRegisterStepWithInference(ctx context.Context, existingService bool) error {
	return nodeRegisterStepWithInferenceAndGPU(ctx, existingService)
}

// nodeRegisterStepWithInferenceAndGPU registers the node with capability that includes inference and optional GPU.
func nodeRegisterStepWithInferenceAndGPU(ctx context.Context, existingService bool) error {
	st := getState(ctx)
	if st == nil || st.server == nil || st.nodeSlug == "" {
		return godog.ErrSkip
	}
	cfg := config.LoadOrchestratorConfig()
	capability := map[string]interface{}{
		"version":     1,
		"reported_at": time.Now().UTC().Format(time.RFC3339),
		"node":        map[string]interface{}{"node_slug": st.nodeSlug},
		"platform":    map[string]interface{}{"os": "linux", "arch": "amd64"},
		"compute":     map[string]interface{}{"cpu_cores": 2, "ram_mb": 4096},
		"inference": map[string]interface{}{
			"supported":        true,
			"existing_service": existingService,
			"running":          false,
		},
	}
	if u := normalizeWorkerAPIBaseURL(st.advertisedWorkerAPIURL); u != "" {
		capability["worker_api"] = map[string]interface{}{"base_url": u}
	}
	if st.registrationGPU != nil {
		gpuJSON, _ := json.Marshal(st.registrationGPU)
		var gpuMap map[string]interface{}
		_ = json.Unmarshal(gpuJSON, &gpuMap)
		capability["gpu"] = gpuMap
	}
	body, _ := json.Marshal(map[string]interface{}{
		"psk":        cfg.NodeRegistrationPSK,
		"capability": capability,
	})
	resp, err := http.Post(st.server.URL+"/v1/nodes/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register returned %d", resp.StatusCode)
	}
	var out struct {
		Auth struct {
			NodeJWT string `json:"node_jwt"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	st.nodeJWT = out.Auth.NodeJWT
	return nil
}
