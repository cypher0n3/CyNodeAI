package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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

// pmaModelDefault is the minimum fallback model when no other candidate is available.
const pmaModelDefault = "qwen3.5:0.8b"

// pmaModelMid is a capable mid-tier model suitable for tool-calling tasks.
const pmaModelMid = "qwen3:8b"

const (
	ollamaVariantCPU  = "cpu"
	ollamaVariantROCm = "rocm"
	ollamaVariantCUDA = "cuda"
)

// NodeHandler handles node registration and management endpoints.
type NodeHandler struct {
	db                       database.Store
	jwt                      *auth.JWTManager
	registrationPSK          string
	orchestratorPublicURL    string
	workerAPIBearerToken     string
	workerAPITargetURL       string
	workerInternalAgentToken string
	logger                   *slog.Logger
}

// NewNodeHandler creates a new node handler.
func NewNodeHandler(db database.Store, jwt *auth.JWTManager, registrationPSK, orchestratorPublicURL, workerAPIBearerToken, workerAPITargetURL, workerInternalAgentToken string, logger *slog.Logger) *NodeHandler {
	return &NodeHandler{
		db:                       db,
		jwt:                      jwt,
		registrationPSK:          registrationPSK,
		orchestratorPublicURL:    orchestratorPublicURL,
		workerAPIBearerToken:     workerAPIBearerToken,
		workerAPITargetURL:       workerAPITargetURL,
		workerInternalAgentToken: workerInternalAgentToken,
		logger:                   logger,
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
	// Derive inference backend config (model candidates, env, Enabled flag).
	// Always populated when the node reports inference support so that desired models
	// can be pulled even when ExistingService==true (Enabled will be false in that case).
	var inferenceBackendEnv map[string]string
	if backend := h.deriveInferenceBackend(ctx, node.ID); backend != nil {
		payload.InferenceBackend = backend
		inferenceBackendEnv = backend.Env
	}
	payload.ManagedServices = h.buildManagedServicesDesiredState(ctx, node, workerAPITargetURL, inferenceBackendEnv)
	return payload
}

func (h *NodeHandler) buildManagedServicesDesiredState(ctx context.Context, node *models.Node, workerAPITargetURL string, inferenceBackendEnv map[string]string) *nodepayloads.ConfigManagedServices {
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
	// For node-local inference, derive the node host from the worker API dispatch URL.
	// If unavailable/loopback, leave base_url empty and let the worker resolve a local default.
	inferenceBaseURL := deriveNodeLocalInferenceBaseURL(workerAPITargetURL)
	defaultModel := h.selectPMAModel(ctx, node.ID)
	workerInternalAgentToken := strings.TrimSpace(h.workerInternalAgentToken)
	if workerInternalAgentToken == "" {
		// Keep dev/prototype managed-agent proxy functional even when a dedicated
		// internal agent token is not configured yet.
		workerInternalAgentToken = strings.TrimSpace(h.workerAPIBearerToken)
	}
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
					BackendEnv:   inferenceBackendEnv,
				},
				Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
					// Let the worker generate identity-bound per-service UDS endpoints.
					MCPGatewayProxyURL:    "auto",
					ReadyCallbackProxyURL: "auto",
					AgentToken:            workerInternalAgentToken,
				},
			},
		},
	}
}

func deriveNodeLocalInferenceBaseURL(workerAPITargetURL string) string {
	dispatchURL := strings.TrimSpace(workerAPITargetURL)
	if dispatchURL == "" {
		return ""
	}
	parsed, err := url.Parse(dispatchURL)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || strings.EqualFold(host, "localhost") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return ""
	}
	return "http://" + host + ":11434"
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

// selectPMAModel returns the orchestrator's chosen inference model for PMA on the given node.
// The orchestrator is the sole authority on model selection. It picks the best VRAM-tier
// candidate regardless of what the node currently has pulled — node-manager is responsible
// for pulling the selected model if it is not yet present.
//
//  1. If the INFERENCE_MODEL env var is set (operator pin), use it directly.
//  2. Otherwise pick the top VRAM-tier candidate for the node's hardware.
func (h *NodeHandler) selectPMAModel(ctx context.Context, nodeID uuid.UUID) string {
	if pinned := strings.TrimSpace(getEnvDefault("INFERENCE_MODEL", "")); pinned != "" {
		return pinned
	}
	candidates := pmaModelCandidates(h.vramTotalMBForNode(ctx, nodeID))
	if len(candidates) > 0 {
		return candidates[0]
	}
	return pmaModelDefault
}

// pmaModelCandidates returns the ordered candidate list for a given VRAM budget (MB).
// Mirrors the VRAM tiers in CYNAI.ORCHES.Operation.SelectProjectManagerModel step 5.
// Lists are best-first; all lists end with guaranteed-available defaults.
func pmaModelCandidates(vramMB int) []string {
	switch {
	case vramMB >= 24000:
		return []string{"qwen3.5:35b", "qwen2.5:32b", "qwen2.5:14b", "qwen3.5:9b", pmaModelMid, pmaModelDefault}
	case vramMB >= 16000:
		return []string{"qwen3.5:9b", pmaModelMid, "qwen2.5:14b", pmaModelDefault}
	case vramMB >= 8000:
		return []string{"qwen3.5:9b", pmaModelMid, pmaModelDefault}
	default:
		return []string{pmaModelMid, pmaModelDefault}
	}
}

// vramTotalMBForNode returns total VRAM across all GPU devices for the node from its capability snapshot.
func (h *NodeHandler) vramTotalMBForNode(ctx context.Context, nodeID uuid.UUID) int {
	snapJSON, err := h.db.GetLatestNodeCapabilitySnapshot(ctx, nodeID)
	if err != nil {
		return 0
	}
	var report nodepayloads.CapabilityReport
	if json.Unmarshal([]byte(snapJSON), &report) != nil || report.GPU == nil {
		return 0
	}
	total := 0
	for _, d := range report.GPU.Devices {
		total += d.VRAMMB
	}
	return total
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
	if json.Unmarshal([]byte(snapJSON), &report) != nil || report.Inference == nil || !report.Inference.Supported {
		return nil
	}
	variant, vramMB := variantAndVRAM(&report)
	return &nodepayloads.ConfigInferenceBackend{
		// When ExistingService is true the node already has Ollama running; set Enabled=false
		// so node-manager does not try to start a second container. SelectedModel and Env are
		// still populated so node-manager can pull the chosen model and apply context settings.
		Enabled:       !report.Inference.ExistingService,
		Image:         "",
		Variant:       variant,
		Port:          11434,
		Env:           inferenceEnvFromHardware(vramMB),
		SelectedModel: h.selectPMAModel(ctx, nodeID),
	}
}

// variantAndVRAM extracts the Ollama variant string and primary GPU VRAM (MB) from a
// capability report. Returns "cpu" and 0 when no GPU is detected.
func variantAndVRAM(report *nodepayloads.CapabilityReport) (variant string, vramMB int) {
	variant = ollamaVariantCPU
	if report.GPU == nil || !report.GPU.Present || len(report.GPU.Devices) == 0 {
		return
	}
	d := report.GPU.Devices[0]
	vramMB = d.VRAMMB
	if d.Features != nil {
		if _, hasROCm := d.Features["rocm_version"]; hasROCm {
			variant = ollamaVariantROCm
		} else if _, hasCUDA := d.Features["cuda_capability"]; hasCUDA {
			variant = ollamaVariantCUDA
		}
	}
	return
}

// inferenceEnvFromHardware derives Ollama environment variables based on reported
// hardware. OLLAMA_CONTEXT_LENGTH is scaled to the available VRAM — we reserve ~40% of
// VRAM for model weights (the remainder is available for the KV cache) and target
// the largest power-of-two context length that fits.
//
// Approximate KV cache cost for qwen3:8b (8 B params, GQA):
//
//	4 096 tokens  ≈  200 MB
//	16 384 tokens ≈  800 MB
//	32 768 tokens ≈ 1 600 MB
//	40 960 tokens ≈ 2 000 MB  (model max)
//
// When vramMB is 0 (CPU-only node) we fall back to a conservative 8 192 token window.
func inferenceEnvFromHardware(vramMB int) map[string]string {
	numCtx := ollamaNumCtxForVRAM(vramMB)
	return map[string]string{
		// OLLAMA_CONTEXT_LENGTH is the Ollama server env var for default context length.
		// OLLAMA_NUM_CTX is the per-request option key, also set here for compatibility.
		"OLLAMA_CONTEXT_LENGTH": fmt.Sprintf("%d", numCtx),
		"OLLAMA_NUM_CTX":        fmt.Sprintf("%d", numCtx),
	}
}

// ollamaNumCtxForVRAM returns an OLLAMA_NUM_CTX value sized to vramMB.
// We assume ~60 % of VRAM is already consumed by model weights and reserve 40 %
// for the KV cache (~50 MB per 1 024 context tokens for an 8 B GQA model).
const (
	ollamaKVCacheMBPer1KTokens = 50  // empirical for 8 B GQA models
	ollamaWeightVRAMFraction   = 0.6 // fraction of VRAM taken by weights
	ollamaMinNumCtx            = 8192
	ollamaMaxNumCtx            = 40960
)

func ollamaNumCtxForVRAM(vramMB int) int {
	if vramMB <= 0 {
		return ollamaMinNumCtx
	}
	availableForKV := int(float64(vramMB) * (1 - ollamaWeightVRAMFraction))
	tokens := (availableForKV / ollamaKVCacheMBPer1KTokens) * 1024
	// Round down to nearest power of two for cache efficiency.
	tokens = prevPow2(tokens)
	if tokens < ollamaMinNumCtx {
		return ollamaMinNumCtx
	}
	if tokens > ollamaMaxNumCtx {
		return ollamaMaxNumCtx
	}
	return tokens
}

// prevPow2 returns the largest power of two ≤ n, or 1 if n ≤ 0.
func prevPow2(n int) int {
	if n <= 0 {
		return 1
	}
	p := 1
	for p*2 <= n {
		p *= 2
	}
	return p
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
