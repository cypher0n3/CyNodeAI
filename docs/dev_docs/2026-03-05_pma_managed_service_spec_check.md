# PMA as Managed Service: Spec Compliance Check

## Spec Requirements (`worker_node_payloads.md`, `worker_node.md`)

- `node_configuration_payload_v1.managed_services.services[]` defines desired state: image, args, env, healthcheck, inference, **orchestrator**.
- Agent runtime services (e.g. PMA) MUST include `orchestrator.mcp_gateway_proxy_url` and `orchestrator.ready_callback_proxy_url`.
- Those are worker-proxy URLs; the agent MUST NOT call the orchestrator directly.
- The node MUST run and supervise managed service containers from this desired state.

## Spec Validation (PMA and Worker)

The following spec anchors confirm that PMA receives worker-proxy URLs from the managed-service start bundle and that the **worker** holds the agent token (agents must never receive tokens or secrets directly).

- **`cynode_pma.md` Configuration Surface:** Worker-proxy URLs for MCP gateway and callback/ready signaling.
  Inference connectivity is in the PMA start bundle.
- **`cynode_pma.md` MCP Tool Access:** PMA calls the worker proxy; the worker proxy holds the PM agent token (delivered to the worker in node config) and attaches it when forwarding to the MCP gateway.
  PMA does not receive or present the token.
  See [Agent-Scoped Tokens or API Keys](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-agentscopedtokens).
- **`cynode_pma.md` Purpose and Trust Boundary:** Agent-to-orchestrator communication MUST flow through the worker proxy.
  PMA MUST NOT call orchestrator directly.
- **`worker_node_payloads.md`** `managed_services.services[]` for agent runtime (e.g. `service_type=pma`) MUST include `orchestrator` with `mcp_gateway_proxy_url`, `ready_callback_proxy_url` (optional), `agent_token` (optional), `agent_token_ref` (optional).
  The `agent_token` is delivered to the **worker**; the worker proxy MUST hold it and attach it when forwarding.
  The token MUST NOT be passed to the agent container.
  See [CYNAI.WORKER.AgentTokensWorkerHeldOnly](../tech_specs/worker_node.md#spec-cynai-worker-agenttokensworkerheldonly).
- **`worker_node.md` Token and credential handling:** Agents MUST NOT be given tokens or secrets directly; the worker proxy holds orchestrator-issued credentials and attaches them when forwarding agent-originated requests.
  See [CYNAI.WORKER.AgentTokensWorkerHeldOnly](../tech_specs/worker_node.md#spec-cynai-worker-agenttokensworkerheldonly).
- **`worker_node.md` Agent Token Storage and Lifecycle:** The worker MUST store agent tokens in the **node-local secure store** ([CYNAI.WORKER.NodeLocalSecureStore](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore)) and MUST NOT pass tokens into any agent container.
  On config apply, the worker resolves the token and writes it to the secure store keyed by service identity; the worker proxy loads the token from the store when forwarding and attaches it to the request.
  Tokens MUST NOT be exposed via env, files, mounts, or logs.
  See [CYNAI.WORKER.AgentTokenStorageAndLifecycle](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle) and its [algorithm](../tech_specs/worker_node.md#algo-cynai-worker-agenttokenstorageandlifecycle).
- **PMA token is system-level:** Per `mcp_gateway_enforcement.md`, the PM (PMA) token is not bound to a user; only PA and sandbox tokens are user-associated.
- **`orchestrator_bootstrap.md` PMA Startup:** The orchestrator delivers the PMA start bundle via node configuration (managed services desired state).
- **`worker_node.md` Managed Service Containers:** The worker MUST converge to desired state (create/start when missing) and keep PMA running when configured.

## Current Behavior

What the orchestrator and worker do today with managed-service config.

### Orchestrator Side

- `buildManagedServicesDesiredState` (orchestrator) sends one PMA service with:
  - `ServiceID`, `ServiceType`, `Image`, `Args`, `Healthcheck`, `RestartPolicy`, `Role`
  - `Inference`: `Mode`, `BaseURL`, `DefaultModel`
  - `Orchestrator`: `MCPGatewayProxyURL`, `ReadyCallbackProxyURL` (worker internal proxy URLs).
- Example: `http://127.0.0.1:12090/v1/worker/internal/orchestrator/mcp:call` and `.../agent:ready`.
- **Closed:** Orchestrator sets `Orchestrator.AgentToken` when `WORKER_INTERNAL_AGENT_TOKEN` is set.
  Token is delivered to the **worker** (in node config); the worker proxy holds it and attaches it when forwarding internal proxy calls to the orchestrator.
  Per spec, the token is not given to the agent.

### Orchestrator (user-gateway / openai_chat)

- **PMA routing:** The orchestrator MUST route `model=cynodeai.pm` traffic to PMA using only worker-reported endpoints from capability snapshots (`managed_services_status`). It MUST NOT use `PMA_BASE_URL` or direct addressing (REQ-ORCHES-0162). Implemented: `resolvePMAEndpoint` returns only from `collectReadyPMACandidates`; no env fallback.

### Worker Node (Node-Manager)

- On config apply: writes `WORKER_NODE_CONFIG_JSON`, `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL`, and `WORKER_MANAGED_SERVICE_TARGETS_JSON`.
- `buildManagedServiceTargetsFromConfig`: builds a map from config's `ManagedServices.Services` but **base_url is always from env `PMA_BASE_URL`** (default `http://127.0.0.1:8090`).
- Config's `Inference.BaseURL` is **not** used.
- **Closed:** Node-manager starts managed service containers from `nodeConfig.ManagedServices.Services` via `StartManagedServices` (RunOptions).
  When config has `managed_services.services[]`, the node runs each service with image/args/env/restart_policy and passes worker-proxy URLs as env (MCP_GATEWAY_PROXY_URL, READY_CALLBACK_PROXY_URL) so the agent knows where to call.
  Per spec the worker MUST store the agent token in the **node-local secure store** and MUST NOT pass it to the container; the worker proxy loads the token from the store when forwarding.
  Implementation may still pass AGENT_TOKEN as env or use in-memory AllowedTokens; it should be updated to implement [AgentTokenStorageAndLifecycle](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle) (secure store, no token in env/mounts/logs).
  Container names: `cynodeai-managed-<service_id>`; PMA type publishes port 8090.
  Implemented in `worker_node/cmd/node-manager/main.go` (startManagedServices) and `worker_node/internal/nodeagent/nodemanager.go` (maybeStartManagedServices).

### Worker API

- **Orchestrator -> agent:** `POST /v1/worker/managed-services/{service_id}/proxy:http` forwards to the target's `BaseURL`.
- Targets come from `WORKER_MANAGED_SERVICE_TARGETS_JSON` (filled by node-manager from config + `PMA_BASE_URL`). The **orchestrator** does not use this env; it uses only the endpoint(s) the worker reports in capability `managed_services_status` (REQ-ORCHES-0162).
- **OK.**
- **Agent -> orchestrator:** Internal mux exposes `POST .../internal/orchestrator/mcp:call` and `.../agent:ready`.
- They forward to `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL`.
- Auth uses `AllowedTokens` (from config's `ManagedServices.Services[].Orchestrator.AgentToken` or `WORKER_INTERNAL_AGENT_TOKENS_JSON`).
  Per spec the worker proxy should load the agent token from the **node-local secure store** (keyed by service identity) when forwarding and attach it; the agent is not given the token.
  Implementation may still use in-memory AllowedTokens; see [AgentTokenStorageAndLifecycle](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle).
- **Endpoints exist.**
- **Gap:** `deriveManagedServiceTargetsFromNodeConfig` returns empty; targets are env-only.
- Config's `inference.base_url` is not used for routing.

## Summary: Does PMA Get Worker Proxy per Spec?

- **Spec expectation:** Node starts PMA container from managed_services desired state.
  - **Current state:** **Yes.**
    Node-manager starts managed service containers via StartManagedServices when config has managed_services.services[].
- **Spec expectation:** PMA receives worker-proxy URLs so it knows where to call; the worker stores the agent token in the secure store and the worker proxy loads it when forwarding (agent never receives the token).
  - **Current state:** **Yes** for proxy URLs.
    Node-manager passes MCP_GATEWAY_PROXY_URL and READY_CALLBACK_PROXY_URL when starting the container.
    Spec requires worker to store the token in the node-local secure store and not pass it to the agent; implementation may still pass token as env or use in-memory AllowedTokens (see Recommendations).
- **Spec expectation:** Orchestrator -> PMA via worker proxy.
  - **Current state:** **Yes**, when PMA is running at the target URL.
- **Spec expectation:** PMA -> orchestrator via worker internal proxy.
  - **Current state:** **Yes.**
    Endpoints exist on the worker; the worker proxy forwards to the orchestrator and (per spec) holds and attaches the agent token; the agent does not receive the token.

## Recommendations

1. **Lifecycle:** Done.
   Node-manager `StartManagedServices` starts containers from `managed_services.services[]`; passes proxy URLs as env so the agent calls the worker proxy.
2. **Orchestrator:** Done.
   Set `WORKER_INTERNAL_AGENT_TOKEN`; orchestrator populates `Orchestrator.AgentToken` in the payload.
   The token is for the **worker** to hold and use when forwarding; it must not be given to the agent.
3. **Token storage (spec alignment):** Implement [AgentTokenStorageAndLifecycle](../tech_specs/worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle): on config apply, resolve token and write to the [node-local secure store](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalsecurestore) keyed by service identity; do not pass token to the container; when the worker proxy forwards an agent-originated request, load the token from the store and attach it.
   Remove any AGENT_TOKEN env or in-container token delivery; do not expose tokens in logs, mounts, or debug endpoints.
   See also `docs/dev_docs/2026-03-05_worker_agent_token_secure_holding_spec_gaps.md`.
4. **Targets:** Optionally derive managed-service target `base_url` from config's `Inference.BaseURL` when present instead of relying only on `PMA_BASE_URL` env.

See also: `docs/dev_docs/2026-03-05_worker_proxy_spec_reconciliation_plan.md` (Phase 5 desired-state wiring and full managed-service lifecycle).
