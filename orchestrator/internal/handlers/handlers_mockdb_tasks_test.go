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

func TestTaskHandler_GetTaskSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	prompt := testPrompt
	userID := uuid.New()
	task := newMockTask(&userID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTaskHandler_GetTaskNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String(), http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String(), http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTask(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskResultSuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	prompt := testPrompt
	userID := uuid.New()
	task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(task)

	result := "job result"
	startedAt := time.Now().UTC()
	endedAt := startedAt.Add(time.Second)
	job := &models.Job{
		JobBase: models.JobBase{
			TaskID:    task.ID,
			Status:    models.JobStatusCompleted,
			Result:    models.NewJSONBString(&result),
			StartedAt: &startedAt,
			EndedAt:   &endedAt,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddJob(job)

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/result", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp userapi.TaskResultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(resp.Jobs))
	}
}

func TestTaskHandler_InvalidPaginationQuery(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewTaskHandler(mockDB, newTestLogger(), "", "")
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)
	mockDB.AddTask(task)
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	cases := []struct {
		name string
		path string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"result_limit", "/v1/tasks/" + task.ID.String() + "/result?limit=0", handler.GetTaskResult},
		{"logs_offset", "/v1/tasks/" + task.ID.String() + "/logs?offset=-1", handler.GetTaskLogs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", c.path, http.NoBody).WithContext(ctx)
			req.SetPathValue("id", task.ID.String())
			rec := httptest.NewRecorder()
			c.fn(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("want 400, got %d", rec.Code)
			}
		})
	}
}

func TestTaskHandler_GetTaskResultNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()

	handler := NewTaskHandler(mockDB, logger, "", "")

	taskID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/tasks/"+taskID.String()+"/result", http.NoBody)
	req.SetPathValue("id", taskID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

// --- Node Handler Tests ---

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_RegisterNewNode(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	body := nodepayloads.RegistrationRequest{PSK: "test-psk-secret", Capability: testNodeCapabilityReport("test-node-1", "Test Node 1", 8, 16384)}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp nodepayloads.BootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT == "" {
		t.Error("expected node JWT")
	}
	if resp.Orchestrator.BaseURL != testOrchestratorURL {
		t.Errorf("expected base_url %q, got %s", testOrchestratorURL, resp.Orchestrator.BaseURL)
	}
	if resp.Orchestrator.Endpoints.NodeReportURL == "" {
		t.Error("expected NodeReportURL in bootstrap")
	}
	if resp.Orchestrator.Endpoints.NodeConfigURL == "" {
		t.Error("expected NodeConfigURL in bootstrap")
	}
}

func TestNodeHandler_RegisterNewNode_StoresWorkerAPIURLFromCapability(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()
	workerURL := "http://worker-01.example.com:12090"
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "bearer-token", "", "", nil, "", "", logger)

	capReport := testNodeCapabilityReport("node-with-url", "Node With URL", 4, 8192)
	capReport.WorkerAPI = &nodepayloads.WorkerAPIReport{BaseURL: workerURL}
	body := nodepayloads.RegistrationRequest{PSK: "test-psk-secret", Capability: capReport}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	node, err := mockDB.GetNodeBySlug(context.Background(), "node-with-url")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.WorkerAPITargetURL == nil || *node.WorkerAPITargetURL != workerURL {
		t.Errorf("expected worker_api_target_url %q, got %v", workerURL, node.WorkerAPITargetURL)
	}
}

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_RegisterExistingNode(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	// Create existing node
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

	body := nodepayloads.RegistrationRequest{PSK: "test-psk-secret", Capability: testNodeCapabilityReport("existing-node", "", 4, 8192)}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_RegisterDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk-secret",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
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

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ReportCapabilitySuccess(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	// Create node
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

	report := testNodeCapabilityReport("test-node", "Test Node", 8, 16384)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Node Config (GET/POST /v1/nodes/config) Tests ---

func TestNodeHandler_GetConfig_Success(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "bearer-token-1", "http://node:12090", "", nil, "", "", logger)

	cfgVer := "1"
	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug:      "cfg-node",
			Status:        models.NodeStatusActive,
			ConfigVersion: &cfgVer,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload.Version != 1 || payload.ConfigVersion != "1" || payload.NodeSlug != "cfg-node" {
		t.Errorf("unexpected payload: version=%d config_version=%s node_slug=%s", payload.Version, payload.ConfigVersion, payload.NodeSlug)
	}
	if payload.WorkerAPI == nil || payload.WorkerAPI.OrchestratorBearerToken != "bearer-token-1" {
		t.Errorf("expected worker_api.orchestrator_bearer_token, got %+v", payload.WorkerAPI)
	}
	if payload.Orchestrator.Endpoints.WorkerAPITargetURL != "http://node:12090" {
		t.Errorf("expected worker_api_target_url from handler, got %s", payload.Orchestrator.Endpoints.WorkerAPITargetURL)
	}
}

func TestNodeHandler_GetConfig_ReturnsInferenceBackendWhenCapabilityInferenceSupported(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "bearer-token", "http://node:12090", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "inference-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	capJSON := `{"version":1,"reported_at":"2026-02-28T12:00:00Z","node":{"node_slug":"inference-node"},"platform":{"os":"linux","arch":"amd64"},"compute":{"cpu_cores":4,"ram_mb":8192},"inference":{"supported":true,"existing_service":false,"running":false}}`
	if err := mockDB.SaveNodeCapabilitySnapshot(context.Background(), node.ID, capJSON); err != nil {
		t.Fatalf("save capability: %v", err)
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload.InferenceBackend == nil {
		t.Error("expected inference_backend in payload when node is inference-capable and not existing_service")
	}
	if payload.InferenceBackend != nil && !payload.InferenceBackend.Enabled {
		t.Error("expected inference_backend.enabled true")
	}
}

func TestNodeHandler_GetConfig_OmitsInferenceBackendWhenCapabilityExistingService(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "bearer-token", "http://node:12090", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "existing-inference-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	capJSON := `{"version":1,"reported_at":"2026-02-28T12:00:00Z","node":{"node_slug":"existing-inference-node"},"inference":{"supported":true,"existing_service":true,"running":true}}`
	if err := mockDB.SaveNodeCapabilitySnapshot(context.Background(), node.ID, capJSON); err != nil {
		t.Fatalf("save capability: %v", err)
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	// InferenceBackend is returned even for existing-service nodes so that node-manager
	// can pull the selected model and apply env settings. Enabled must be false so no
	// second container is started. SelectedModel must be set for the pull to be triggered.
	if payload.InferenceBackend == nil {
		t.Fatal("expected inference_backend in config even when node reports existing_service true")
	}
	if payload.InferenceBackend.Enabled {
		t.Error("InferenceBackend.Enabled must be false when node reports existing_service true")
	}
	if payload.InferenceBackend.SelectedModel == "" {
		t.Error("InferenceBackend.SelectedModel must be set so node-manager can pull the chosen model")
	}
	if len(payload.InferenceBackend.ModelsToEnsure) == 0 {
		t.Error("InferenceBackend.ModelsToEnsure must list orchestrator-directed models to pull")
	}
}

func TestNodeHandler_GetConfig_ReturnsInferenceBackendWithVariantFromGPU(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "bearer-token", "http://node:12090", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "gpu-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	capJSON := `{"version":1,"reported_at":"2026-02-28T12:00:00Z","node":{"node_slug":"gpu-node"},"platform":{"os":"linux","arch":"amd64"},"compute":{"cpu_cores":8,"ram_mb":16384},"gpu":{"present":true,"devices":[{"vendor":"AMD","features":{"rocm_version":"5.0"}}]},"inference":{"supported":true,"existing_service":false}}`
	if err := mockDB.SaveNodeCapabilitySnapshot(context.Background(), node.ID, capJSON); err != nil {
		t.Fatalf("save capability: %v", err)
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload nodepayloads.NodeConfigurationPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload.InferenceBackend == nil || payload.InferenceBackend.Variant != "rocm" {
		t.Errorf("expected inference_backend with variant=rocm from GPU capability, got %+v", payload.InferenceBackend)
	}
}

func TestNodeHandler_GetConfig_NoNodeID(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeHandler_GetConfig_NodeNotFound(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, uuid.New())
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_Success(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", logger)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "ack-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ack := nodepayloads.ConfigAck{
		Version:       1,
		NodeSlug:      "ack-node",
		ConfigVersion: "1",
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        "applied",
	}
	jsonBody, _ := json.Marshal(ack)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	// Node should have config ack fields updated (via mock)
	updated, _ := mockDB.GetNodeByID(context.Background(), node.ID)
	if updated.ConfigAckStatus == nil || *updated.ConfigAckStatus != "applied" {
		t.Errorf("expected config_ack_status applied, got %v", updated.ConfigAckStatus)
	}
}

func TestNodeHandler_ConfigAck_NoNodeID(t *testing.T) {
	handler := NewNodeHandler(nil, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	ack := nodepayloads.ConfigAck{Version: 1, NodeSlug: "x", ConfigVersion: "1", AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied"}
	jsonBody, _ := json.Marshal(ack)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_BadRequestCases(t *testing.T) {
	tests := []struct {
		name     string
		ack      nodepayloads.ConfigAck
		nodeSlug string
	}{
		{"bad slug", nodepayloads.ConfigAck{
			Version: 1, NodeSlug: "wrong-slug", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied",
		}, "real-slug"},
		{"invalid status", nodepayloads.ConfigAck{
			Version: 1, NodeSlug: "status-node", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "rolled_back",
		}, "status-node"},
		{"unsupported version", nodepayloads.ConfigAck{
			Version: 2, NodeSlug: "ver-node", ConfigVersion: "1",
			AckAt: time.Now().UTC().Format(time.RFC3339), Status: "applied",
		}, "ver-node"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := testutil.NewMockDB()
			handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)
			node := &models.Node{
				NodeBase: models.NodeBase{
					NodeSlug: tt.nodeSlug,
					Status:   models.NodeStatusActive,
				},
				ID:        uuid.New(),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			mockDB.AddNode(node)
			jsonBody, _ := json.Marshal(tt.ack)
			ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
			req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.ConfigAck(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rec.Code)
			}
		})
	}
}

func TestNodeHandler_GetConfig_SetsConfigVersionWhenNil(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "tok", "", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "no-ver-node",
			Status:   models.NodeStatusActive,
			// ConfigVersion nil
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Handler should have persisted a ULID config_version (26-char Crockford Base32)
	updated, _ := mockDB.GetNodeByID(context.Background(), node.ID)
	if updated.ConfigVersion == nil || len(*updated.ConfigVersion) != 26 {
		t.Errorf("expected config_version ULID (26 chars) to be set, got %v", updated.ConfigVersion)
	}
}

func TestNodeHandler_ConfigAck_InvalidBody(t *testing.T) {
	mockDB := testutil.NewMockDB()
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "body-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBufferString("{invalid}")).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestNodeHandler_GetConfig_DBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "db-err-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("GET", "/v1/nodes/config", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetConfig(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ConfigAck_DBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("db error")
	handler := NewNodeHandler(mockDB, nil, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", nil)

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "ack-db-node",
			Status:   models.NodeStatusActive,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)

	ack := nodepayloads.ConfigAck{
		Version:       1,
		NodeSlug:      "ack-db-node",
		ConfigVersion: "1",
		AckAt:         time.Now().UTC().Format(time.RFC3339),
		Status:        "applied",
	}
	jsonBody, _ := json.Marshal(ack)
	ctx := context.WithValue(context.Background(), contextKeyNodeID, node.ID)
	req := httptest.NewRequest("POST", "/v1/nodes/config", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ConfigAck(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestNodeHandler_ReportCapabilityDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk-secret", testOrchestratorURL, "", "", "", nil, "", "", logger)

	nodeID := uuid.New()
	report := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
		Platform: nodepayloads.Platform{
			OS:   "linux",
			Arch: "amd64",
		},
		Compute: nodepayloads.Compute{
			CPUCores: 4,
			RAMMB:    8192,
		},
	}

	ctx := context.WithValue(context.Background(), contextKeyNodeID, nodeID)
	jsonBody, _ := json.Marshal(report)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// --- AuditLog Tests ---

func TestAuditLogWithDB(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, nil, "", "", logger)

	userID := uuid.New()
	handler.auditLog(context.Background(), &userID, "test_event", true, "192.168.1.1", "Mozilla/5.0", "test detail")

	// Check audit log was created
	if len(mockDB.AuditLogs) != 1 {
		t.Errorf("expected 1 audit log, got %d", len(mockDB.AuditLogs))
	}
}

func TestAuditLogDBError(t *testing.T) {
	mockDB := testutil.NewMockDB()
	mockDB.ForceError = errors.New("database error")
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, nil, "", "", logger)

	// Should not panic
	handler.auditLog(context.Background(), nil, "test_event", false, "", "", "")
}

// Additional tests to increase coverage

func TestAuthHandler_RefreshDBErrorOnInvalidate(t *testing.T) {
	mockDB := testutil.NewMockDB()
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, nil, "", "", logger)

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddUser(user)

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)
	mockDB.AddRefreshSession(session)

	// Set error after getting session
	mockDB.ForceError = errors.New("database error")

	body := userapi.RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestAuthHandler_RefreshUserNotFound(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	rateLimiter := auth.NewRateLimiter(100, time.Minute)
	logger := newTestLogger()

	user := &models.User{
		UserBase: models.UserBase{
			Handle:   "testuser",
			IsActive: true,
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	refreshToken, expiresAt, _ := jwtMgr.GenerateRefreshToken(user.ID)
	tokenHash := auth.HashToken(refreshToken)

	session := newMockRefreshSession(user.ID, tokenHash, expiresAt)

	// Create mockDB with session but no user
	mockDB := testutil.NewMockDB()
	mockDB.AddRefreshSession(session)

	handler := NewAuthHandler(mockDB, jwtMgr, rateLimiter, nil, "", "", logger)

	body := userapi.RefreshRequest{RefreshToken: refreshToken}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}
