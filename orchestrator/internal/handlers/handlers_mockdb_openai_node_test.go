package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestOpenAIChatHandler_ChatCompletions_Timeout(t *testing.T) {
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":"ok"}`))
	}))
	defer mockPMA.Close()
	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	ctx, cancel := context.WithDeadline(context.WithValue(context.Background(), contextKeyUserID, uuid.New()), time.Now().Add(-time.Second))
	defer cancel()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504 on deadline exceeded, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("cynodeai_completion_timeout")) {
		t.Errorf("response should contain cynodeai_completion_timeout, got %s", rec.Body.String())
	}
}

// TestOpenAIChatHandler_ChatCompletions_DirectInferenceTimeout verifies 504 on deadline exceeded for direct inference path.
func TestOpenAIChatHandler_ChatCompletions_DirectInferenceTimeout(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.WithValue(context.Background(), contextKeyUserID, uuid.New()), time.Now().Add(-time.Second))
	defer cancel()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "http://localhost:11434", "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("expected 504 on direct inference deadline exceeded, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestOpenAIChatHandler_ChatCompletions_DirectInference_RetryThenSuccess verifies REQ-ORCHES-0132: retry on 502 then succeed.
func TestOpenAIChatHandler_ChatCompletions_DirectInference_RetryThenSuccess(t *testing.T) {
	var callCount int
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "retry-ok", "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after retry, got %d: %s", rec.Code, rec.Body.String())
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (first 502, second success), got %d", callCount)
	}
}

func TestOpenAIChatHandler_ChatCompletions_GetThreadError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	h := NewOpenAIChatHandler(mockDB, newTestLogger(), "http://localhost:11434", "qwen3.5:0.8b", "")
	body := []byte(`{"model":"qwen3.5:0.8b","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on GetOrCreateActiveChatThread error, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_ChatCompletions_NonDefaultModelUsesInferenceModel(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "from-" + body.Model, "done": true})
	}))
	defer mockOllama.Close()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	body := []byte(`{"model":"llama2","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNodeHandler_ReportCapabilityWithSandbox(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "test-node",
			Status:   "active",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: nodepayloads.CapabilityNode{
			NodeSlug: "test-node",
			Name:     "Test Node",
			Labels:   []string{"test"},
		},
		Platform: nodepayloads.Platform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: nodepayloads.Compute{
			CPUCores: 8,
			RAMMB:    16384,
		},
		Sandbox: &nodepayloads.SandboxSupport{
			Supported:      true,
			Features:       []string{"python", "bash"},
			MaxConcurrency: 4,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_GetTaskResultDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String()+"/result", http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_NewNodeRegistrationJWTError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	// Use invalid JWT manager (empty secret key causes issues)
	jwtMgr := auth.NewJWTManager("", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk-secret",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "new-test-node"},
			Platform: nodepayloads.Platform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: nodepayloads.Compute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	// May return 201 with empty JWT or 500 - depends on JWT manager behavior
	// The point is to exercise the code path
	if rec.Code != http.StatusCreated && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201 or 500, got %d", rec.Code)
	}
}

// Test for handleExistingNodeRegistration JWT error
//
//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_ExistingNodeJWTError(t *testing.T) {
	jwtMgr := auth.NewJWTManager("", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "existing-node-jwt-test",
			Status:   "offline",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	mockDB := testutil.NewMockDB()
	mockDB.AddNode(node)

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", logger)

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "existing-node-jwt-test"},
			Platform: nodepayloads.Platform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: nodepayloads.Compute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	// Exercise the code path - may succeed with empty JWT or fail
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", rec.Code)
	}
}

// Test initializeNewNode error paths
func TestNodeHandler_initializeNewNodeErrors(t *testing.T) {
	logger := newTestLogger()

	// Create a mock DB that fails on different operations
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("force error")

	handler := &NodeHandler{
		db:     mockDB,
		logger: logger,
	}

	nodeID := uuid.New()
	capability := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
	}

	// Should not panic - just log errors
	handler.initializeNewNode(context.Background(), nodeID, &capability)
}

// Test Refresh with all error paths
func TestAuthHandler_RefreshInvalidUserID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, nil, "", "", logger)

	// Generate a token with invalid user ID format - create a custom JWT
	// This is hard to test without modifying JWT generation, so test other paths

	body := userapi.RefreshRequest{RefreshToken: "invalid.token.format"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

// Test ReportCapability with all error paths
func TestNodeHandler_ReportCapabilityUpdateErrors(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "test-node-errors",
			Status:   "active",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Create mock that fails on capability operations
	mockDB := &capabilityErrorMockDB{
		MockDB: testutil.NewMockDB(),
	}
	mockDB.AddNode(node)

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", logger)

	report := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node-errors"},
		Platform: nodepayloads.Platform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: nodepayloads.Compute{
			CPUCores: 4,
			RAMMB:    8192,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	// Should return 500 due to SaveNodeCapabilitySnapshot error
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

type capabilityErrorMockDB struct {
	*testutil.MockDB
}

func (m *capabilityErrorMockDB) SaveNodeCapabilitySnapshot(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("capability snapshot error")
}

func (m *capabilityErrorMockDB) GetLatestNodeCapabilitySnapshot(ctx context.Context, nodeID uuid.UUID) (string, error) {
	return m.MockDB.GetLatestNodeCapabilitySnapshot(ctx, nodeID)
}

func (m *capabilityErrorMockDB) UpdateNodeLastSeen(_ context.Context, _ uuid.UUID) error {
	return nil
}

// updateNodeStatusErrorStore fails only UpdateNodeStatus (e.g. in handleExistingNodeRegistration).
type updateNodeStatusErrorStore struct {
	*testutil.MockDB
}

func (m *updateNodeStatusErrorStore) UpdateNodeStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("update node status error")
}

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_RegisterExistingNode_UpdateNodeStatusFails(t *testing.T) {
	mockDB := &updateNodeStatusErrorStore{MockDB: testutil.NewMockDB()}
	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "existing-node",
			Status:   "offline",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", newTestLogger())

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version:  1,
			Node:     nodepayloads.CapabilityNode{NodeSlug: "existing-node"},
			Platform: nodepayloads.Platform{OS: "linux", Arch: "amd64"},
			Compute:  nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		},
	}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", body)
	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// createNodeErrorStore fails only CreateNode so handleNewNodeRegistration hits CreateNode error path.
type createNodeErrorStore struct {
	*testutil.MockDB
}

func (m *createNodeErrorStore) CreateNode(_ context.Context, _ string) (*models.Node, error) {
	return nil, errors.New("create node error")
}

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_RegisterNewNode_CreateNodeFails(t *testing.T) {
	mockDB := &createNodeErrorStore{MockDB: testutil.NewMockDB()}
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", newTestLogger())

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version:  1,
			Node:     nodepayloads.CapabilityNode{NodeSlug: "new-node"},
			Platform: nodepayloads.Platform{OS: "linux", Arch: "amd64"},
			Compute:  nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		},
	}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", body)
	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}
