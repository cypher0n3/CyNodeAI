# Local Inference Backend Alternatives: Proposed Spec

- [1. Purpose, Status, and Current State](#1-purpose-status-and-current-state)
  - [1.1. Current State](#11-current-state)
- [2. Abstraction: Local Inference Backend](#2-abstraction-local-inference-backend)
- [3. Alternatives to Ollama](#3-alternatives-to-ollama)
  - [3.1 Recommended Backends (Rank-Ordered by Value)](#31-recommended-backends-rank-ordered-by-value)
- [4. How Alternatives Could Be Used](#4-how-alternatives-could-be-used)
  - [4.1 Node-Local Inference (Worker Node)](#41-node-local-inference-worker-node)
  - [4.2 Orchestrator / Dev Stack](#42-orchestrator--dev-stack)
  - [4.3 API Egress / Sanity Checker](#43-api-egress--sanity-checker)
  - [4.4 Ports and Endpoints](#44-ports-and-endpoints)
- [5. Specific Requirement Updates and Additions](#5-specific-requirement-updates-and-additions)
  - [5.1 Edits to Existing Requirements](#51-edits-to-existing-requirements)
  - [5.2 New Requirements (Proposed IDs)](#52-new-requirements-proposed-ids)
  - [5.3 System Setting Key (Backend-Agnostic Model)](#53-system-setting-key-backend-agnostic-model)
- [6. Specific Spec Changes](#6-specific-spec-changes)
  - [6.1 Worker Node Spec (`worker_node.md`)](#61-worker-node-spec-worker_nodemd)
  - [6.2 Ports and Endpoints Spec (`ports_and_endpoints.md`)](#62-ports-and-endpoints-spec-ports_and_endpointsmd)
  - [6.3 Other Specs (Orchestrator, Bootstrap, CLI, Web Console)](#63-other-specs-orchestrator-bootstrap-cli-web-console)
  - [6.4 SBA and Sandbox Container Specs (`cynode_sba.md`, `sandbox_container.md`)](#64-sba-and-sandbox-container-specs-cynode_sbamd-sandbox_containermd)
- [7. Implementation Notes](#7-implementation-notes)
- [8. References](#8-references)

## 1. Purpose, Status, and Current State

**Status:** Proposal (dev_docs).
No code or normative spec changes unless explicitly directed.

**Purpose:** Consider alternatives to Ollama for node-local and orchestrator-side inference.
Propose how they could be integrated under a single abstraction so the system is backend-agnostic while preserving current behavior and security.

### 1.1. Current State

- **Worker node:** Node Manager starts a single "Ollama" container (image from orchestrator/env).
  Inference proxy sidecar in sandbox pods forwards to it; sandbox receives `OLLAMA_BASE_URL=http://localhost:11434`.
- **Orchestrator / PMA:** PMA and chat routing call node-local or external inference.
  Requirements and specs refer to "Ollama or similar" (e.g. REQ-MODELS-0004, REQ-WORKER-0115, [API Egress sanity check model config](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheckermodelconfig)).
- **Ports:** Ollama fixed at 11434 (orchestrator dev stack and node).
  Inference proxy listens 11434 inside the pod and forwards to node Ollama.
- **Requirements:** REQ-WORKER-0114 (node-local inference), REQ-WORKER-0115 (keep Ollama private), REQ-WORKER-0123 (node MAY run no Ollama), REQ-MODELS-0004 (orchestrator requests node load model for PM).

Ollama is the only concrete backend specified today.
The rest of the design is already partly backend-agnostic (proxy, allowlist models, no credentials in sandbox).

## 2. Abstraction: Local Inference Backend

Define a **local inference backend** as any node-orchestrator service that:

- Exposes an **OpenAI-compatible** HTTP API (at minimum `/v1/chat/completions`, optionally `/v1/completions`, `/v1/embeddings`).
- Is reachable at a **configurable base URL and port** (no hardcoded 11434 for all backends).
- Is **started and supervised** by the Node Manager (or orchestrator in dev stack) when inference is enabled.
- Does **not** receive provider API keys; it is local-only.

The existing inference proxy and sandbox flow remain: sandbox sees a single stable URL (e.g. `http://localhost:<proxy_port>`) and the proxy forwards to the backend.
The real port or URL of the backend is node-internal only.

## 3. Alternatives to Ollama

- **Backend:** **Ollama**
  - default port: 11434
  - openai api: Native + compat
  - model loading: `ollama pull`
  - notes: Current default; simple, one container.
- **Backend:** **LocalAI**
  - default port: 8080
  - openai api: Drop-in OpenAI
  - model loading: Multiple backends
  - notes: MIT; multi-model, optional GPU; config-driven.
- **Backend:** **vLLM**
  - default port: 8000
  - openai api: OpenAI-compatible
  - model loading: Load at serve time
  - notes: High throughput; GPU-focused; one model per process.
- **Backend:** **ezLocalai**
  - default port: Configurable
  - openai api: Full OpenAI compat
  - model loading: Auto GPU detection
  - notes: One-command setup; distributed inference.
- **Backend:** **Lemonade**
  - default port: Configurable
  - openai api: Standard OpenAI
  - model loading: Local NPU/GPU
  - notes: Windows-friendly; OpenAI interface.

All of the above can serve chat completions over HTTP and can be placed behind the same inference proxy so the sandbox never talks to the backend directly.

### 3.1 Recommended Backends (Rank-Ordered by Value)

Recommendation: support the following backends in priority order.
Rationale emphasizes fit for CyNodeAI (local-first, sandboxed workers, zero-trust, optional GPU).

#### 3.1.1 1. Ollama (Highest Value for Initial Support)

- Already the implemented default; lowest migration and ops cost.
- Single container, on-demand model pull (`ollama pull`), simple lifecycle.
- Strong ecosystem and model library; well-understood in the project.
- **Recommendation:** Define as the default and first-supported backend; retain as the only mandatory backend for MVP.

#### 3.1.2 2. `vllm` (High Value for Scaling and Throughput)

- Production-grade throughput and batching; OpenAI-compatible server.
- Ideal for GPU-heavy nodes and high concurrency (e.g. shared dev or staging).
- One model per process; model loaded at serve time (restart to switch).
- **Recommendation:** Add as the second supported backend once the local-inference abstraction is in place; document default port 8000 and upstream URL derivation.

#### 3.1.3 3. Localai (High Value for Flexibility and Heterogeneous Hardware)

- Multi-backend (e.g. llama.cpp, backends); multi-model; optional GPU.
- Config-driven; good for edge or low-resource nodes and varied hardware.
- **Recommendation:** Add as the third supported backend; document default port 8080 and model-load semantics (config vs pull).

#### 3.1.4 4. Ezlocalai, Lemonade (Lower Priority)

- ezLocalai: one-command setup, distributed inference; smaller ecosystem.
- Lemonade: Windows-friendly; useful if Windows nodes are in scope later.
- **Recommendation:** Do not specify in initial requirements; allow as operator-configured "other" backends via generic upstream URL and optional backend type if the adapter contract supports it.

## 4. How Alternatives Could Be Used

This section describes integration points for alternative backends across the node, orchestrator, and API Egress.

### 4.1 Node-Local Inference (Worker Node)

- **Backend selection:** Orchestrator delivers in node config: backend type (e.g. `ollama`, `vllm`, `localai`) and backend-specific payload (container image, optional args, default port).
  Today: single "Ollama container" image; could become `inference_backend.type` + `inference_backend.image` (and optional `inference_backend.port` or equivalent).
- **Node Manager:** Starts **one** local inference backend container per node when inference is enabled (same policy as current "at most one Ollama container").
  Backend listens on its native port (11434 for Ollama, 8000 for vLLM, etc.) on the node-internal network.
- **Inference proxy:** Forwards to a **configurable upstream URL** (e.g. `OLLAMA_UPSTREAM_URL` generalized to `INFERENCE_UPSTREAM_URL` or derived from backend type + port).
  Proxy continues to listen on a fixed port inside the pod (e.g. 11434) so sandbox env stays `INFERENCE_BASE_URL=http://localhost:11434` (or a single documented port).
- **Sandbox:** Receives one base URL for inference (e.g. `INFERENCE_BASE_URL`).
  Clients use OpenAI-compatible client; no code path should assume Ollama-specific endpoints.
- **Model loading:** Orchestrator request to "load model X" is backend-specific (Ollama: pull; vLLM: restart with model; LocalAI: config).
  Spec would define a small **backend adapter** contract (e.g. "ensure model available") so the orchestrator can drive model readiness regardless of backend.

### 4.2 Orchestrator / Dev Stack

- **Compose / dev:** Instead of a single `ollama` service, support a parameter or profile for "local inference backend" (e.g. `ollama` vs `vllm`) with the appropriate image and port.
  Default can remain Ollama on 11434.
- **PMA and chat:** Already use a base URL and model name; they would use the same `INFERENCE_BASE_URL` (or equivalent) and OpenAI-compatible client.
  No change to flow except configuration source.

### 4.3 API Egress / Sanity Checker

- The [API Egress sanity checker](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck) already allows "Ollama or similar" and configurable external API.
  That would map to "local inference backend (e.g. Ollama, LocalAI, vLLM) or external API," with the same sanity-check contract (allow/deny/escalate) and no credentials in sandbox.

### 4.4 Ports and Endpoints

- **Spec change (proposed):** [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) would describe:
  - **Inference proxy:** Fixed listen port inside sandbox pod (e.g. 11434) and configurable upstream URL to the node local inference backend (node-internal).
  - **Local inference backend:** Default ports per backend (11434 Ollama, 8000 vLLM, 8080 LocalAI, etc.); backend MUST NOT be exposed on a public interface (unchanged from REQ-WORKER-0115).

## 5. Specific Requirement Updates and Additions

Concrete requirement edits and new requirement text with proposed IDs.

### 5.1 Edits to Existing Requirements

Apply the following text changes to the listed requirement docs.

#### 5.1.1 File: `docs/requirements/worker.md`

- **REQ-WORKER-0115 (edit):** Replace current text with:
  - "The node MUST keep the local inference backend access private to the node and MUST NOT require exposing the local inference backend on a public interface."
  - Retain traceability to `CYNAI.WORKER.NodeLocalInference` and `CYNAI.STANDS.PortsAndEndpoints`.

- **REQ-WORKER-0123 (edit):** Replace current text with:
  - "A node MAY be configured to run no local inference backend container."
  - Retain traceability to `CYNAI.WORKER.SandboxOnlyNodes`.

#### 5.1.2 File: `docs/requirements/models.md`

- **REQ-MODELS-0004 (edit):** Replace "When local inference is available (Ollama or similar)" with:
  - "When local inference is available (local inference backend, e.g. Ollama, vLLM, or LocalAI)."
  - Keep the rest of the requirement unchanged (orchestrator requests node load model; qwen3.5:0.8b minimum; traceability unchanged).

#### 5.1.3 File: `docs/requirements/sandbx.md`

- **REQ-SANDBX-0107 (edit):** Replace "The node MUST NOT require exposing Ollama on a public network interface" with:
  - "The node MUST NOT require exposing the local inference backend on a public network interface."

### 5.2 New Requirements (Proposed IDs)

Add the following requirements with the proposed IDs.

#### 5.2.1 Worker Requirements (`docs/requirements/worker.md`)

- **REQ-WORKER-0270 (new):** The node MUST support at least one configurable local inference backend type when inference is enabled.
  The default supported backend MUST be Ollama.
  Other backends (e.g. vLLM, LocalAI) MAY be supported as defined by the implementation and orchestrator configuration.
  Trace to: `CYNAI.WORKER.LocalInferenceBackendPolicy` (worker_node.md).

- **REQ-WORKER-0271 (new):** When the node runs a local inference backend, the Node Manager MUST configure the inference proxy upstream URL from the backend type and port (or from explicit config) so the sandbox never connects directly to the backend.
  Trace to: `CYNAI.WORKER.NodeLocalInference`, `CYNAI.STANDS.InferenceOllamaAndProxy` (or renamed inference spec).

**File: `docs/requirements/models.md`** (new requirement)

- **REQ-MODELS-0126 (new):** The orchestrator node configuration payload MUST allow specifying the local inference backend type and backend-specific parameters (e.g. container image, default port) so the node can start the correct backend.
  Trace to: worker_node_payloads.md (bootstrap or node config payload), worker_node.md Local Inference Backend Policy.

### 5.3 System Setting Key (Backend-Agnostic Model)

**Files:** `docs/requirements/bootst.md`, `docs/requirements/client.md`, and tech specs that reference the key (e.g. `web_console.md`, `cli_management_app_commands_admin.md`, `orchestrator.md`, `orchestrator_bootstrap.md`).

- **Rename (optional but recommended):** `agents.project_manager.model.local_default_ollama_model` to `agents.project_manager.model.local_default_model`.
  If renamed: update all references in bootst.md, client.md, web_console.md, cli_management_app_commands_admin.md, orchestrator.md, orchestrator_bootstrap.md, and examples.
  Behavior: when set, pins the local Project Manager model name for the configured backend (backend-agnostic model id).

## 6. Specific Spec Changes

Exact section names and replacement text for tech specs.

### 6.1 Worker Node Spec (`worker_node.md`)

- **Section "Node Manager" (overview):** Replace "worker API, Ollama, sandbox containers" with "worker API, local inference backend (when enabled), sandbox containers."

- **Section "Node-Local Inference and Sandbox Workflow":**
  - Replace "when a sandbox and Ollama inference are co-located" with "when a sandbox and node-local inference are co-located."
  - In Option A: replace "node's single Ollama container" with "node's single local inference backend container"; replace "Ollama remains a single long-lived container" with "The local inference backend remains a single long-lived container on the node."
  - Replace "The Node Manager MUST inject `OLLAMA_BASE_URL=http://localhost:11434`" with "The Node Manager MUST inject the inference base URL into the sandbox container environment (e.g. `INFERENCE_BASE_URL=http://localhost:11434` or legacy `OLLAMA_BASE_URL` when backend is Ollama).
    The proxy listen port MUST be fixed (e.g. 11434) so the sandbox always sees the same URL regardless of backend."

- **Section "Node Startup Procedure":**
  - Replace "before starting the Ollama container" with "before starting the local inference backend container."
  - Replace "orchestrator can select an Ollama container image" with "orchestrator can select a local inference backend type and image (or equivalent)."
  - Replace "Inference may be provided by node-local inference (Ollama) or by external model routing" with "Inference may be provided by node-local inference (local inference backend) or by external model routing."
  - Replace "Start the single Ollama container specified by the orchestrator" with "Start the single local inference backend container specified by the orchestrator (backend type and image or equivalent from node config)."

- **Rename section "Ollama Container Policy" to "Local Inference Backend Policy":**
  - Spec ID: change to `CYNAI.WORKER.LocalInferenceBackendPolicy` (and add anchor).
  - Replace "The node MUST run at most one Ollama container at a time" with "The node MUST run at most one local inference backend container at a time."
  - Replace "That container MUST be granted access to all GPUs and NPUs" with "That container MUST be granted access to all GPUs and NPUs on the system when the backend uses them."
  - Add: "The orchestrator MUST be able to deliver backend type and backend-specific parameters (e.g. image, default port) in the node configuration so the Node Manager can start the correct backend."

- **Node Startup YAML / capability:** Where inference config is described, add that the node MAY report supported backend types (e.g. `ollama`, `vllm`, `localai`) and that the orchestrator-delivered config MAY include `inference_backend.type` and `inference_backend.image` (or equivalent).

### 6.2 Ports and Endpoints Spec (`ports_and_endpoints.md`)

- **Rename section "Inference (Ollama and Proxy)" to "Inference (Local Backend and Proxy)":**
  - Spec ID: consider renaming to `CYNAI.STANDS.InferenceLocalBackendAndProxy` (and update traceability from worker.md, sandbox_container.md as needed).
  - Replace "Ollama: standard port 11434" with "Local inference backend: default port is backend-dependent (Ollama 11434, vLLM 8000, LocalAI 8080).
    The actual backend port is node-internal only; the inference proxy upstream URL MUST be configurable (e.g. `INFERENCE_UPSTREAM_URL` or derived from backend type and port)."
  - Keep "Inference proxy (sidecar): inside each sandbox pod, the proxy listens on `:11434`" (or a single documented port) "so the sandbox can use `INFERENCE_BASE_URL=http://localhost:11434` (or legacy `OLLAMA_BASE_URL` for backward compatibility).
    The proxy forwards to the node local inference backend (e.g. `http://host.containers.internal:11434` for Ollama, or the configured upstream URL)."

- **Environment and Config Overrides:** Replace "Ollama - Default: 11434" with "Local inference backend - Default port backend-dependent (Ollama 11434, vLLM 8000, LocalAI 8080).
  Override: inference proxy upstream via `INFERENCE_UPSTREAM_URL` (or backend-specific env); sandbox receives `INFERENCE_BASE_URL` (or `OLLAMA_BASE_URL` for legacy)."

- **Conflict Avoidance / E2E:** Where "Ollama" or "11434" are mentioned, add a note that the default backend for E2E and single-host dev is Ollama on 11434; other backends use their default ports when selected.

### 6.3 Other Specs (Orchestrator, Bootstrap, CLI, Web Console)

- **orchestrator.md, orchestrator_bootstrap.md, web_console.md, cli_management_app_commands_admin.md:** If the system setting key is renamed to `agents.project_manager.model.local_default_model`, replace every occurrence of `local_default_ollama_model` with `local_default_model` and state that it denotes the local model id for the configured backend (backend-agnostic).
  If the key is not renamed, add a sentence that the value is the model identifier for the local inference backend in use (e.g. Ollama model name, vLLM model id).

### 6.4 SBA and Sandbox Container Specs (`cynode_sba.md`, `sandbox_container.md`)

- Replace references to "Ollama" or "OLLAMA_BASE_URL" with "local inference backend" and "inference base URL (e.g. `INFERENCE_BASE_URL` or `OLLAMA_BASE_URL`)" so the SBA and sandbox specs are backend-agnostic while preserving legacy env name.

## 7. Implementation Notes

- **Minimal change path:** Keep Ollama as the default and only implemented backend initially; introduce `inference_backend.type` (default `ollama`) and `INFERENCE_UPSTREAM_URL` so the proxy and env are backend-agnostic.
  Add backends later via new type values and backend-specific startup/load logic.
- **Testing:** E2E and BDD can continue to use Ollama; add optional scenarios for a second backend (e.g. vLLM) if desired.
- **Docs:** Operator and development setup docs would list supported backends (Ollama, then vLLM, then LocalAI per recommendation), default ports, and how to set backend type and image.

## 8. References

- [worker_node.md](../tech_specs/worker_node.md) (Node-Local Inference, Ollama Container Policy, Node Startup Procedure)
- [ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) (Inference and proxy)
- [requirements/worker.md](../requirements/worker.md) (REQ-WORKER-0114, 0115, 0123)
- [requirements/models.md](../requirements/models.md) (REQ-MODELS-0004)
- [external_model_routing.md](../tech_specs/external_model_routing.md)
- [API Egress Server (sanity check)](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck)
- [Spec authoring and validation](../docs_standards/spec_authoring_writing_and_validation.md)
