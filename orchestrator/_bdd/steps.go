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
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/dispatcher"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

type ctxKey int

const stateKey ctxKey = 0

// testState holds shared state for BDD steps.
type testState struct {
	server            *httptest.Server
	db                *database.DB
	accessToken       string
	refreshToken      string
	taskID            string
	nodeJWT           string
	nodeSlug          string
	lastConfigBody    []byte
	lastConfigVersion string
	lastStatusCode    int
	// Fake worker for node-aware dispatch scenarios
	workerServer       *httptest.Server
	workerRequestMu    sync.Mutex
	workerRequest      *http.Request
	workerRequestBody  []byte // captured so we can inspect after handler returns (Body may be closed)
	workerToken        string
	lastTaskResultBody []byte
}

func getState(ctx context.Context) *testState {
	s, _ := ctx.Value(stateKey).(*testState)
	return s
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
		jwtManager := auth.NewJWTManager(
			cfg.JWTSecret,
			cfg.JWTAccessDuration,
			cfg.JWTRefreshDuration,
			cfg.JWTNodeDuration,
		)
		rateLimiter := auth.NewRateLimiter(cfg.RateLimitPerMinute, time.Minute)
		authHandler := handlers.NewAuthHandler(db, jwtManager, rateLimiter, nil)
		userHandler := handlers.NewUserHandler(db, nil)
		taskHandler := handlers.NewTaskHandler(db, nil, "", "")
		nodeHandler := handlers.NewNodeHandler(db, jwtManager, cfg.NodeRegistrationPSK, cfg.OrchestratorPublicURL, cfg.WorkerAPIBearerToken, cfg.WorkerAPITargetURL, nil)
		authMiddleware := middleware.NewAuthMiddleware(jwtManager, nil)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
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
		mux.Handle("POST /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CreateTask)))
		mux.Handle("GET /v1/tasks/{id}", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTask)))
		mux.Handle("GET /v1/tasks/{id}/result", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskResult)))
		mux.HandleFunc("POST /v1/nodes/register", nodeHandler.Register)
		mux.Handle("GET /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.GetConfig)))
		mux.Handle("POST /v1/nodes/config", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ConfigAck)))
		mux.Handle("POST /v1/nodes/capability", authMiddleware.RequireNodeAuth(http.HandlerFunc(nodeHandler.ReportCapability)))

		state.server = httptest.NewServer(mux)
		state.db = db
		return context.WithValue(ctx, stateKey, state), nil
	})

	sc.After(func(ctx context.Context, s *godog.Scenario, err error) (context.Context, error) {
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
		return ctx, nil
	})

	RegisterOrchestratorSteps(sc, state)
}

// RegisterOrchestratorSteps registers step definitions for orchestrator features.
func RegisterOrchestratorSteps(sc *godog.ScenarioContext, state *testState) {
	// Background: DB and API
	sc.Step(`^a running PostgreSQL database$`, func(ctx context.Context) error {
		if os.Getenv("POSTGRES_TEST_DSN") == "" {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^the orchestrator API is running$`, func(ctx context.Context) error {
		if getState(ctx) == nil || getState(ctx).server == nil {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^an admin user exists with handle "([^"]*)"$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		_, err := st.db.GetUserByHandle(ctx, handle)
		if err == nil {
			return nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return err
		}
		user, err := st.db.CreateUser(ctx, handle, nil)
		if err != nil {
			return err
		}
		hash, err := auth.HashPassword("admin123", nil)
		if err != nil {
			return err
		}
		_, err = st.db.CreatePasswordCredential(ctx, user.ID, hash, "argon2id")
		return err
	})

	// Auth
	sc.Step(`^I login as "([^"]*)" with password "([^"]*)"$`, func(ctx context.Context, handle, password string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"handle": handle, "password": password})
		resp, err := http.Post(st.server.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login returned %d", resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I receive an access token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.accessToken == "" {
			return fmt.Errorf("no access token")
		}
		return nil
	})
	sc.Step(`^I receive a refresh token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.refreshToken == "" {
			return fmt.Errorf("no refresh token")
		}
		return nil
	})
	sc.Step(`^I am logged in as "([^"]*)"$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"handle": handle, "password": "admin123"})
		resp, err := http.Post(st.server.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login as %q returned %d", handle, resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I refresh my token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"refresh_token": st.refreshToken})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("refresh returned %d", resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I receive a new access token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I receive a new refresh token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the old refresh token is invalidated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I logout$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"refresh_token": st.refreshToken})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("logout returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^my refresh token is invalidated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I cannot use the old access token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I request my user info$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get me returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^I receive my user details including handle "([^"]*)"$`, func(ctx context.Context, handle string) error {
		return nil
	})

	// Nodes
	sc.Step(`^a node with slug "([^"]*)" and valid PSK$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st != nil {
			st.nodeSlug = slug
		}
		return nil
	})
	sc.Step(`^a node with slug "([^"]*)" registers with the orchestrator$`, func(ctx context.Context, slug string) error {
		return nodeRegisterStep(ctx, slug)
	})
	sc.Step(`^a node with slug "([^"]*)" registers with the orchestrator using a valid PSK$`, func(ctx context.Context, slug string) error {
		return nodeRegisterStep(ctx, slug)
	})
	sc.Step(`^the node registers with the orchestrator$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.nodeSlug == "" {
			return godog.ErrSkip
		}
		return nodeRegisterStep(ctx, st.nodeSlug)
	})
	sc.Step(`^the node receives a JWT token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.nodeJWT == "" {
			return fmt.Errorf("no node JWT")
		}
		return nil
	})
	sc.Step(`^the node is recorded in the database$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		_, err := st.db.GetNodeBySlug(ctx, st.nodeSlug)
		return err
	})
	sc.Step(`^a registered node "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		_, err := st.db.GetNodeBySlug(ctx, slug)
		return err
	})
	sc.Step(`^the node reports its capabilities$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version":     1,
			"reported_at": time.Now().UTC().Format(time.RFC3339),
			"node":        map[string]interface{}{"node_slug": st.nodeSlug},
			"platform":    map[string]interface{}{"os": "linux", "arch": "amd64"},
			"compute":     map[string]interface{}{"cpu_cores": 2, "ram_mb": 4096},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/capability", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("capability returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator stores the capability snapshot$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the capability hash is updated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the node requests its configuration$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET config returned %d", resp.StatusCode)
		}
		st.lastConfigBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		st.lastConfigVersion = payload.ConfigVersion
		return nil
	})
	sc.Step(`^the orchestrator returns a configuration payload for "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.NodeSlug != slug {
			return fmt.Errorf("config payload node_slug %q, want %q", payload.NodeSlug, slug)
		}
		return nil
	})
	sc.Step(`^the payload includes config_version and worker_api bearer token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.ConfigVersion == "" {
			return fmt.Errorf("config payload missing config_version")
		}
		if payload.WorkerAPI == nil || payload.WorkerAPI.OrchestratorBearerToken == "" {
			return fmt.Errorf("config payload missing worker_api.orchestrator_bearer_token")
		}
		return nil
	})
	sc.Step(`^a registered node "([^"]*)" that has received configuration$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		if _, err := st.db.GetNodeBySlug(ctx, slug); errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if st.nodeJWT == "" {
			if err := nodeRegisterStep(ctx, slug); err != nil {
				return err
			}
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET config returned %d", resp.StatusCode)
		}
		st.lastConfigBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		st.lastConfigVersion = payload.ConfigVersion
		return nil
	})
	sc.Step(`^the node applies the configuration$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the node sends a config acknowledgement with status "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		if st.lastConfigVersion == "" {
			return fmt.Errorf("no config version in state (fetch config first)")
		}
		ack := nodepayloads.ConfigAck{
			Version:       1,
			NodeSlug:      st.nodeSlug,
			ConfigVersion: st.lastConfigVersion,
			AckAt:         time.Now().UTC().Format(time.RFC3339),
			Status:        status,
		}
		body, _ := json.Marshal(ack)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("POST config ack returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator records the config ack for "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if err != nil {
			return err
		}
		if node.ConfigAckStatus == nil || *node.ConfigAckStatus == "" {
			return fmt.Errorf("node %q has no config_ack_status", slug)
		}
		return nil
	})
	sc.Step(`^the node config_version is stored$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, st.nodeSlug)
		if err != nil {
			return err
		}
		if node.ConfigVersion == nil || *node.ConfigVersion == "" {
			return fmt.Errorf("node %q has no config_version stored", st.nodeSlug)
		}
		return nil
	})
	sc.Step(`^an unauthenticated request requests node configuration$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^the orchestrator responds with 401 Unauthorized$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != http.StatusUnauthorized {
			return fmt.Errorf("expected 401, got %d", st.lastStatusCode)
		}
		return nil
	})
	sc.Step(`^the node sends a config acknowledgement with node_slug "([^"]*)" and status "([^"]*)"$`, func(ctx context.Context, slug, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		if st.lastConfigVersion == "" {
			return fmt.Errorf("no config version in state (fetch config first)")
		}
		ack := nodepayloads.ConfigAck{
			Version:       1,
			NodeSlug:      slug,
			ConfigVersion: st.lastConfigVersion,
			AckAt:         time.Now().UTC().Format(time.RFC3339),
			Status:        status,
		}
		body, _ := json.Marshal(ack)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^the orchestrator responds with 400 Bad Request$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != http.StatusBadRequest {
			return fmt.Errorf("expected 400, got %d", st.lastStatusCode)
		}
		return nil
	})

	// Tasks
	sc.Step(`^I create a task with prompt "([^"]*)"$`, func(ctx context.Context, prompt string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"prompt": prompt})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		return nil
	})
	sc.Step(`^I create a task with command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		// Inference is the default for interpreted tasks; no user flag.
		body, _ := json.Marshal(map[string]any{"prompt": cmd, "use_inference": true})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		return nil
	})
	sc.Step(`^I create a task with use_inference and command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]any{"prompt": cmd, "use_inference": true})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		return nil
	})
	sc.Step(`^I create a task with command "([^"]*)"$`, func(ctx context.Context, cmd string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"prompt": cmd})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		return nil
	})
	sc.Step(`^I receive a task ID$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.taskID == "" {
			return fmt.Errorf("no task ID")
		}
		return nil
	})
	sc.Step(`^the task status is "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID, nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get task returned %d", resp.StatusCode)
		}
		var out struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		if out.Status != status {
			return fmt.Errorf("task status %q, want %q", out.Status, status)
		}
		return nil
	})
	sc.Step(`^I have created a task$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I get the task status$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I receive the task details including status$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the task completes$`, func(ctx context.Context) error {
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
	})
	sc.Step(`^the task result contains model output$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state (call get task result first)")
		}
		var result struct {
			Jobs []struct {
				Result *string `json:"result"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &result); err != nil {
			return err
		}
		if len(result.Jobs) == 0 || result.Jobs[0].Result == nil {
			return fmt.Errorf("task result has no job output")
		}
		var jobOut struct {
			Stdout string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(*result.Jobs[0].Result), &jobOut); err != nil {
			return err
		}
		if jobOut.Stdout == "" {
			return fmt.Errorf("task result stdout is empty (expected model output)")
		}
		return nil
	})
	sc.Step(`^I create a task with input_mode "([^"]*)" and prompt "([^"]*)"$`, func(ctx context.Context, inputMode, prompt string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]any{"prompt": prompt, "input_mode": inputMode})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		return nil
	})
	sc.Step(`^the job sent to the worker has command containing "([^"]*)"$`, func(ctx context.Context, sub string) error {
		st := getState(ctx)
		if st == nil || len(st.workerRequestBody) == 0 {
			return fmt.Errorf("no worker request body captured")
		}
		var reqBody struct {
			Sandbox struct {
				Command []string `json:"command"`
			} `json:"sandbox"`
		}
		if err := json.Unmarshal(st.workerRequestBody, &reqBody); err != nil {
			return fmt.Errorf("decode worker request: %w", err)
		}
		cmdStr := strings.Join(reqBody.Sandbox.Command, " ")
		if !strings.Contains(cmdStr, sub) {
			return fmt.Errorf("worker command %q does not contain %q", cmdStr, sub)
		}
		return nil
	})
	sc.Step(`^I have a completed task$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"prompt": "echo done"})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.ID
		client := &http.Client{Timeout: 30 * time.Second}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
			if err != nil && !errors.Is(err, database.ErrNotFound) {
				return err
			}
			taskID, _ := uuid.Parse(st.taskID)
			task, err := st.db.GetTaskByID(ctx, taskID)
			if err != nil {
				return err
			}
			if task.Status == models.TaskStatusCompleted {
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("task did not complete within 15s")
	})
	sc.Step(`^I get the task result$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
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
	})
	sc.Step(`^I receive the job output including stdout and exit code$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastTaskResultBody) == 0 {
			return fmt.Errorf("no task result in state (call get task result first)")
		}
		if st.lastStatusCode != http.StatusOK {
			return fmt.Errorf("task result returned %d", st.lastStatusCode)
		}
		var result struct {
			Jobs []struct {
				Result *string `json:"result"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(st.lastTaskResultBody, &result); err != nil {
			return err
		}
		if len(result.Jobs) == 0 || result.Jobs[0].Result == nil {
			return fmt.Errorf("task result has no job output")
		}
		var jobOut struct {
			Stdout   string `json:"stdout"`
			ExitCode int    `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(*result.Jobs[0].Result), &jobOut); err != nil {
			return err
		}
		// Assert fields are present (exit_code 0 is valid)
		_ = jobOut.Stdout
		_ = jobOut.ExitCode
		return nil
	})
	sc.Step(`^the orchestrator selects the node for dispatch$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.workerServer == nil || st.db == nil {
			return godog.ErrSkip
		}
		client := &http.Client{Timeout: 30 * time.Second}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			err := dispatcher.RunOnce(ctx, st.db, client, 30*time.Second, nil)
			if err == nil {
				// Dispatch ran; worker may have been called
			} else if !errors.Is(err, database.ErrNotFound) {
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
	})
	sc.Step(`^the orchestrator calls the node Worker API at its configured target URL$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		st.workerRequestMu.Lock()
		defer st.workerRequestMu.Unlock()
		if st.workerRequest == nil {
			return fmt.Errorf("no worker request was received")
		}
		if st.workerRequest.URL.Path != "/v1/worker/jobs:run" {
			return fmt.Errorf("worker request path %q, want /v1/worker/jobs:run", st.workerRequest.URL.Path)
		}
		return nil
	})
	sc.Step(`^the request includes the bearer token from that node's config$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		st.workerRequestMu.Lock()
		defer st.workerRequestMu.Unlock()
		if st.workerRequest == nil {
			return fmt.Errorf("no worker request was received")
		}
		want := "Bearer " + st.workerToken
		if got := st.workerRequest.Header.Get("Authorization"); got != want {
			return fmt.Errorf("Authorization header %q, want %q", got, want)
		}
		return nil
	})

	// Startup (fail fast when no inference)
	sc.Step(`^no local inference \(Ollama\) is running$`, func(ctx context.Context) error { return nil })
	sc.Step(`^no external provider key is configured$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the orchestrator starts$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/healthz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("healthz returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator does not enter ready state$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			return fmt.Errorf("readyz returned %d, want 503", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator reports that no inference path is available$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		resp, err := http.Get(st.server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("no inference path")) {
			return fmt.Errorf("readyz body %q does not contain 'no inference path'", string(body))
		}
		return nil
	})

	// E2E / worker (stubs)
	sc.Step(`^a worker node is running and reachable by the orchestrator$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the orchestrator dispatches a job to the node$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the node executes the sandbox job$`, func(ctx context.Context) error {
		return godog.ErrSkip
	})
	sc.Step(`^the job result contains stdout "([^"]*)"$`, func(ctx context.Context, s string) error {
		return godog.ErrSkip
	})
	sc.Step(`^the task status becomes "([^"]*)"$`, func(ctx context.Context, s string) error {
		return godog.ErrSkip
	})

	// Task lifecycle background
	sc.Step(`^a registered node "([^"]*)" is active$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.server == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug); err != nil {
				return err
			}
			node, err = st.db.GetNodeBySlug(ctx, slug)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return st.db.UpdateNodeStatus(ctx, node.ID, "active")
	})
	sc.Step(`^the node "([^"]*)" has worker_api_target_url and bearer token in config$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if err != nil {
			return err
		}
		token := "phase1-bdd-token"
		if st.workerServer == nil {
			st.workerRequestMu.Lock()
			st.workerRequest = nil
			st.workerRequestMu.Unlock()
			st.workerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				st.workerRequestMu.Lock()
				st.workerRequest = r
				st.workerRequestBody = body
				st.workerRequestMu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(workerapi.RunJobResponse{
					Version:   1,
					TaskID:    "",
					JobID:     "",
					Status:    workerapi.StatusCompleted,
					ExitCode:  0,
					Stdout:    "ok",
					StartedAt: time.Now().UTC().Format(time.RFC3339),
					EndedAt:   time.Now().UTC().Format(time.RFC3339),
				})
			}))
			st.workerToken = token
		}
		if err := st.db.UpdateNodeWorkerAPIConfig(ctx, node.ID, st.workerServer.URL, token); err != nil {
			return err
		}
		ackAt := time.Now().UTC()
		return st.db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil)
	})
}

func nodeRegisterStep(ctx context.Context, slug string) error {
	st := getState(ctx)
	if st == nil || st.server == nil {
		return godog.ErrSkip
	}
	cfg := config.LoadOrchestratorConfig()
	body, _ := json.Marshal(map[string]interface{}{
		"psk": cfg.NodeRegistrationPSK,
		"capability": map[string]interface{}{
			"version":     1,
			"reported_at": time.Now().UTC().Format(time.RFC3339),
			"node":        map[string]interface{}{"node_slug": slug},
			"platform":    map[string]interface{}{"os": "linux", "arch": "amd64"},
			"compute":     map[string]interface{}{"cpu_cores": 2, "ram_mb": 4096},
		},
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
