package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// NodeHandler handles node registration and management endpoints.
type NodeHandler struct {
	db                        database.Store
	jwt                       *auth.JWTManager
	registrationPSK           string
	orchestratorPublicURL      string
	workerAPIBearerToken       string
	workerAPITargetURL         string
	workerInternalAgentToken   string
	logger                     *slog.Logger
}

// NewNodeHandler creates a new node handler.
func NewNodeHandler(db database.Store, jwt *auth.JWTManager, registrationPSK, orchestratorPublicURL, workerAPIBearerToken, workerAPITargetURL, workerInternalAgentToken string, logger *slog.Logger) *NodeHandler {
	return &NodeHandler{
		db:                      db,
		jwt:                     jwt,
		registrationPSK:         registrationPSK,
		orchestratorPublicURL:   orchestratorPublicURL,
		workerAPIBearerToken:    workerAPIBearerToken,
		workerAPITargetURL:      workerAPITargetURL,
		workerInternalAgentToken: workerInternalAgentToken,
		logger:                  logger,
	}
}

func (h *NodeHandler) logError(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Error(msg, args...)
	}
}

func (h *NodeHandler) logWarn(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Warn(msg, args...)
	}
}

func (h *NodeHandler) logInfo(msg string, args ...any) {
	if h.logger != nil {
		h.logger.Info(msg, args...)
	}
}

// Register handles POST /v1/nodes/register.
func (h *NodeHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, ok := h.validateRegistrationRequest(w, r)
	if !ok {
		return
	}

	existingNode, err := h.db.GetNodeBySlug(ctx, req.Capability.Node.NodeSlug)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		h.logError("get node by slug", "error", err)
		WriteInternalError(w, "Failed to register node")
		return
	}

	if existingNode != nil {
		h.handleExistingNodeRegistration(ctx, w, existingNode, req)
		return
	}
	h.handleNewNodeRegistration(ctx, w, req)
}

func (h *NodeHandler) validateRegistrationRequest(w http.ResponseWriter, r *http.Request) (*nodepayloads.RegistrationRequest, bool) {
	var req nodepayloads.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return nil, false
	}

	if req.PSK != h.registrationPSK {
		h.logWarn("invalid registration PSK", "node_slug", req.Capability.Node.NodeSlug)
		WriteUnauthorized(w, "Invalid registration PSK")
		return nil, false
	}

	if req.Capability.Version != 1 {
		WriteBadRequest(w, "Unsupported capability version")
		return nil, false
	}

	if req.Capability.Node.NodeSlug == "" {
		WriteBadRequest(w, "Node slug is required")
		return nil, false
	}

	return &req, true
}

func (h *NodeHandler) handleExistingNodeRegistration(ctx context.Context, w http.ResponseWriter, node *models.Node, req *nodepayloads.RegistrationRequest) {
	if err := h.db.UpdateNodeStatus(ctx, node.ID, "active"); err != nil {
		h.logError("update node status", "error", err)
		WriteInternalError(w, "Failed to register node")
		return
	}

	h.saveCapabilitySnapshotAndHash(ctx, node.ID, &req.Capability)
	h.applyWorkerAPIURLFromCapability(ctx, node.ID, &req.Capability)

	nodeJWT, expiresAt, err := h.jwt.GenerateNodeToken(node.ID, node.NodeSlug)
	if err != nil {
		h.logError("generate node JWT", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	WriteJSON(w, http.StatusOK, h.buildBootstrapResponse(h.orchestratorPublicURL, nodeJWT, expiresAt))
}

func (h *NodeHandler) handleNewNodeRegistration(ctx context.Context, w http.ResponseWriter, req *nodepayloads.RegistrationRequest) {
	newNode, err := h.db.CreateNode(ctx, req.Capability.Node.NodeSlug)
	if err != nil {
		h.logError("create node", "error", err)
		WriteInternalError(w, "Failed to register node")
		return
	}

	h.initializeNewNode(ctx, newNode.ID, &req.Capability)

	nodeJWT, expiresAt, err := h.jwt.GenerateNodeToken(newNode.ID, newNode.NodeSlug)
	if err != nil {
		h.logError("generate node JWT", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	h.logInfo("node registered", "node_id", newNode.ID, "node_slug", newNode.NodeSlug)
	WriteJSON(w, http.StatusCreated, h.buildBootstrapResponse(h.orchestratorPublicURL, nodeJWT, expiresAt))
}

func (h *NodeHandler) initializeNewNode(ctx context.Context, nodeID uuid.UUID, capability *nodepayloads.CapabilityReport) {
	if err := h.db.UpdateNodeStatus(ctx, nodeID, "active"); err != nil {
		h.logError("update node status", "error", err)
	}

	if err := h.db.UpdateNodeConfigVersion(ctx, nodeID, "1"); err != nil {
		h.logError("set initial config version", "error", err)
	}

	h.saveCapabilitySnapshotAndHash(ctx, nodeID, capability)
	h.applyWorkerAPIURLFromCapability(ctx, nodeID, capability)
}

// saveCapabilitySnapshotAndHash persists the capability snapshot and updates the node's capability hash.
func (h *NodeHandler) saveCapabilitySnapshotAndHash(ctx context.Context, nodeID uuid.UUID, capability *nodepayloads.CapabilityReport) {
	capJSON, _ := json.Marshal(capability)
	if err := h.db.SaveNodeCapabilitySnapshot(ctx, nodeID, string(capJSON)); err != nil {
		h.logError("save capability snapshot", "error", err)
	}
	hashBytes := sha256.Sum256(capJSON)
	capHash := "sha256:" + hex.EncodeToString(hashBytes[:])
	if err := h.db.UpdateNodeCapability(ctx, nodeID, capHash); err != nil {
		h.logError("update capability hash", "error", err)
	}
}

// applyWorkerAPIURLFromCapability sets the node's worker_api_target_url from capability.worker_api.base_url
// unless an explicit override (h.workerAPITargetURL) is configured.
func (h *NodeHandler) applyWorkerAPIURLFromCapability(ctx context.Context, nodeID uuid.UUID, capability *nodepayloads.CapabilityReport) {
	workerURL := ""
	if h.workerAPITargetURL != "" {
		workerURL = h.workerAPITargetURL
	} else if capability != nil && capability.WorkerAPI != nil && capability.WorkerAPI.BaseURL != "" {
		workerURL = strings.TrimSpace(capability.WorkerAPI.BaseURL)
	}
	if workerURL != "" && h.workerAPIBearerToken != "" {
		if err := h.db.UpdateNodeWorkerAPIConfig(ctx, nodeID, workerURL, h.workerAPIBearerToken); err != nil {
			h.logError("update node worker api config", "error", err)
		}
	}
}

func (h *NodeHandler) buildBootstrapResponse(baseURL, nodeJWT string, expiresAt time.Time) nodepayloads.BootstrapResponse {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return nodepayloads.BootstrapResponse{
		Version:  1,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
		Orchestrator: nodepayloads.BootstrapOrchestrator{
			BaseURL: baseURL,
			Endpoints: nodepayloads.BootstrapEndpoints{
				WorkerRegistrationURL: baseURL + "/v1/nodes/register",
				NodeReportURL:         baseURL + "/v1/nodes/capability",
				NodeConfigURL:         baseURL + "/v1/nodes/config",
			},
		},
		Auth: nodepayloads.BootstrapAuth{
			NodeJWT:   nodeJWT,
			ExpiresAt: expiresAt.Format(time.RFC3339),
		},
	}
}

// resolveConfigVersion returns the node's config version or a new ULID, persisting when new.
// config_version in node_configuration_payload_v1 per worker_node_payloads.md (CYNAI.WORKER.Payload.ConfigurationV1).
func (h *NodeHandler) resolveConfigVersion(ctx context.Context, node *models.Node) string {
	if node.ConfigVersion != nil && *node.ConfigVersion != "" {
		return *node.ConfigVersion
	}
	configVersion := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	if err := h.db.UpdateNodeConfigVersion(ctx, node.ID, configVersion); err != nil {
		h.logError("set config version", "error", err)
	}
	return configVersion
}

// GetConfig handles GET /v1/nodes/config. Returns node_configuration_payload_v1.
func (h *NodeHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeID := getNodeIDFromContext(ctx)
	if nodeID == nil {
		WriteUnauthorized(w, "Node authentication required")
		return
	}

	node, err := h.db.GetNodeByID(ctx, *nodeID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "Node not found")
			return
		}
		h.logError("get node", "error", err)
		WriteInternalError(w, "Failed to get node config")
		return
	}

	configVersion := h.resolveConfigVersion(ctx, node)
	// Explicit override (e.g. WORKER_API_TARGET_URL for same-host) wins; otherwise use node's stored URL (from registration/capability).
	workerAPITargetURL := h.workerAPITargetURL
	if workerAPITargetURL == "" && node.WorkerAPITargetURL != nil && *node.WorkerAPITargetURL != "" {
		workerAPITargetURL = *node.WorkerAPITargetURL
	}

	payload := h.buildNodeConfigPayload(ctx, node, configVersion, workerAPITargetURL)

	managedCount := 0
	if payload.ManagedServices != nil {
		managedCount = len(payload.ManagedServices.Services)
	}
	h.logInfo("node config built",
		"node_slug", node.NodeSlug,
		"config_version", configVersion,
		"inference_backend", payload.InferenceBackend != nil,
		"managed_services_count", managedCount)

	if workerAPITargetURL != "" && h.workerAPIBearerToken != "" {
		if err := h.db.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerAPITargetURL, h.workerAPIBearerToken); err != nil {
			h.logError("update node worker api config", "error", err)
		}
	}

	WriteJSON(w, http.StatusOK, payload)
}

func (h *NodeHandler) buildNodeConfigPayload(ctx context.Context, node *models.Node, configVersion, workerAPITargetURL string) nodepayloads.NodeConfigurationPayload {
	baseURL := strings.TrimSuffix(h.orchestratorPublicURL, "/")
	payload := nodepayloads.NodeConfigurationPayload{
		Version:       1,
		ConfigVersion: configVersion,
		IssuedAt:      time.Now().UTC().Format(time.RFC3339),
		NodeSlug:      node.NodeSlug,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: baseURL,
			Endpoints: nodepayloads.ConfigEndpoints{
				WorkerAPITargetURL: workerAPITargetURL,
				NodeReportURL:      baseURL + "/v1/nodes/capability",
			},
		},
		SandboxRegistries: nil, // empty: node uses default Docker Hub per worker_node_payloads.md
		ModelCache:        nodepayloads.ConfigModelCache{CacheURL: ""},
	}
	if h.workerAPIBearerToken != "" {
		payload.WorkerAPI = &nodepayloads.ConfigWorkerAPI{
			OrchestratorBearerToken: h.workerAPIBearerToken,
		}
	}
	// When node is inference-capable and does not report existing_service, instruct it to start the backend.
	if backend := h.deriveInferenceBackend(ctx, node.ID); backend != nil {
		payload.InferenceBackend = backend
	}
	payload.ManagedServices = h.buildManagedServicesDesiredState(ctx, node)
	return payload
}

func (h *NodeHandler) buildManagedServicesDesiredState(ctx context.Context, node *models.Node) *nodepayloads.ConfigManagedServices {
	if h.db == nil || node == nil {
		if h.logger != nil {
			h.logger.Debug("managed services skipped", "reason", "db_or_node_nil")
		}
		return nil
	}
	serviceID := strings.TrimSpace(getEnvDefault("PMA_SERVICE_ID", "pma-main"))
	image := strings.TrimSpace(getEnvDefault("PMA_IMAGE", "ghcr.io/cypher0n3/cynode-pma:latest"))
	if serviceID == "" || image == "" {
		if h.logger != nil {
			h.logger.Debug("managed services skipped", "reason", "pma_service_id_or_image_empty", "node_slug", node.NodeSlug)
		}
		return nil
	}
	selectedNodeSlug := h.selectPMAHostNodeSlug(ctx, node.NodeSlug)
	if selectedNodeSlug == "" || selectedNodeSlug != node.NodeSlug {
		if h.logger != nil {
			h.logger.Info("managed services skipped for node",
				"node_slug", node.NodeSlug,
				"selected_pma_host", selectedNodeSlug,
				"reason", "pma_host_is_other_node")
		}
		return nil
	}
	// URL for PMA container (node-run) to reach Ollama. Use NODE_PMA_OLLAMA_BASE_URL when set
	// (e.g. http://host.containers.internal:11434 so node's PMA container can reach host-mapped Ollama).
	inferenceBaseURL := strings.TrimSpace(getEnvDefault("NODE_PMA_OLLAMA_BASE_URL", getEnvDefault("OLLAMA_BASE_URL", getEnvDefault("INFERENCE_URL", "http://127.0.0.1:11434"))))
	defaultModel := strings.TrimSpace(getEnvDefault("INFERENCE_MODEL", "tinyllama"))
	if h.logger != nil {
		h.logger.Info("managed services desired state built",
			"node_slug", node.NodeSlug,
			"pma_service_id", serviceID,
			"pma_image", image,
			"count", 1)
	}
	return &nodepayloads.ConfigManagedServices{
		Services: []nodepayloads.ConfigManagedService{
			{
				ServiceID:   serviceID,
				ServiceType: "pma",
				Image:       image,
				Args:        []string{"--role=project_manager"},
				Healthcheck: &nodepayloads.ConfigManagedServiceHealthcheck{
					Path:           "/healthz",
					ExpectedStatus: http.StatusOK,
				},
				RestartPolicy: "always",
				Role:          "project_manager",
				Inference: &nodepayloads.ConfigManagedServiceInference{
					Mode:         "node_local",
					BaseURL:      inferenceBaseURL,
					DefaultModel: defaultModel,
				},
				Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
					MCPGatewayProxyURL:    "http://127.0.0.1:12090/v1/worker/internal/orchestrator/mcp:call",
					ReadyCallbackProxyURL: "http://127.0.0.1:12090/v1/worker/internal/orchestrator/agent:ready",
					AgentToken:            strings.TrimSpace(h.workerInternalAgentToken),
				},
			},
		},
	}
}

func (h *NodeHandler) selectPMAHostNodeSlug(ctx context.Context, fallbackNodeSlug string) string {
	explicit := strings.TrimSpace(getEnvDefault("PMA_HOST_NODE_SLUG", ""))
	if explicit != "" {
		return explicit
	}
	nodes, err := h.db.ListActiveNodes(ctx)
	if err != nil || len(nodes) == 0 {
		return fallbackNodeSlug
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeSlug < nodes[j].NodeSlug })
	preferLabel := strings.TrimSpace(getEnvDefault("PMA_PREFER_HOST_LABEL", "orchestrator_host"))
	if preferLabel != "" {
		for _, n := range nodes {
			if h.nodeHasLabel(ctx, n.ID, preferLabel) {
				return n.NodeSlug
			}
		}
	}
	return nodes[0].NodeSlug
}

func (h *NodeHandler) nodeHasLabel(ctx context.Context, nodeID uuid.UUID, label string) bool {
	snapJSON, err := h.db.GetLatestNodeCapabilitySnapshot(ctx, nodeID)
	if err != nil {
		return false
	}
	var report nodepayloads.CapabilityReport
	if err := json.Unmarshal([]byte(snapJSON), &report); err != nil {
		return false
	}
	for _, got := range report.Node.Labels {
		if got == label {
			return true
		}
	}
	return false
}

func getEnvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func boolEnvDefault(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (h *NodeHandler) deriveInferenceBackend(ctx context.Context, nodeID uuid.UUID) *nodepayloads.ConfigInferenceBackend {
	snapJSON, err := h.db.GetLatestNodeCapabilitySnapshot(ctx, nodeID)
	if err != nil {
		return nil
	}
	var report nodepayloads.CapabilityReport
	if json.Unmarshal([]byte(snapJSON), &report) != nil || report.Inference == nil || !report.Inference.Supported || report.Inference.ExistingService {
		return nil
	}
	variant := "cpu"
	if report.GPU != nil && report.GPU.Present && len(report.GPU.Devices) > 0 {
		if d := report.GPU.Devices[0]; d.Features != nil {
			if _, hasROCm := d.Features["rocm_version"]; hasROCm {
				variant = "rocm"
			} else if _, hasCUDA := d.Features["cuda_capability"]; hasCUDA {
				variant = "cuda"
			}
		}
	}
	return &nodepayloads.ConfigInferenceBackend{
		Enabled: true,
		Image:   "",
		Variant: variant,
		Port:    11434,
	}
}

// ConfigAck handles POST /v1/nodes/config. Accepts node_config_ack_v1 and records the acknowledgement.
func (h *NodeHandler) ConfigAck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeID := getNodeIDFromContext(ctx)
	if nodeID == nil {
		WriteUnauthorized(w, "Node authentication required")
		return
	}

	node, err := h.db.GetNodeByID(ctx, *nodeID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			WriteNotFound(w, "Node not found")
			return
		}
		h.logError("get node", "error", err)
		WriteInternalError(w, "Failed to record config ack")
		return
	}

	var ack nodepayloads.ConfigAck
	if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return
	}

	if ack.Version != 1 {
		WriteBadRequest(w, "Unsupported config ack version")
		return
	}
	if ack.NodeSlug != node.NodeSlug {
		WriteBadRequest(w, "node_slug does not match authenticated node")
		return
	}
	if ack.Status != "applied" && ack.Status != "failed" {
		WriteBadRequest(w, "status must be applied or failed")
		return
	}

	ackAt := time.Now().UTC()
	if ack.AckAt != "" {
		if t, err := time.Parse(time.RFC3339, ack.AckAt); err == nil {
			ackAt = t.UTC()
		}
	}

	var errMsg *string
	if ack.Error != nil && ack.Error.Message != "" {
		errMsg = &ack.Error.Message
	}

	if err := h.db.UpdateNodeConfigAck(ctx, node.ID, ack.ConfigVersion, ack.Status, ackAt, errMsg); err != nil {
		h.logError("update config ack", "error", err)
		WriteInternalError(w, "Failed to record config ack")
		return
	}

	h.logInfo("config ack recorded", "node_slug", node.NodeSlug, "config_version", ack.ConfigVersion, "status", ack.Status)
	w.WriteHeader(http.StatusNoContent)
}

// ReportCapability handles POST /v1/nodes/capability.
func (h *NodeHandler) ReportCapability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeID := getNodeIDFromContext(ctx)

	if nodeID == nil {
		WriteUnauthorized(w, "Node authentication required")
		return
	}

	var report nodepayloads.CapabilityReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		WriteBadRequest(w, "Invalid request body")
		return
	}

	capJSON, _ := json.Marshal(report)
	if err := h.db.SaveNodeCapabilitySnapshot(ctx, *nodeID, string(capJSON)); err != nil {
		h.logError("save capability snapshot", "error", err)
		WriteInternalError(w, "Failed to save capability")
		return
	}

	hashBytes := sha256.Sum256(capJSON)
	capHash := "sha256:" + hex.EncodeToString(hashBytes[:])
	if err := h.db.UpdateNodeCapability(ctx, *nodeID, capHash); err != nil {
		h.logError("update capability hash", "error", err)
		WriteInternalError(w, "Failed to update capability")
		return
	}

	h.applyWorkerAPIURLFromCapability(ctx, *nodeID, &report)

	if err := h.db.UpdateNodeLastSeen(ctx, *nodeID); err != nil {
		h.logError("update last seen", "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
