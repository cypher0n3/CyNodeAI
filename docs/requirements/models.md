# MODELS Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `MODELS` domain.
It covers model lifecycle requirements, routing, and model management behavior.

## 2 Requirements

- REQ-MODELS-0001: Model lifecycle, registration, and selection.
  [CYNAI.MODELS.Doc.ModelManagement](../tech_specs/model_management.md#spec-cynai-models-doc-modelmanagement)
  <a id="req-models-0001"></a>
- REQ-MODELS-0002: External model calls routed via API Egress; no provider keys in agents or sandboxes.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0002"></a>
- REQ-MODELS-0003: MVP Phase 1 requires at least one inference-capable path (local inference on at least one node, or external provider routing with a configured key).
  In the single-node case, startup must fail fast (or refuse to enter a ready state) if no local inference is available and no external provider key is configured.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md)
  [node.md](../tech_specs/node.md)
  <a id="req-models-0003"></a>

- REQ-MODELS-0100: Worker nodes SHOULD load models from the orchestrator cache instead of downloading from external sources.
  [CYNAI.MODELS.ModelCache](../tech_specs/model_management.md#spec-cynai-models-modelcache)
  <a id="req-models-0100"></a>
- REQ-MODELS-0101: The cache SHOULD be content-addressed by a strong hash (e.g. sha256).
  [CYNAI.MODELS.ModelCache](../tech_specs/model_management.md#spec-cynai-models-modelcache)
  <a id="req-models-0101"></a>
- REQ-MODELS-0102: The cache MUST record artifact size and integrity metadata.
  [CYNAI.MODELS.ModelCache](../tech_specs/model_management.md#spec-cynai-models-modelcache)
  <a id="req-models-0102"></a>
- REQ-MODELS-0103: The cache MAY store multiple formats for the same logical model.
  [CYNAI.MODELS.ModelCache](../tech_specs/model_management.md#spec-cynai-models-modelcache)
  <a id="req-models-0103"></a>
- REQ-MODELS-0104: Nodes SHOULD retrieve model artifacts from the orchestrator cache using local network paths.
  [CYNAI.MODELS.NodeLoadWorkflow](../tech_specs/model_management.md#spec-cynai-models-nodeloadworkflow)
  <a id="req-models-0104"></a>
- REQ-MODELS-0105: When selecting a node for a task, the Project Manager Agent SHOULD prefer nodes where the required model is already loaded.
  [CYNAI.MODELS.NodeLoadWorkflow](../tech_specs/model_management.md#spec-cynai-models-nodeloadworkflow)
  <a id="req-models-0105"></a>
- REQ-MODELS-0106: The orchestrator MUST support user-directed actions to populate the cache.
  [CYNAI.MODELS.UserDirectedDownloads](../tech_specs/model_management.md#spec-cynai-models-userdirecteddownloads)
  <a id="req-models-0106"></a>
- REQ-MODELS-0107: Model management actions MUST be policy-controlled and audited.
  [CYNAI.MODELS.UserDirectedDownloads](../tech_specs/model_management.md#spec-cynai-models-userdirecteddownloads)
  <a id="req-models-0107"></a>
- REQ-MODELS-0108: Model management SHOULD be configurable via PostgreSQL preferences.
  [CYNAI.MODELS.PreferencesConstraints](../tech_specs/model_management.md#spec-cynai-models-preferencesconstraints)
  <a id="req-models-0108"></a>
- REQ-MODELS-0109: The orchestrator SHOULD record all model downloads, imports, and evictions.
  [CYNAI.MODELS.AuditingSafety](../tech_specs/model_management.md#spec-cynai-models-auditingsafety)
  <a id="req-models-0109"></a>
- REQ-MODELS-0110: The orchestrator SHOULD verify model artifact integrity using `sha256` before exposing artifacts to nodes.
  [CYNAI.MODELS.AuditingSafety](../tech_specs/model_management.md#spec-cynai-models-auditingsafety)
  <a id="req-models-0110"></a>
- REQ-MODELS-0111: Nodes SHOULD be configured to avoid downloading models directly from the public internet.
  [CYNAI.MODELS.AuditingSafety](../tech_specs/model_management.md#spec-cynai-models-auditingsafety)
  <a id="req-models-0111"></a>
- REQ-MODELS-0112: The orchestrator SHOULD prefer local execution when it can meet required capabilities and latency objectives.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0112"></a>
- REQ-MODELS-0113: The orchestrator MUST be able to route to configured external AI APIs when needed and when policy allows it.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0113"></a>
- REQ-MODELS-0114: If no worker can provide local inference and no external provider is configured (via API Egress), the system MUST fail fast (or refuse to enter a ready state).
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0114"></a>
- REQ-MODELS-0115: The orchestrator MUST attempt local execution when a worker can satisfy capability requirements and is not overloaded.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0115"></a>
- REQ-MODELS-0116: The orchestrator SHOULD honor a user override selecting a specific external provider when policy allows it.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0116"></a>
- REQ-MODELS-0117: The orchestrator MUST deny external routing when policy does not allow it.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0117"></a>
- REQ-MODELS-0118: The orchestrator SHOULD record the routing decision and the primary reasons for it.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0118"></a>
- REQ-MODELS-0119: External model calls MUST be performed through the API Egress Server so credentials are not exposed to agents or sandbox containers.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0119"></a>
- REQ-MODELS-0120: Sandboxes MUST NOT receive provider API keys.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0120"></a>
- REQ-MODELS-0121: Sandboxes SHOULD access external capabilities only through orchestrator-mediated MCP tools.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0121"></a>
- REQ-MODELS-0122: Routing behavior SHOULD be configurable via PostgreSQL preferences.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0122"></a>
- REQ-MODELS-0123: Orchestrator-side agents MAY use separate preferences for external provider routing.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0123"></a>
- REQ-MODELS-0124: The API Egress Server MUST log each outbound call with task context and subject identity.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0124"></a>
- REQ-MODELS-0125: Responses SHOULD be filtered for secret leakage and stored with least privilege.
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-models-0125"></a>
