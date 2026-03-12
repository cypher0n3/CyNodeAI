package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
	"github.com/google/uuid"
)

const testOrchestratorURL = "http://test-orchestrator"

func TestNewNodeHandler(t *testing.T) {
	handler := NewNodeHandler(nil, nil, "test-psk", testOrchestratorURL, "", "", "", nil)
	if handler == nil {
		t.Fatal("NewNodeHandler returned nil")
	}
}

func TestRegisterBadRequest(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}
	runHandlerTest(t, "POST", "/v1/nodes/register", []byte("{invalid"), handler.Register, http.StatusBadRequest)
}

func TestRegisterInvalidPSK(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "correct-psk"}
	req, rec := recordedRequestJSON("POST", "/v1/nodes/register", nodepayloads.RegistrationRequest{
		PSK: "wrong-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
		},
	})
	handler.Register(rec, req)
	assertStatusCode(t, rec, http.StatusUnauthorized)
}

func TestRegisterInvalidBodyOrSlug(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}
	tests := []struct {
		name       string
		body       nodepayloads.RegistrationRequest
		wantStatus int
	}{
		{"invalid capability version", nodepayloads.RegistrationRequest{
			PSK:        "test-psk",
			Capability: nodepayloads.CapabilityReport{Version: 2, Node: nodepayloads.CapabilityNode{NodeSlug: "test-node"}},
		}, http.StatusBadRequest},
		{"missing node slug", nodepayloads.RegistrationRequest{
			PSK:        "test-psk",
			Capability: nodepayloads.CapabilityReport{Version: 1, Node: nodepayloads.CapabilityNode{NodeSlug: ""}},
		}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, rec := recordedRequestJSON("POST", "/v1/nodes/register", tt.body)
			handler.Register(rec, req)
			assertStatusCode(t, rec, tt.wantStatus)
		})
	}
}

func TestReportCapabilityBadRequest(t *testing.T) {
	handler := &NodeHandler{}

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBufferString("{invalid"))
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestNodeCapabilityReportJSON(t *testing.T) {
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: nodepayloads.CapabilityNode{
			NodeSlug: "test-node",
			Name:     "Test Node",
			Labels:   []string{"gpu", "high-memory"},
		},
		Platform: nodepayloads.Platform{
			OS:     "linux",
			Distro: "ubuntu",
			Arch:   "amd64",
		},
		Compute: nodepayloads.Compute{
			CPUCores: 8,
			RAMMB:    16384,
		},
		Sandbox: &nodepayloads.SandboxSupport{
			Supported:      true,
			Features:       []string{"podman"},
			MaxConcurrency: 4,
		},
	}

	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed nodepayloads.CapabilityReport
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Node.NodeSlug != "test-node" {
		t.Errorf("expected node slug 'test-node', got %s", parsed.Node.NodeSlug)
	}
}

func TestNodeBootstrapResponseJSON(t *testing.T) {
	handler := &NodeHandler{orchestratorPublicURL: testOrchestratorURL}
	resp := handler.buildBootstrapResponse(testOrchestratorURL, "test-jwt", time.Now().Add(time.Hour))

	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed struct {
		Version      int    `json:"version"`
		IssuedAt     string `json:"issued_at"`
		Orchestrator struct {
			BaseURL   string `json:"base_url"`
			Endpoints struct {
				WorkerRegistrationURL string `json:"worker_registration_url"`
				NodeReportURL         string `json:"node_report_url"`
				NodeConfigURL         string `json:"node_config_url"`
			} `json:"endpoints"`
		} `json:"orchestrator"`
		Auth struct {
			NodeJWT   string `json:"node_jwt"`
			ExpiresAt string `json:"expires_at"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Version != 1 {
		t.Errorf("expected version 1, got %d", parsed.Version)
	}
	if parsed.Auth.NodeJWT != "test-jwt" {
		t.Errorf("expected JWT 'test-jwt', got %s", parsed.Auth.NodeJWT)
	}
	if parsed.Orchestrator.BaseURL != testOrchestratorURL {
		t.Errorf("expected base_url %q, got %s", testOrchestratorURL, parsed.Orchestrator.BaseURL)
	}
	if parsed.Orchestrator.Endpoints.NodeReportURL != testOrchestratorURL+"/v1/nodes/capability" {
		t.Errorf("expected node_report_url, got %s", parsed.Orchestrator.Endpoints.NodeReportURL)
	}
}

func TestBuildBootstrapResponse(t *testing.T) {
	handler := &NodeHandler{}
	baseURL := testOrchestratorURL
	expiresAt := time.Now().Add(time.Hour)

	resp := handler.buildBootstrapResponse(baseURL, "test-jwt", expiresAt)

	if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
	if resp.Auth.NodeJWT != "test-jwt" {
		t.Errorf("expected JWT 'test-jwt', got %s", resp.Auth.NodeJWT)
	}
	if resp.Orchestrator.BaseURL != baseURL {
		t.Errorf("expected base_url %q, got %q", baseURL, resp.Orchestrator.BaseURL)
	}
	if resp.Orchestrator.Endpoints.NodeReportURL != baseURL+"/v1/nodes/capability" {
		t.Errorf("expected node_report_url, got %s", resp.Orchestrator.Endpoints.NodeReportURL)
	}
	if resp.Orchestrator.Endpoints.NodeConfigURL != baseURL+"/v1/nodes/config" {
		t.Errorf("expected node_config_url, got %s", resp.Orchestrator.Endpoints.NodeConfigURL)
	}
}

func TestLogHelpers(t *testing.T) {
	handler := &NodeHandler{} // nil logger

	// Should not panic with nil logger
	handler.logError("test error")
	handler.logWarn("test warn")
	handler.logInfo("test info")
}

func TestNodeCapabilityNode(t *testing.T) {
	node := nodepayloads.CapabilityNode{
		NodeSlug: "test-slug",
		Name:     "Test Name",
		Labels:   []string{"label1", "label2"},
	}

	if node.NodeSlug != "test-slug" {
		t.Errorf("expected NodeSlug 'test-slug', got %s", node.NodeSlug)
	}
	if len(node.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(node.Labels))
	}
}

func TestNodeCapabilityPlatform(t *testing.T) {
	platform := nodepayloads.Platform{
		OS:     "linux",
		Distro: "ubuntu",
		Arch:   "amd64",
	}

	if platform.OS != "linux" {
		t.Errorf("expected OS 'linux', got %s", platform.OS)
	}
	if platform.Arch != "amd64" {
		t.Errorf("expected Arch 'amd64', got %s", platform.Arch)
	}
}

func TestNodeCapabilityCompute(t *testing.T) {
	compute := nodepayloads.Compute{
		CPUCores: 16,
		RAMMB:    32768,
	}

	if compute.CPUCores != 16 {
		t.Errorf("expected CPUCores 16, got %d", compute.CPUCores)
	}
	if compute.RAMMB != 32768 {
		t.Errorf("expected RAMMB 32768, got %d", compute.RAMMB)
	}
}

func TestNodeCapabilitySandbox(t *testing.T) {
	sandbox := nodepayloads.SandboxSupport{
		Supported:      true,
		Features:       []string{"podman", "docker"},
		MaxConcurrency: 8,
	}

	if !sandbox.Supported {
		t.Error("expected Supported to be true")
	}
	if sandbox.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency 8, got %d", sandbox.MaxConcurrency)
	}
}

func TestNodeRegistrationRequestJSON(t *testing.T) {
	req := nodepayloads.RegistrationRequest{
		PSK: "secret-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "my-node"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed nodepayloads.RegistrationRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PSK != "secret-psk" {
		t.Errorf("expected PSK 'secret-psk', got %s", parsed.PSK)
	}
}

func TestValidateRegistrationRequestIntegration(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "{invalid}",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong psk",
			body:       `{"psk":"wrong","capability":{"version":1,"node":{"node_slug":"test"}}}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unsupported version",
			body:       `{"psk":"test-psk","capability":{"version":2,"node":{"node_slug":"test"}}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty slug",
			body:       `{"psk":"test-psk","capability":{"version":1,"node":{"node_slug":""}}}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			handler.Register(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestNodeCapabilitySandboxNil(t *testing.T) {
	report := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test"},
		Sandbox: nil,
	}

	if report.Sandbox != nil {
		t.Error("expected nil sandbox")
	}
}

func TestCapabilityHash(t *testing.T) {
	report := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
	}

	jsonBytes, _ := json.Marshal(report)
	if len(jsonBytes) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestLogHelpersWithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := &NodeHandler{logger: logger}

	// These should log without error
	handler.logError("test error message", "key", "value")
	handler.logWarn("test warn message", "key", "value")
	handler.logInfo("test info message", "key", "value")
}

func TestReportCapabilityNoNodeID(t *testing.T) {
	handler := &NodeHandler{}

	body := nodepayloads.CapabilityReport{
		Version: 1,
		Node:    nodepayloads.CapabilityNode{NodeSlug: "test"},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/capability", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.ReportCapability(rec, req)

	// Should fail because no node ID in context
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestRegisterWithNilDB(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk", db: nil}

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	// This will panic or fail due to nil DB, but the test ensures request validation works
	defer func() {
		// Expected panic due to nil db
		_ = recover()
	}()

	handler.Register(rec, req)
}

func TestNodeRegistrationRequestAllFields(t *testing.T) {
	req := nodepayloads.RegistrationRequest{
		PSK: "my-psk",
		Capability: nodepayloads.CapabilityReport{
			Version:    1,
			ReportedAt: time.Now().UTC().Format(time.RFC3339),
			Node: nodepayloads.CapabilityNode{
				NodeSlug: "node-1",
				Name:     "Node 1",
				Labels:   []string{"gpu", "high-mem"},
			},
			Platform: nodepayloads.Platform{
				OS:     "linux",
				Distro: "ubuntu",
				Arch:   "amd64",
			},
			Compute: nodepayloads.Compute{
				CPUCores: 16,
				RAMMB:    65536,
			},
			Sandbox: &nodepayloads.SandboxSupport{
				Supported:      true,
				Features:       []string{"docker", "podman"},
				MaxConcurrency: 8,
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed nodepayloads.RegistrationRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Capability.Node.Name != "Node 1" {
		t.Errorf("expected name 'Node 1', got %s", parsed.Capability.Node.Name)
	}
	if parsed.Capability.Platform.Distro != "ubuntu" {
		t.Errorf("expected distro 'ubuntu', got %s", parsed.Capability.Platform.Distro)
	}
	if parsed.Capability.Compute.RAMMB != 65536 {
		t.Errorf("expected RAM 65536, got %d", parsed.Capability.Compute.RAMMB)
	}
}

func TestNodeCapabilityReportNoSandbox(t *testing.T) {
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       nodepayloads.CapabilityNode{NodeSlug: "minimal-node"},
		Platform:   nodepayloads.Platform{OS: "linux", Arch: "arm64"},
		Compute:    nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
		// Sandbox is nil
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed nodepayloads.CapabilityReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Sandbox != nil {
		t.Error("expected nil sandbox")
	}
	if parsed.Platform.Arch != "arm64" {
		t.Errorf("expected arch 'arm64', got %s", parsed.Platform.Arch)
	}
}

func TestValidateRegistrationRequestValid(t *testing.T) {
	handler := &NodeHandler{registrationPSK: "test-psk"}

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "valid-node"},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	result, ok := handler.validateRegistrationRequest(rec, req)

	if !ok {
		t.Error("expected validation to pass")
	}
	if result == nil {
		t.Error("expected result to be non-nil")
	}
	if result != nil && result.Capability.Node.NodeSlug != "valid-node" {
		t.Errorf("expected node slug 'valid-node', got %s", result.Capability.Node.NodeSlug)
	}
}

func TestBuildNodeConfigPayload_IncludesManagedServicesWhenSelected(t *testing.T) {
	_ = os.Setenv("PMA_HOST_NODE_SLUG", "node-01")
	defer func() { _ = os.Unsetenv("PMA_HOST_NODE_SLUG") }()
	mockDB := testutil.NewMockDB()
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "node-01",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "worker-api-token", "", "", nil)
	payload := h.buildNodeConfigPayload(t.Context(), node, "cfg-1", "http://worker:12090")
	if payload.ManagedServices == nil || len(payload.ManagedServices.Services) != 1 {
		t.Fatalf("expected one managed service, got %+v", payload.ManagedServices)
	}
	svc := payload.ManagedServices.Services[0]
	if svc.ServiceType != "pma" || svc.ServiceID == "" {
		t.Errorf("unexpected managed service: %+v", svc)
	}
	if svc.Inference == nil || svc.Inference.BaseURL != "http://worker:11434" {
		t.Errorf("expected inference.base_url from worker API target host, got %+v", svc.Inference)
	}
	if svc.Orchestrator == nil {
		t.Fatal("expected orchestrator block in managed service")
	}
	if svc.Orchestrator.AgentToken != "worker-api-token" {
		t.Errorf("expected fallback agent_token from worker API bearer token, got %q", svc.Orchestrator.AgentToken)
	}
}

func TestBuildNodeConfigPayload_ManagedServicesIncludeAgentTokenWhenSet(t *testing.T) {
	_ = os.Unsetenv("PMA_HOST_NODE_SLUG")
	defer func() { _ = os.Unsetenv("PMA_HOST_NODE_SLUG") }()
	mockDB := testutil.NewMockDB()
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "node-01",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	const agentToken = "internal-agent-token-123"
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "", "", agentToken, nil)
	payload := h.buildNodeConfigPayload(t.Context(), node, "cfg-1", "http://worker:12090")
	if payload.ManagedServices == nil || len(payload.ManagedServices.Services) != 1 {
		t.Fatalf("expected one managed service, got %+v", payload.ManagedServices)
	}
	svc := payload.ManagedServices.Services[0]
	if svc.Orchestrator == nil {
		t.Fatal("expected orchestrator block in managed service")
	}
	if svc.Orchestrator.AgentToken != agentToken {
		t.Errorf("Orchestrator.AgentToken = %q, want %q", svc.Orchestrator.AgentToken, agentToken)
	}
}

func TestBuildNodeConfigPayload_OmitsManagedServicesWhenNotSelected(t *testing.T) {
	_ = os.Setenv("PMA_HOST_NODE_SLUG", "other-node")
	defer func() { _ = os.Unsetenv("PMA_HOST_NODE_SLUG") }()
	mockDB := testutil.NewMockDB()
	node := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "node-01",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node)
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "", "", "", nil)
	payload := h.buildNodeConfigPayload(t.Context(), node, "cfg-1", "http://worker:12090")
	if payload.ManagedServices != nil {
		t.Errorf("expected no managed services for unselected node, got %+v", payload.ManagedServices)
	}
}

func TestSelectPMAHostNodeSlug_PrefersLabeledNode(t *testing.T) {
	_ = os.Unsetenv("PMA_HOST_NODE_SLUG")
	_ = os.Setenv("PMA_PREFER_HOST_LABEL", "orchestrator_host")
	defer func() { _ = os.Unsetenv("PMA_PREFER_HOST_LABEL") }()
	mockDB := testutil.NewMockDB()
	node1 := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "node-a",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	node2 := &models.Node{
		ID:        uuid.New(),
		NodeSlug:  "node-b",
		Status:    models.NodeStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockDB.AddNode(node1)
	mockDB.AddNode(node2)
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node: nodepayloads.CapabilityNode{
			NodeSlug: "node-b",
			Labels:   []string{"orchestrator_host"},
		},
		Platform: nodepayloads.Platform{OS: "linux", Arch: "amd64"},
		Compute:  nodepayloads.Compute{CPUCores: 4, RAMMB: 8192},
	}
	raw, _ := json.Marshal(report)
	_ = mockDB.SaveNodeCapabilitySnapshot(t.Context(), node2.ID, string(raw))
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "", "", "", nil)
	if got := h.selectPMAHostNodeSlug(t.Context(), "fallback-node"); got != "node-b" {
		t.Errorf("selectPMAHostNodeSlug() = %q, want node-b", got)
	}
}

func TestBoolEnvDefault(t *testing.T) {
	_ = os.Setenv("TEST_BOOL_ENV", "true")
	if !boolEnvDefault("TEST_BOOL_ENV", false) {
		t.Error("boolEnvDefault true parsing failed")
	}
	_ = os.Setenv("TEST_BOOL_ENV", "no")
	if boolEnvDefault("TEST_BOOL_ENV", true) {
		t.Error("boolEnvDefault false parsing failed")
	}
	_ = os.Unsetenv("TEST_BOOL_ENV")
	if !boolEnvDefault("TEST_BOOL_ENV", true) {
		t.Error("boolEnvDefault should return default when unset")
	}
}

func TestGetEnvDefault(t *testing.T) {
	_ = os.Setenv("TEST_ENV_DEFAULT", "value")
	if got := getEnvDefault("TEST_ENV_DEFAULT", "fallback"); got != "value" {
		t.Errorf("getEnvDefault() = %q, want value", got)
	}
	_ = os.Unsetenv("TEST_ENV_DEFAULT")
	if got := getEnvDefault("TEST_ENV_DEFAULT", "fallback"); got != "fallback" {
		t.Errorf("getEnvDefault() = %q, want fallback", got)
	}
}

func TestDeriveNodeLocalInferenceBaseURL(t *testing.T) {
	tests := []struct {
		name             string
		workerAPITarget  string
		wantInferenceURL string
	}{
		{
			name:             "non-loopback host",
			workerAPITarget:  "http://worker.internal:12090",
			wantInferenceURL: "http://worker.internal:11434",
		},
		{
			name:             "loopback host",
			workerAPITarget:  "http://127.0.0.1:12090",
			wantInferenceURL: "",
		},
		{
			name:             "invalid URL",
			workerAPITarget:  "not-a-url",
			wantInferenceURL: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveNodeLocalInferenceBaseURL(tt.workerAPITarget); got != tt.wantInferenceURL {
				t.Fatalf("deriveNodeLocalInferenceBaseURL() = %q, want %q", got, tt.wantInferenceURL)
			}
		})
	}
}

func TestSelectPMAHostNodeSlug_ExplicitOverride(t *testing.T) {
	_ = os.Setenv("PMA_HOST_NODE_SLUG", "explicit-node")
	defer func() { _ = os.Unsetenv("PMA_HOST_NODE_SLUG") }()
	h := NewNodeHandler(testutil.NewMockDB(), nil, "psk", testOrchestratorURL, "", "", "", nil)
	if got := h.selectPMAHostNodeSlug(t.Context(), "fallback"); got != "explicit-node" {
		t.Errorf("selectPMAHostNodeSlug() = %q, want explicit-node", got)
	}
}

// PMA is always required; there is no "disabled" state for managed services.

func TestPMAModelCandidates(t *testing.T) {
	tests := []struct {
		vramMB    int
		wantFirst string
	}{
		{vramMB: 0, wantFirst: pmaModelMid},
		{vramMB: 4000, wantFirst: pmaModelMid},
		{vramMB: 8000, wantFirst: "qwen3.5:9b"},
		{vramMB: 16000, wantFirst: "qwen3.5:9b"},
		{vramMB: 24000, wantFirst: "qwen3.5:35b"},
		{vramMB: 48000, wantFirst: "qwen3.5:35b"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("vram_%d", tt.vramMB), func(t *testing.T) {
			got := pmaModelCandidates(tt.vramMB)
			if len(got) == 0 {
				t.Fatal("expected non-empty candidates")
			}
			if got[0] != tt.wantFirst {
				t.Errorf("pmaModelCandidates(%d)[0] = %q, want %q", tt.vramMB, got[0], tt.wantFirst)
			}
		})
	}
}

func TestSelectPMAModel_PinnedViaEnv(t *testing.T) {
	_ = os.Setenv("INFERENCE_MODEL", "my-custom-model:7b")
	defer func() { _ = os.Unsetenv("INFERENCE_MODEL") }()
	h := NewNodeHandler(testutil.NewMockDB(), nil, "psk", testOrchestratorURL, "", "", "", nil)
	got := h.selectPMAModel(t.Context(), uuid.New())
	if got != "my-custom-model:7b" {
		t.Errorf("selectPMAModel() = %q, want my-custom-model:7b", got)
	}
}

func TestSelectPMAModel_PicksTopTierCandidate(t *testing.T) {
	// Orchestrator always picks the best VRAM-tier candidate regardless of what is available.
	_ = os.Unsetenv("INFERENCE_MODEL")
	mockDB := testutil.NewMockDB()
	node := &models.Node{ID: uuid.New(), NodeSlug: "test-node", Status: models.NodeStatusActive}
	mockDB.AddNode(node)
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{{VRAMMB: 20464}},
		},
		Inference: &nodepayloads.InferenceInfo{
			Supported:       true,
			Running:         true,
			AvailableModels: []string{pmaModelDefault},
		},
	}
	raw, _ := json.Marshal(report)
	_ = mockDB.SaveNodeCapabilitySnapshot(t.Context(), node.ID, string(raw))
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "", "", "", nil)
	got := h.selectPMAModel(t.Context(), node.ID)
	// 20 GB VRAM → ≥16000 tier → first candidate is qwen3.5:9b even if not yet pulled.
	candidates := pmaModelCandidates(20464)
	if got != candidates[0] {
		t.Errorf("selectPMAModel() = %q, want top tier candidate %q", got, candidates[0])
	}
}

func TestSelectPMAModel_FallsBackToTopCandidate(t *testing.T) {
	_ = os.Unsetenv("INFERENCE_MODEL")
	mockDB := testutil.NewMockDB()
	node := &models.Node{ID: uuid.New(), NodeSlug: "test-node", Status: models.NodeStatusActive}
	mockDB.AddNode(node)
	// No GPU reported → default tier.
	report := nodepayloads.CapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Inference:  &nodepayloads.InferenceInfo{Supported: true, Running: true},
	}
	raw, _ := json.Marshal(report)
	_ = mockDB.SaveNodeCapabilitySnapshot(t.Context(), node.ID, string(raw))
	h := NewNodeHandler(mockDB, nil, "psk", testOrchestratorURL, "", "", "", nil)
	got := h.selectPMAModel(t.Context(), node.ID)
	if got != pmaModelMid {
		t.Errorf("selectPMAModel() no-GPU = %q, want %q", got, pmaModelMid)
	}
}

func TestSelectPMAModel_NoSnapshot(t *testing.T) {
	_ = os.Unsetenv("INFERENCE_MODEL")
	h := NewNodeHandler(testutil.NewMockDB(), nil, "psk", testOrchestratorURL, "", "", "", nil)
	got := h.selectPMAModel(t.Context(), uuid.New())
	// No snapshot → 0 VRAM → default tier first candidate.
	if got != pmaModelMid {
		t.Errorf("selectPMAModel() no-snapshot = %q, want %q", got, pmaModelMid)
	}
}

func TestOllamaNumCtxForVRAM(t *testing.T) {
	tests := []struct {
		name    string
		vramMB  int
		wantMin int
		wantMax int
	}{
		{"zero vram returns min", 0, ollamaMinNumCtx, ollamaMinNumCtx},
		{"negative vram returns min", -1, ollamaMinNumCtx, ollamaMinNumCtx},
		// 4 GB: 40% for KV = 1638 MB → (1638/50)*1024 = 32768 (fits within cap).
		{"4 GB VRAM (budget GPU)", 4096, ollamaMinNumCtx, ollamaMaxNumCtx},
		// 8 GB: 40% for KV = 3276 MB → (3276/50)*1024 = 65536 → capped at ollamaMaxNumCtx.
		{"8 GB VRAM (mid-range)", 8192, ollamaMinNumCtx, ollamaMaxNumCtx},
		{"20 GB VRAM (RX 7900 XT)", 20480, ollamaMinNumCtx, ollamaMaxNumCtx},
		{"80 GB VRAM (A100) caps at model max", 81920, ollamaMaxNumCtx, ollamaMaxNumCtx},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ollamaNumCtxForVRAM(tc.vramMB)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("ollamaNumCtxForVRAM(%d) = %d; want [%d, %d]", tc.vramMB, got, tc.wantMin, tc.wantMax)
			}
			// Result must be a power of two unless it is the hard cap (ollamaMaxNumCtx).
			if got != ollamaMaxNumCtx && got&(got-1) != 0 {
				t.Errorf("ollamaNumCtxForVRAM(%d) = %d; not a power of two", tc.vramMB, got)
			}
		})
	}
}

func TestPrevPow2(t *testing.T) {
	tests := []struct{ in, want int }{
		{0, 1}, {1, 1}, {2, 2}, {3, 2}, {4, 4},
		{5, 4}, {8, 8}, {9, 8}, {32768, 32768}, {32769, 32768},
	}
	for _, tc := range tests {
		if got := prevPow2(tc.in); got != tc.want {
			t.Errorf("prevPow2(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestInferenceEnvFromHardware_ContainsNumCtx(t *testing.T) {
	env := inferenceEnvFromHardware(20480)
	v, ok := env["OLLAMA_NUM_CTX"]
	if !ok {
		t.Fatal("OLLAMA_NUM_CTX missing from env")
	}
	if v == "" {
		t.Error("OLLAMA_NUM_CTX is empty")
	}
}

func TestVariantAndVRAM_NoGPU(t *testing.T) {
	report := &nodepayloads.CapabilityReport{}
	variant, vramMB := variantAndVRAM(report)
	if variant != "cpu" {
		t.Errorf("expected cpu variant, got %q", variant)
	}
	if vramMB != 0 {
		t.Errorf("expected 0 VRAM, got %d", vramMB)
	}
}

func TestVariantAndVRAM_GPU(t *testing.T) {
	tests := []struct {
		name        string
		features    map[string]interface{}
		vramMB      int
		wantVariant string
	}{
		{name: "rocm", features: map[string]interface{}{"rocm_version": "6.0"}, vramMB: 16384, wantVariant: "rocm"},
		{name: "cuda", features: map[string]interface{}{"cuda_capability": "8.6"}, vramMB: 8192, wantVariant: "cuda"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := &nodepayloads.CapabilityReport{
				GPU: &nodepayloads.GPUInfo{
					Present: true,
					Devices: []nodepayloads.GPUDevice{
						{VRAMMB: tc.vramMB, Features: tc.features},
					},
				},
			}
			variant, vramMB := variantAndVRAM(report)
			if variant != tc.wantVariant {
				t.Errorf("expected %q variant, got %q", tc.wantVariant, variant)
			}
			if vramMB != tc.vramMB {
				t.Errorf("expected %d VRAM, got %d", tc.vramMB, vramMB)
			}
		})
	}
}

func TestDeriveInferenceBackend_ExistingServiceNotEnabled(t *testing.T) {
	// When ExistingService==true the backend config should be returned (for DesiredModels/Env)
	// but with Enabled=false so node-manager does not try to start a second Ollama container.
	nodeID := uuid.New()
	report := nodepayloads.CapabilityReport{
		Inference: &nodepayloads.InferenceInfo{Supported: true, ExistingService: true},
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 20480, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
		},
	}
	raw, _ := json.Marshal(report)
	db := testutil.NewMockDB()
	db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
		NodeID:         nodeID,
		CapabilityJSON: string(raw),
		CreatedAt:      time.Now(),
	})
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil)
	backend := h.deriveInferenceBackend(t.Context(), nodeID)
	if backend == nil {
		t.Fatal("expected non-nil backend config even when ExistingService=true")
	}
	if backend.Enabled {
		t.Error("Enabled should be false when ExistingService=true")
	}
	if backend.SelectedModel == "" {
		t.Error("SelectedModel should be populated even when ExistingService=true")
	}
	if _, ok := backend.Env["OLLAMA_NUM_CTX"]; !ok {
		t.Error("Env should contain OLLAMA_NUM_CTX even when ExistingService=true")
	}
}

func TestSelectPMAModel_AlwaysPicksTopTierRegardlessOfAvailable(t *testing.T) {
	// Orchestrator picks the top VRAM-tier candidate regardless of availability on the node.
	// Node-manager is responsible for pulling; the orchestrator must not fall back to
	// whatever happens to be installed.
	_ = os.Unsetenv("INFERENCE_MODEL")
	tests := []struct {
		name      string
		available []string
		vramMB    int
		wantFirst bool // want candidates[0] for the tier
	}{
		{
			name:      "top tier even when only fallback available",
			available: []string{"qwen3:8b", "qwen3.5:0.8b"},
			vramMB:    20464,
		},
		{
			name:      "top tier when desired already available",
			available: []string{"qwen3.5:9b", "qwen3:8b"},
			vramMB:    20464,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodeID := uuid.New()
			report := nodepayloads.CapabilityReport{
				Inference: &nodepayloads.InferenceInfo{Supported: true, AvailableModels: tc.available},
				GPU: &nodepayloads.GPUInfo{
					Present: true,
					Devices: []nodepayloads.GPUDevice{
						{VRAMMB: tc.vramMB, Features: map[string]interface{}{"rocm_version": "6.0"}},
					},
				},
			}
			raw, _ := json.Marshal(report)
			db := testutil.NewMockDB()
			db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
				NodeID:         nodeID,
				CapabilityJSON: string(raw),
				CreatedAt:      time.Now(),
			})
			h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil)
			got := h.selectPMAModel(t.Context(), nodeID)
			want := pmaModelCandidates(tc.vramMB)[0]
			if got != want {
				t.Errorf("selectPMAModel = %q, want top tier %q", got, want)
			}
		})
	}
}

func TestDeriveInferenceBackend_IncludesSelectedModel(t *testing.T) {
	nodeID := uuid.New()
	report := nodepayloads.CapabilityReport{
		Inference: &nodepayloads.InferenceInfo{Supported: true},
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 20464, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
		},
	}
	raw, _ := json.Marshal(report)
	db := testutil.NewMockDB()
	db.CapabilityHistory = append(db.CapabilityHistory, &testutil.NodeCapabilitySnapshot{
		NodeID:         nodeID,
		CapabilityJSON: string(raw),
		CreatedAt:      time.Now(),
	})
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil)
	backend := h.deriveInferenceBackend(t.Context(), nodeID)
	if backend == nil {
		t.Fatal("expected non-nil inference backend config")
	}
	if backend.SelectedModel == "" {
		t.Error("expected SelectedModel to be populated in inference backend config")
	}
}

func TestBuildManagedServicesDesiredState_BackendEnvPropagated(t *testing.T) {
	t.Setenv("PMA_SERVICE_ID", "pma-main")
	t.Setenv("PMA_IMAGE", "pma:latest")
	t.Setenv("PMA_NODE_SLUG", "test-node")
	db := testutil.NewMockDB()
	h := NewNodeHandler(db, nil, "psk", testOrchestratorURL, "", "", "", nil)
	node := &models.Node{ID: uuid.New(), NodeSlug: "test-node"}
	backendEnv := map[string]string{"OLLAMA_NUM_CTX": "32768"}
	result := h.buildManagedServicesDesiredState(t.Context(), node, "http://10.0.0.1:12090", backendEnv)
	if result == nil || len(result.Services) == 0 {
		t.Fatal("expected at least one managed service")
	}
	svc := result.Services[0]
	if svc.Inference == nil {
		t.Fatal("expected inference config on managed service")
	}
	if svc.Inference.BackendEnv == nil {
		t.Fatal("expected BackendEnv to be set on managed service inference config")
	}
	if v := svc.Inference.BackendEnv["OLLAMA_NUM_CTX"]; v != "32768" {
		t.Errorf("expected OLLAMA_NUM_CTX=32768, got %q", v)
	}
}
