# PMA as Managed Service: Spec Compliance Check

## Spec Requirements (`worker_node_payloads.md`, `worker_node.md`)

- `node_configuration_payload_v1.managed_services.services[]` defines desired state: image, args, env, healthcheck, inference, **orchestrator**.
- Agent runtime services (e.g. PMA) MUST include `orchestrator.mcp_gateway_proxy_url` and `orchestrator.ready_callback_proxy_url`.
- Those are worker-proxy URLs; the agent MUST NOT call the orchestrator directly.
- The node MUST run and supervise managed service containers from this desired state.

## Current Behavior

What the orchestrator and worker do today with managed-service config.

### Orchestrator Side

- `buildManagedServicesDesiredState` (orchestrator) sends one PMA service with:
  - `ServiceID`, `ServiceType`, `Image`, `Args`, `Healthcheck`, `RestartPolicy`, `Role`
  - `Inference`: `Mode`, `BaseURL`, `DefaultModel`
  - `Orchestrator`: `MCPGatewayProxyURL`, `ReadyCallbackProxyURL` (worker internal proxy URLs).
- Example: `http://127.0.0.1:12090/v1/worker/internal/orchestrator/mcp:call` and `.../agent:ready`.
- **Gap:** Orchestrator does **not** set `Orchestrator.AgentToken` in the payload.
- Internal proxy auth on the worker relies on `WORKER_INTERNAL_AGENT_TOKENS_JSON` or config; without token in config, PMA would have no token for internal proxy calls.

### Worker Node (Node-Manager)

- On config apply: writes `WORKER_NODE_CONFIG_JSON`, `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL`, and `WORKER_MANAGED_SERVICE_TARGETS_JSON`.
- `buildManagedServiceTargetsFromConfig`: builds a map from config's `ManagedServices.Services` but **base_url is always from env `PMA_BASE_URL`** (default `http://127.0.0.1:8090`).
- Config's `Inference.BaseURL` is **not** used.
- **Gap:** There is **no** code that starts a PMA (or any managed service) container from `nodeConfig.ManagedServices.Services`.
- No image/args/env/healthcheck from config is used to run a container.
- Only Ollama is started via `StartOllama`.
- So when the orchestrator sends "run PMA as managed service," the worker does **not** start the PMA container; it only configures proxy targets assuming PMA is already running.

### Worker API

- **Orchestrator -> agent:** `POST /v1/worker/managed-services/{service_id}/proxy:http` forwards to the target's `BaseURL`.
- Targets come from `WORKER_MANAGED_SERVICE_TARGETS_JSON` (filled by node-manager from config + `PMA_BASE_URL`).
- So if PMA is running at `PMA_BASE_URL`, orchestrator can reach it via the worker.
- **OK.**
- **Agent -> orchestrator:** Internal mux exposes `POST .../internal/orchestrator/mcp:call` and `.../agent:ready`.
- They forward to `ORCHESTRATOR_INTERNAL_PROXY_BASE_URL`.
- Auth uses `AllowedTokens` (from config's `ManagedServices.Services[].Orchestrator.AgentToken` or `WORKER_INTERNAL_AGENT_TOKENS_JSON`).
- **Endpoints exist.**
- PMA would need to be given the proxy URLs and a token.
- **Gap:** `deriveManagedServiceTargetsFromNodeConfig` returns empty; targets are env-only.
- Config's `inference.base_url` is not used for routing.

## Summary: Does PMA Get Worker Proxy per Spec?

- **Spec expectation:** Node starts PMA container from managed_services desired state.
  - **Current state:** **No.** Worker never starts a managed service container from config.
- **Spec expectation:** PMA receives worker-proxy URLs (mcp_gateway_proxy_url, ready_callback_proxy_url).
  - **Current state:** **No.** Nothing starts PMA or passes config's `Orchestrator` (or env) into a PMA process.
- **Spec expectation:** Orchestrator -> PMA via worker proxy.
  - **Current state:** **Yes**, if PMA is already running at `PMA_BASE_URL`.
- **Spec expectation:** PMA -> orchestrator via worker internal proxy.
  - **Current state:** **Endpoints exist** on worker; PMA is not given the URLs or agent token by the worker (orchestrator also does not send `agent_token` in config).

## Recommendations

1. **Lifecycle:** Add managed-service reconciliation in the worker (e.g. in node-manager or a dedicated component).
   - When config has `managed_services.services[]`, start/update/stop containers from image/args/env/healthcheck.
   - Pass `orchestrator.mcp_gateway_proxy_url`, `orchestrator.ready_callback_proxy_url`, and `orchestrator.agent_token` (or ref) as env to the container so PMA uses worker proxy per spec.
2. **Orchestrator:** Populate `Orchestrator.AgentToken` (or a secure ref) in the managed service payload so the worker can pass it to the agent and the worker-api can authorize internal proxy calls.
3. **Targets:** Optionally derive managed-service target `base_url` from config's `Inference.BaseURL` when present (e.g. for node-local PMA) instead of relying only on `PMA_BASE_URL` env.

See also: `docs/dev_docs/2026-03-05_worker_proxy_spec_reconciliation_plan.md` (Phase 5 desired-state wiring and full managed-service lifecycle).
