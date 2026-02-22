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
	db                    database.Store
	jwt                   *auth.JWTManager
	registrationPSK       string
	orchestratorPublicURL string
	workerAPIBearerToken  string
	workerAPITargetURL    string
	logger                *slog.Logger
}

// NewNodeHandler creates a new node handler.
func NewNodeHandler(db database.Store, jwt *auth.JWTManager, registrationPSK, orchestratorPublicURL, workerAPIBearerToken, workerAPITargetURL string, logger *slog.Logger) *NodeHandler {
	return &NodeHandler{
		db:                    db,
		jwt:                   jwt,
		registrationPSK:       registrationPSK,
		orchestratorPublicURL: orchestratorPublicURL,
		workerAPIBearerToken:  workerAPIBearerToken,
		workerAPITargetURL:    workerAPITargetURL,
		logger:                logger,
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

// NodeCapabilityReport represents the capability report from a node.
// See docs/tech_specs/worker_node_payloads.md for full schema.
type NodeCapabilityReport struct {
	Version    int                    `json:"version"`
	ReportedAt string                 `json:"reported_at"`
	Node       NodeCapabilityNode     `json:"node"`
	Platform   NodeCapabilityPlatform `json:"platform"`
	Compute    NodeCapabilityCompute  `json:"compute"`
	Sandbox    *NodeCapabilitySandbox `json:"sandbox,omitempty"`
}

// NodeCapabilityNode contains node identity info.
type NodeCapabilityNode struct {
	NodeSlug string   `json:"node_slug"`
	Name     string   `json:"name,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

// NodeCapabilityPlatform contains platform info.
type NodeCapabilityPlatform struct {
	OS     string `json:"os"`
	Distro string `json:"distro,omitempty"`
	Arch   string `json:"arch"`
}

// NodeCapabilityCompute contains compute resources info.
type NodeCapabilityCompute struct {
	CPUCores int `json:"cpu_cores"`
	RAMMB    int `json:"ram_mb"`
}

// NodeCapabilitySandbox contains sandbox capability info.
type NodeCapabilitySandbox struct {
	Supported      bool     `json:"supported"`
	Features       []string `json:"features,omitempty"`
	MaxConcurrency int      `json:"max_concurrency,omitempty"`
}

// NodeRegistrationRequest represents the registration request with PSK.
type NodeRegistrationRequest struct {
	PSK        string               `json:"psk"`
	Capability NodeCapabilityReport `json:"capability"`
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
		h.handleExistingNodeRegistration(ctx, w, existingNode)
		return
	}

	h.handleNewNodeRegistration(ctx, w, req)
}

func (h *NodeHandler) validateRegistrationRequest(w http.ResponseWriter, r *http.Request) (*NodeRegistrationRequest, bool) {
	var req NodeRegistrationRequest
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

func (h *NodeHandler) handleExistingNodeRegistration(ctx context.Context, w http.ResponseWriter, node *models.Node) {
	if err := h.db.UpdateNodeStatus(ctx, node.ID, "active"); err != nil {
		h.logError("update node status", "error", err)
		WriteInternalError(w, "Failed to register node")
		return
	}

	nodeJWT, expiresAt, err := h.jwt.GenerateNodeToken(node.ID, node.NodeSlug)
	if err != nil {
		h.logError("generate node JWT", "error", err)
		WriteInternalError(w, "Failed to generate token")
		return
	}

	WriteJSON(w, http.StatusOK, h.buildBootstrapResponse(h.orchestratorPublicURL, nodeJWT, expiresAt))
}

func (h *NodeHandler) handleNewNodeRegistration(ctx context.Context, w http.ResponseWriter, req *NodeRegistrationRequest) {
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

func (h *NodeHandler) initializeNewNode(ctx context.Context, nodeID uuid.UUID, capability *NodeCapabilityReport) {
	if err := h.db.UpdateNodeStatus(ctx, nodeID, "active"); err != nil {
		h.logError("update node status", "error", err)
	}

	if err := h.db.UpdateNodeConfigVersion(ctx, nodeID, "1"); err != nil {
		h.logError("set initial config version", "error", err)
	}

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
	workerAPITargetURL := h.workerAPITargetURL
	if node.WorkerAPITargetURL != nil && *node.WorkerAPITargetURL != "" {
		workerAPITargetURL = *node.WorkerAPITargetURL
	}

	payload := h.buildNodeConfigPayload(node, configVersion, workerAPITargetURL)

	if workerAPITargetURL != "" && h.workerAPIBearerToken != "" {
		if err := h.db.UpdateNodeWorkerAPIConfig(ctx, node.ID, workerAPITargetURL, h.workerAPIBearerToken); err != nil {
			h.logError("update node worker api config", "error", err)
		}
	}

	WriteJSON(w, http.StatusOK, payload)
}

func (h *NodeHandler) buildNodeConfigPayload(node *models.Node, configVersion, workerAPITargetURL string) nodepayloads.NodeConfigurationPayload {
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
		SandboxRegistry: nodepayloads.ConfigSandboxRegistry{RegistryURL: ""},
		ModelCache:      nodepayloads.ConfigModelCache{CacheURL: ""},
	}
	if h.workerAPIBearerToken != "" {
		payload.WorkerAPI = &nodepayloads.ConfigWorkerAPI{
			OrchestratorBearerToken: h.workerAPIBearerToken,
		}
	}
	return payload
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

	var report NodeCapabilityReport
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

	if err := h.db.UpdateNodeLastSeen(ctx, *nodeID); err != nil {
		h.logError("update last seen", "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
