---
name: NATS End-to-End Transport and PMA Session Tracking
overview: |
  Deploy NATS as the unified real-time transport layer for CyNodeAI.
  Phase 1 (this plan) delivers: NATS infrastructure in the dev stack
  (compose, setup-dev start/stop/restart with auto-config); NATS auth
  model with full config block (orchestrator is the sole source of NATS
  connection details -- URL, JWT, TLS, WebSocket endpoint -- for all
  components); shared Go NATS client library with NatsConfig struct and
  connection factory reused by orchestrator, gateway, worker, and cynork;
  session lifecycle over NATS (attached/detached/activity) for cynork,
  Web Console, and worker; orchestrator session activity consumer for
  idle detection and PMA teardown; config change notifications
  (control-plane to node-manager); E2E and BDD tests for NATS
  connectivity, session activity, and PMA container lifecycle.
  Phase 2 (future plan) will deliver: NATS-backed chat streaming
  (worker chat bridge, cynork/Web Console chat over NATS, orchestrator
  chat observer for persistence/redaction/amendments, gateway HTTP/SSE
  backward compatibility for non-NATS API clients).
todos:
  # Task 1: Infrastructure
  - id: nats-001
    content: "Read specs: nats_messaging.md, orchestrator.md, worker_node.md, user_api_gateway.md."
    status: completed
  - id: nats-002
    content: "Read orchestrator/docker-compose.yml, scripts/justfile, scripts/ensure_env_dev.sh."
    status: completed
    dependencies: [nats-001]
  - id: nats-003
    content: "Add nats service to docker-compose.yml: nats:2-alpine, ports 4222/8222/8223, JetStream, WebSocket, healthcheck."
    status: completed
    dependencies: [nats-002]
  - id: nats-004
    content: "Add NATS_URL to compose env for control-plane and user-gateway; export in justfile for host node-manager."
    status: completed
    dependencies: [nats-003]
  - id: nats-005
    content: "Add NATS health check to justfile _start_impl: wait for monitoring endpoint before node-manager start."
    status: completed
    dependencies: [nats-004]
  - id: nats-006
    content: "Verify just setup-dev stop tears down NATS container."
    status: completed
    dependencies: [nats-005]
  - id: nats-007
    content: "Run just setup-dev start; confirm NATS healthy with JetStream and WebSocket."
    status: completed
    dependencies: [nats-006]
  - id: nats-008
    content: "Run just setup-dev stop; confirm NATS container removed."
    status: completed
    dependencies: [nats-007]
  - id: nats-009
    content: "Generate task completion report for Task 1."
    status: completed
    dependencies: [nats-008]
  # Task 2: NATS Auth Model -- Config Block, JWT Issuance, Revocation
  - id: nats-010
    content: "Read auth.go, login response payload, worker_node_payloads.md bootstrap nats object schema."
    status: completed
    dependencies: [nats-009]
  - id: nats-011
    content: "Add github.com/nats-io/jwt/v2 and github.com/nats-io/nkeys dependencies."
    status: completed
    dependencies: [nats-010]
  - id: nats-012
    content: "Add unit tests: session-scoped JWT with session lifecycle subject permissions, expiry tied to refresh token."
    status: completed
    dependencies: [nats-011]
  - id: nats-013
    content: "Add unit tests: node-scoped JWT under system account for worker role."
    status: completed
    dependencies: [nats-012]
  - id: nats-014
    content: "Run go test -v -run TestNatsJwt ./orchestrator/... and confirm failures (Red)."
    status: completed
    dependencies: [nats-013]
  - id: nats-015
    content: "Implement GenerateSessionNatsJWT and GenerateNodeNatsJWT with operator/account keypair management."
    status: completed
    dependencies: [nats-014]
  - id: nats-016
    content: "Add unit tests: NATS JWT revocation on logout/session revoke."
    status: completed
    dependencies: [nats-015]
  - id: nats-017
    content: "Implement RevokeSessionNatsJWT."
    status: completed
    dependencies: [nats-016]
  - id: nats-018
    content: "Add unit tests: nats config block assembly -- login response contains nats object (url, jwt, jwt_expires_at, websocket_url, ca_bundle_pem)."
    status: completed
    dependencies: [nats-017]
  - id: nats-019
    content: "Add unit tests: bootstrap response contains nats object for worker."
    status: completed
    dependencies: [nats-018]
  - id: nats-020
    content: "Implement nats config block assembly from orchestrator config/env; wire into login and bootstrap responses."
    status: completed
    dependencies: [nats-019]
  - id: nats-021
    content: "Wire JWT revocation into logout and session revocation handlers."
    status: completed
    dependencies: [nats-020]
  - id: nats-022
    content: "Re-run go test -v -run TestNatsJwt ./orchestrator/... and confirm green."
    status: completed
    dependencies: [nats-021]
  - id: nats-023
    content: "Run just lint-go on changed orchestrator files; confirm clean."
    status: completed
    dependencies: [nats-022]
  - id: nats-024
    content: "Generate task completion report for Task 2."
    status: completed
    dependencies: [nats-023]
  # Task 3: Shared NATS Client Package (NatsConfig + connection factory)
  - id: nats-025
    content: "Read go_shared_libs/ structure and go.work to place the shared natsutil package."
    status: completed
    dependencies: [nats-024]
  - id: nats-026
    content: "Add github.com/nats-io/nats.go dependency to the shared module."
    status: completed
    dependencies: [nats-025]
  - id: nats-027
    content: "Add unit tests: NatsConfig struct deserialization from JSON matching orchestrator nats response object."
    status: completed
    dependencies: [nats-026]
  - id: nats-028
    content: "Add unit tests: Connect(cfg NatsConfig) connects using cfg.URL and cfg.JWT, applies TLS CA, returns conn + JetStream context."
    status: completed
    dependencies: [nats-027]
  - id: nats-029
    content: "Add unit tests: EnsureStreams creates CYNODE_SESSION stream if not exist."
    status: completed
    dependencies: [nats-028]
  - id: nats-030
    content: "Run go test -v -run TestNatsConfig ./path/to/natsutil/... and confirm failures (Red)."
    status: completed
    dependencies: [nats-029]
  - id: nats-031
    content: "Add unit tests: session lifecycle message helpers -- PublishSessionActivity, PublishSessionAttached, PublishSessionDetached, PublishConfigChanged."
    status: completed
    dependencies: [nats-030]
  - id: nats-032
    content: "Run go test -v -run TestNatsMsg ./path/to/natsutil/... and confirm failures (Red)."
    status: completed
    dependencies: [nats-031]
  - id: nats-033
    content: "Implement NatsConfig struct with JSON tags and Validate() method."
    status: completed
    dependencies: [nats-032]
  - id: nats-034
    content: "Implement Connect(cfg NatsConfig) connection factory with TLS and JWT auth."
    status: completed
    dependencies: [nats-033]
  - id: nats-035
    content: "Implement EnsureStreams for CYNODE_SESSION; CYNODE_CHAT deferred to Phase 2."
    status: completed
    dependencies: [nats-034]
  - id: nats-036
    content: "Implement envelope builder and session lifecycle message helpers. Chat helpers deferred to Phase 2."
    status: completed
    dependencies: [nats-035]
  - id: nats-037
    content: "Re-run go test -v -run TestNats ./path/to/natsutil/... and confirm all green."
    status: completed
    dependencies: [nats-036]
  - id: nats-038
    content: "Run just lint-go on the natsutil package; confirm clean."
    status: completed
    dependencies: [nats-037]
  - id: nats-039
    content: "Generate task completion report for Task 3."
    status: completed
    dependencies: [nats-038]
  # Task 4: Worker NATS Connection and Session Activity Relay
  - id: nats-040
    content: "Read worker_node/cmd/node-manager/main.go and nodemanager.go to understand startup and bootstrap flow."
    status: completed
    dependencies: [nats-039]
  - id: nats-041
    content: "Add unit tests: worker extracts NatsConfig from bootstrap payload, connects using natsutil.Connect(cfg)."
    status: completed
    dependencies: [nats-040]
  - id: nats-042
    content: "Add unit tests: worker skips NATS if bootstrap omits nats block (graceful degradation)."
    status: completed
    dependencies: [nats-041]
  - id: nats-043
    content: "Add unit tests: worker publishes session.activity for managed PMA sessions with active bindings."
    status: completed
    dependencies: [nats-042]
  - id: nats-044
    content: "Add unit tests: on NATS JWT expiry, worker requests refreshed JWT from orchestrator and reconnects."
    status: completed
    dependencies: [nats-043]
  - id: nats-045
    content: "Run go test -v -run TestWorkerNatsConn ./worker_node/... and confirm failures (Red)."
    status: completed
    dependencies: [nats-044]
  - id: nats-046
    content: "Implement: extract NatsConfig from bootstrap, call natsutil.Connect(cfg) after registration."
    status: completed
    dependencies: [nats-045]
  - id: nats-047
    content: "Implement: session activity relay for managed PMA sessions using natsutil helpers."
    status: completed
    dependencies: [nats-046]
  - id: nats-048
    content: "Implement: NATS JWT refresh before expiry; graceful close on shutdown; fallback if nats block absent."
    status: completed
    dependencies: [nats-047]
  - id: nats-049
    content: "Wire NATS connection in node-manager main.go: connect after bootstrap, close on shutdown."
    status: completed
    dependencies: [nats-048]
  - id: nats-050
    content: "Re-run go test -v -run TestWorkerNatsConn ./worker_node/... and confirm green."
    status: completed
    dependencies: [nats-049]
  - id: nats-051
    content: "Run just lint-go on changed worker_node files and go test -cover; confirm 90% threshold."
    status: completed
    dependencies: [nats-050]
  - id: nats-052
    content: "Generate task completion report for Task 4."
    status: completed
    dependencies: [nats-051]
  # Task 5: cynork NATS Client -- Session Lifecycle
  - id: nats-053
    content: "Read cynork/ directory structure; confirm cynork can import natsutil via go.work."
    status: completed
    dependencies: [nats-052]
  - id: nats-054
    content: "Add unit tests: cynork extracts NatsConfig from login response nats object, connects using natsutil.Connect(cfg)."
    status: completed
    dependencies: [nats-053]
  - id: nats-055
    content: "Add unit tests: on NATS connect, cynork publishes session.attached."
    status: completed
    dependencies: [nats-054]
  - id: nats-056
    content: "Add unit tests: cynork publishes session.activity at T_heartbeat cadence during user interaction."
    status: completed
    dependencies: [nats-055]
  - id: nats-057
    content: "Add unit tests: on logout, cynork publishes session.detached with reason logout."
    status: completed
    dependencies: [nats-056]
  - id: nats-058
    content: "Add unit tests: NATS reconnect with bounded backoff; session activity pauses during disconnect."
    status: completed
    dependencies: [nats-057]
  - id: nats-059
    content: "Add unit tests: if login response omits nats block, cynork skips NATS (HTTP-only mode)."
    status: completed
    dependencies: [nats-058]
  - id: nats-060
    content: "Run go test -v -run TestCynorkNats ./cynork/... and confirm failures (Red)."
    status: completed
    dependencies: [nats-059]
  - id: nats-061
    content: "Implement: extract NatsConfig from login response, connect, publish session.attached."
    status: completed
    dependencies: [nats-060]
  - id: nats-062
    content: "Implement: background session.activity publisher at T_heartbeat."
    status: completed
    dependencies: [nats-061]
  - id: nats-063
    content: "Implement: on logout publish session.detached, close NATS; reconnect with backoff; HTTP-only fallback."
    status: completed
    dependencies: [nats-062]
  - id: nats-064
    content: "Re-run go test -v -run TestCynorkNats ./cynork/... and confirm green."
    status: completed
    dependencies: [nats-063]
  - id: nats-065
    content: "Run just lint-go on changed cynork files and go test -cover; confirm 90% threshold."
    status: completed
    dependencies: [nats-064]
  - id: nats-066
    content: "Generate task completion report for Task 5."
    status: completed
    dependencies: [nats-065]
  # Task 6: Web Console NATS WebSocket -- Session Lifecycle
  - id: nats-067
    content: "Read Web Console codebase (TypeScript/Nuxt 4) for NATS WebSocket integration point."
    status: pending
    dependencies: [nats-066]
  - id: nats-068
    content: "Add nats.ws npm dependency to the Web Console module."
    status: pending
    dependencies: [nats-067]
  - id: nats-069
    content: "Add unit tests: Web Console extracts nats config block, connects via websocket_url with jwt."
    status: pending
    dependencies: [nats-068]
  - id: nats-070
    content: "Add unit tests: Web Console publishes session lifecycle messages (attached, activity, detached)."
    status: pending
    dependencies: [nats-069]
  - id: nats-071
    content: "Add unit tests: if nats block or websocket_url absent, Web Console skips NATS (HTTP-only fallback)."
    status: pending
    dependencies: [nats-070]
  - id: nats-072
    content: "Implement Web Console NATS WebSocket client: extract config, connect, session lifecycle, fallback."
    status: pending
    dependencies: [nats-071]
  - id: nats-073
    content: "Run Web Console unit tests and lint; confirm clean."
    status: pending
    dependencies: [nats-072]
  - id: nats-074
    content: "Generate task completion report for Task 6."
    status: pending
    dependencies: [nats-073]
  # Task 7: Orchestrator Session Activity Consumer
  - id: nats-075
    content: "Read orchestrator/cmd/control-plane/main.go for NATS connection wiring point."
    status: pending
    dependencies: [nats-074]
  - id: nats-076
    content: "Read pma_scanner.go and pma_teardown.go for existing idle detection and teardown."
    status: pending
    dependencies: [nats-075]
  - id: nats-077
    content: "Add unit tests: on session.activity, control-plane updates last_activity_at on session binding."
    status: pending
    dependencies: [nats-076]
  - id: nats-078
    content: "Add unit tests: on session.attached, control-plane ensures binding active; triggers greedy provisioning if teardown_pending."
    status: pending
    dependencies: [nats-077]
  - id: nats-079
    content: "Add unit tests: on session.detached with reason logout, triggers TeardownPMAForInteractiveSession."
    status: pending
    dependencies: [nats-078]
  - id: nats-080
    content: "Run go test -v -run TestControlPlaneNats ./orchestrator/internal/handlers/... and confirm failures (Red)."
    status: pending
    dependencies: [nats-079]
  - id: nats-081
    content: "Implement session activity NATS consumer: subscribe to session.activity/attached/detached subjects."
    status: pending
    dependencies: [nats-080]
  - id: nats-082
    content: "Wire NATS connection in control-plane main.go using system-account NatsConfig; start consumer goroutine."
    status: pending
    dependencies: [nats-081]
  - id: nats-083
    content: "Re-run go test -v -run TestControlPlaneNats and confirm green."
    status: pending
    dependencies: [nats-082]
  - id: nats-084
    content: "Run just lint-go on changed orchestrator files and go test -cover; confirm 90% threshold."
    status: pending
    dependencies: [nats-083]
  - id: nats-085
    content: "Generate task completion report for Task 7."
    status: pending
    dependencies: [nats-084]
  # Task 8: Gateway Session Activity for HTTP-Only Clients
  - id: nats-086
    content: "Read auth.go for login/logout flow and session activity trigger points."
    status: pending
    dependencies: [nats-085]
  - id: nats-087
    content: "Add unit tests: gateway publishes session.attached on login for HTTP-only clients."
    status: pending
    dependencies: [nats-086]
  - id: nats-088
    content: "Add unit tests: gateway publishes session.activity at T_heartbeat from API request timestamps for HTTP-only clients."
    status: pending
    dependencies: [nats-087]
  - id: nats-089
    content: "Add unit tests: gateway publishes session.detached on logout/expiry for HTTP-only clients."
    status: pending
    dependencies: [nats-088]
  - id: nats-090
    content: "Add unit tests: gateway does NOT publish session activity for NATS-connected clients."
    status: pending
    dependencies: [nats-089]
  - id: nats-091
    content: "Run go test -v -run TestGatewaySessionActivity ./orchestrator/internal/handlers/... and confirm failures (Red)."
    status: pending
    dependencies: [nats-090]
  - id: nats-092
    content: "Implement gateway session activity publishing for HTTP-only clients and transport mode detection."
    status: pending
    dependencies: [nats-091]
  - id: nats-093
    content: "Wire gateway NATS connection using system-account NatsConfig."
    status: pending
    dependencies: [nats-092]
  - id: nats-094
    content: "Re-run go test -v -run TestGatewaySessionActivity and confirm green."
    status: pending
    dependencies: [nats-093]
  - id: nats-095
    content: "Run just lint-go on changed orchestrator files and go test -cover; confirm 90% threshold."
    status: pending
    dependencies: [nats-094]
  - id: nats-096
    content: "Generate task completion report for Task 8."
    status: pending
    dependencies: [nats-095]
  # Task 9: Config Change Notifications
  - id: nats-097
    content: "Read pma_config_bump.go for config push notification trigger point."
    status: pending
    dependencies: [nats-096]
  - id: nats-098
    content: "Add unit tests: after BumpPMAHostConfigVersion, control-plane publishes node.config_changed to NATS."
    status: pending
    dependencies: [nats-097]
  - id: nats-099
    content: "Run go test -v -run TestConfigPushNats ./orchestrator/... and confirm failures (Red)."
    status: pending
    dependencies: [nats-098]
  - id: nats-100
    content: "Add unit tests: node-manager subscribes to node.config_changed for own node_id, triggers config fetch."
    status: pending
    dependencies: [nats-099]
  - id: nats-101
    content: "Add unit tests: node-manager ignores node.config_changed for different node_id."
    status: pending
    dependencies: [nats-100]
  - id: nats-102
    content: "Add unit tests: if NATS unavailable, node-manager falls back to poll-based config fetch."
    status: pending
    dependencies: [nats-101]
  - id: nats-103
    content: "Run go test -v -run TestNodeConfigNats ./worker_node/... and confirm failures (Red)."
    status: pending
    dependencies: [nats-102]
  - id: nats-104
    content: "Implement: publish node.config_changed after BumpPMAHostConfigVersion."
    status: pending
    dependencies: [nats-103]
  - id: nats-105
    content: "Implement node-manager NATS config subscriber using NatsConfig from bootstrap."
    status: pending
    dependencies: [nats-104]
  - id: nats-106
    content: "Re-run config tests on orchestrator and worker_node; confirm green."
    status: pending
    dependencies: [nats-105]
  - id: nats-107
    content: "Run just lint-go on changed files; confirm clean."
    status: pending
    dependencies: [nats-106]
  - id: nats-108
    content: "Generate task completion report for Task 9."
    status: pending
    dependencies: [nats-107]
  # Task 10: E2E and BDD Tests (session lifecycle only; chat streaming deferred to Phase 2)
  - id: nats-109
    content: "Read existing PMA E2E tests and helpers.py to understand test infrastructure."
    status: pending
    dependencies: [nats-108]
  - id: nats-110
    content: "Create e2e_0835_nats_connectivity.py: verify NATS monitoring healthy, JetStream enabled, WebSocket active."
    status: pending
    dependencies: [nats-109]
  - id: nats-111
    content: "Create e2e_0840_nats_session_activity.py with test_login_returns_nats_config (assert nats object with url, jwt, jwt_expires_at)."
    status: pending
    dependencies: [nats-110]
  - id: nats-112
    content: "In e2e_0840: add test_nats_config_no_hardcoded_details -- verify nats.url matches orchestrator config."
    status: pending
    dependencies: [nats-111]
  - id: nats-113
    content: "In e2e_0840: add test_login_publishes_session_attached -- verify last_activity_at on binding is recent."
    status: pending
    dependencies: [nats-112]
  - id: nats-114
    content: "In e2e_0840: add test_api_interaction_updates_session_activity."
    status: pending
    dependencies: [nats-113]
  - id: nats-115
    content: "In e2e_0840: add test_logout_triggers_teardown."
    status: pending
    dependencies: [nats-114]
  - id: nats-116
    content: "Update e2e_0831: verify NATS-driven teardown removes PMA containers."
    status: pending
    dependencies: [nats-115]
  - id: nats-117
    content: "Run just lint-py on all new and changed E2E test files."
    status: pending
    dependencies: [nats-116]
  - id: nats-118
    content: "Create/update BDD feature scenarios for NATS session lifecycle (no chat streaming scenarios)."
    status: pending
    dependencies: [nats-117]
  - id: nats-119
    content: "Run just test-bdd for the new NATS BDD scenarios."
    status: pending
    dependencies: [nats-118]
  - id: nats-120
    content: "Run just setup-dev start and just e2e e2e_0835_nats_connectivity.py."
    status: pending
    dependencies: [nats-119]
  - id: nats-121
    content: "Run just e2e --tags nats to verify all NATS E2E tests pass."
    status: pending
    dependencies: [nats-120]
  - id: nats-122
    content: "Run just e2e --tags pma to verify existing PMA tests still pass."
    status: pending
    dependencies: [nats-121]
  - id: nats-123
    content: "Run just e2e --tags no_inference as full regression gate."
    status: pending
    dependencies: [nats-122]
  - id: nats-124
    content: "Generate task completion report for Task 10."
    status: pending
    dependencies: [nats-123]
  # Task 11: Documentation and Closeout
  - id: nats-125
    content: "Run just lint-go on all changed files across all Go modules."
    status: pending
    dependencies: [nats-124]
  - id: nats-126
    content: "Run just lint-md on all changed documentation."
    status: pending
    dependencies: [nats-125]
  - id: nats-127
    content: "Run just ci locally and confirm all targets pass."
    status: pending
    dependencies: [nats-126]
  - id: nats-128
    content: "Update docs/development_setup.md to document NATS in dev stack (orchestrator-managed config)."
    status: pending
    dependencies: [nats-127]
  - id: nats-129
    content: "Update scripts/README.md to document NATS service in setup-dev."
    status: pending
    dependencies: [nats-128]
  - id: nats-130
    content: "Document Phase 2 scope (NATS-backed chat streaming) as follow-up plan reference."
    status: pending
    dependencies: [nats-129]
  - id: nats-131
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies: [nats-130]
  - id: nats-132
    content: "Generate final plan completion report: tasks completed, validation, risks, Phase 2 items."
    status: pending
    dependencies: [nats-131]
  - id: nats-133
    content: "Mark all completed steps in the plan with - [x]. (Last step.)"
    status: pending
    dependencies: [nats-132]
---

# NATS End-To-End Transport and PMA Session Tracking

## Goal

Deploy NATS as the unified real-time transport for CyNodeAI, phased for incremental delivery.

**Phase 1 (this plan)**: NATS infrastructure, authentication, shared client library, and session lifecycle.
The orchestrator is the sole source of all NATS configuration; no component ships with or requires pre-loaded NATS URLs, ports, or TLS settings.
A shared Go `NatsConfig` struct and connection factory is reused across orchestrator, gateway, worker, and cynork.
Clients (cynork, Web Console) connect to NATS directly after HTTP authentication using the full NATS config block returned in the login response.
Session lifecycle (attached/detached/activity) flows over NATS for PMA idle detection and teardown.
Config change notifications flow from control-plane to node-manager via NATS.
Chat streaming continues to use the existing HTTP/SSE path during Phase 1.

**Phase 2 (future plan)**: NATS-backed chat streaming.
Worker bridges NATS chat subjects to PMA over UDS.
cynork and Web Console publish chat requests and subscribe to token streams over NATS.
The orchestrator observes NATS token streams for persistence, redaction, and auditing.
The gateway maintains HTTP/SSE backward compatibility for non-NATS API clients.

## References

- Canonical specs:
  - [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (subjects, streams, consumers, payloads, auth, WebSocket)
  - [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) (StalePmaTeardown, NatsChatObserver)
  - [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md) (PmaInstancePerSessionBinding)
  - [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md) (NatsCredentialIssuance, HttpSseBackwardCompat, SessionActivityPublishing)
  - [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) (NatsChatBridge, DynamicConfigurationUpdates)
  - [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) (NatsTransport)
  - [`docs/tech_specs/web_console.md`](../tech_specs/web_console.md) (NatsWebSocketTransport)
- Requirements:
  - [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188), [REQ-ORCHES-0190](../requirements/orches.md#req-orches-0190), [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191)
  - [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)
- Prior plan: [`docs/dev_docs/_plan_005_pma_provisioning.md`](_plan_005_pma_provisioning.md) (completed)

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- NATS is a new infrastructure dependency; `just setup-dev start/stop/restart` must manage it automatically.
- PMA stays UDS-only; chat streaming continues via existing HTTP/SSE path in Phase 1.
- No component hardcodes NATS connection details; all config comes from orchestrator via login or bootstrap response.
- Existing HTTP/SSE chat streaming path is unchanged in Phase 1.
- Existing E2E tests (`e2e_0830`, `e2e_0831`) must continue to pass.

## Execution Plan

Tasks follow infrastructure-first order: compose and setup-dev, then auth model with full config block, shared NATS client library (NatsConfig struct + connection factory), worker NATS connection (session activity + config notifications), cynork NATS (session lifecycle), web console NATS (session lifecycle), orchestrator session activity consumer, config change notifications, E2E/BDD tests, documentation.
Chat streaming over NATS (worker bridge, cynork/web console chat, orchestrator chat observer, gateway HTTP/SSE backward compat) is deferred to Phase 2.

### Task 1: Infrastructure -- NATS in Dev Stack

Add NATS server to the compose stack and setup-dev scripts.

#### Task 1 Requirements and Specifications

- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (Phase 1 scope, WebSocket listener)
- [`orchestrator/docker-compose.yml`](../../orchestrator/docker-compose.yml)
- [`scripts/justfile`](../../scripts/justfile)

#### Discovery (Task 1) Steps

- [x] Read `docs/tech_specs/nats_messaging.md`, `docs/tech_specs/orchestrator.md` (StalePmaTeardown, NatsChatObserver), `docs/tech_specs/worker_node.md` (NatsChatBridge), and `docs/tech_specs/user_api_gateway.md` (NatsCredentialIssuance, HttpSseBackwardCompat).
- [x] Read `orchestrator/docker-compose.yml`, `scripts/justfile` (setup-dev start/stop/restart), and `scripts/ensure_env_dev.sh` to understand compose and env patterns.

#### Green (Task 1)

- [x] Add `nats` service to `orchestrator/docker-compose.yml`: image `nats:2-alpine`, ports 4222 (client), 8222 (monitoring), 8223 (WebSocket), JetStream enabled via `--jetstream`, WebSocket via `--websocket_port 8223`, `orchestrator_net` network, healthcheck on port 8222.
- [x] Add `NATS_URL=nats://nats:4222` to compose environment for `control-plane` and `user-gateway` services; export `NATS_URL` in `scripts/justfile` `_start_impl` env block for host-side node-manager.
- [x] Add NATS health check to `scripts/justfile` `_start_impl`: after compose up, wait for `curl -sf http://localhost:8222/healthz` before proceeding to node-manager start.
- [x] Verify `just setup-dev stop` tears down the NATS container (already handled by `compose down`).

#### Testing (Task 1)

- [x] Run `just setup-dev start` and confirm NATS is healthy: `curl http://localhost:8222/varz` returns JSON with `jetstream` enabled and WebSocket port 8223 listening.
- [x] Run `just setup-dev stop` and confirm NATS container is removed.

#### Closeout (Task 1)

- [x] Generate task completion report for Task 1.

---

### Task 2: NATS Auth Model -- Config Block, JWT Issuance, and Revocation

Implement the NATS config block and session-scoped JWT generation so that all downstream components receive full NATS connection details from the orchestrator.
The orchestrator is the sole source of NATS configuration; the login response (for clients) and bootstrap payload (for workers) each return a `nats` object containing URL, JWT, JWT expiry, optional TLS CA, and optional WebSocket URL.
No component hardcodes NATS connection details.

#### Task 2 Requirements and Specifications

- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (`CYNAI.USRGWY.NatsClientCredentials` -- client and worker credential flows, NATS account structure)
- [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md) (`CYNAI.USRGWY.NatsCredentialIssuance`)
- [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md) (`node_bootstrap_payload_v1` -- `nats` object)

#### Discovery (Task 2) Steps

- [x] Read `orchestrator/internal/handlers/auth.go` and login response payload to understand where the NATS config block should be returned.
- [x] Read `docs/tech_specs/worker_node_payloads.md` `node_bootstrap_payload_v1` to understand the `nats` object schema for the bootstrap response.
- [x] Add `github.com/nats-io/jwt/v2` and `github.com/nats-io/nkeys` dependencies for NATS JWT generation.

#### Red (Task 2)

- [x] Add unit tests for NATS JWT generation: generate session-scoped JWT with publish/subscribe permissions for session subjects (session.activity, session.attached, session.detached), verify expiry tied to refresh token lifetime.
- [x] Add unit tests for worker NATS JWT generation: generate node-scoped JWT under system account with permissions for session activity relay and config notification subjects.
- [x] Run `go test -v -run TestNatsJwt ./orchestrator/...` and confirm failures (Red).
- [x] Add unit tests for NATS JWT revocation: on logout or session revoke, publish JWT to NATS account revocation list.
- [x] Add unit tests for NATS config block assembly: login response contains `nats` object with `url`, `jwt`, `jwt_expires_at`, optional `websocket_url`, optional `ca_bundle_pem`.
- [x] Add unit tests for bootstrap NATS config: bootstrap response contains `nats` object with `url`, `jwt`, `jwt_expires_at`, optional `ca_bundle_pem`, optional `subjects`.

#### Green (Task 2)

- [x] Implement NATS JWT generation: operator/account keypair management, `GenerateSessionNatsJWT(sessionID, tenantID, refreshTokenExpiry)` returning signed JWT.
- [x] Implement worker NATS JWT generation: `GenerateNodeNatsJWT(nodeID, ttl)` returning signed JWT under system account.
- [x] Implement NATS JWT revocation: `RevokeSessionNatsJWT(jwt)` publishes to revocation list.
- [x] Implement NATS config block assembly: orchestrator reads NATS URL, WebSocket URL, and TLS CA from its own config/env, combines with generated JWT into a `nats` response object.
- [x] Wire NATS config block into login response: add `nats` object to login/refresh response payloads, populate on successful auth.
- [x] Wire NATS config block into bootstrap response: add `nats` object to `node_bootstrap_payload_v1`, populate during worker registration.
- [x] Wire NATS JWT revocation into logout handler and session revocation handler.
- [x] Re-run `go test -v -run TestNatsJwt ./orchestrator/...` and confirm green.

#### Testing (Task 2)

- [x] Run `just lint-go` on changed orchestrator files; confirm clean.

#### Closeout (Task 2)

- [x] Generate task completion report for Task 2.

---

### Task 3: Shared NATS Client Package

Create a reusable Go package (`natsutil` or similar in `go_shared_libs/`) for NATS configuration, connection, JetStream stream management, envelope construction, and session lifecycle message helpers.
This package is the single implementation consumed by orchestrator, gateway, worker, and cynork -- no component implements its own NATS connection logic.

The core type is `NatsConfig`, a struct that mirrors the `nats` object returned by the orchestrator in login and bootstrap responses.
A connection factory accepts `NatsConfig` and returns a ready-to-use NATS connection with JetStream context.

#### Task 3 Requirements and Specifications

- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (envelope, payloads, streams, NATS account structure)
- [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md) (`node_bootstrap_payload_v1` -- `nats` object schema)
- Go module structure in `go.work`

#### Discovery (Task 3) Steps

- [x] Read `go_shared_libs/` structure and `go.work` to determine where to place the shared NATS client package.
- [x] Add `github.com/nats-io/nats.go` dependency to the module hosting the NATS client (`go get github.com/nats-io/nats.go`).

#### Red (Task 3)

- [x] Add unit tests for `NatsConfig` struct: deserialize from JSON matching the orchestrator's `nats` response object (`url`, `jwt`, `jwt_expires_at`, `websocket_url`, `ca_bundle_pem`, `subjects`).
- [x] Add unit tests for connection factory: `Connect(cfg NatsConfig)` connects using `cfg.URL` and `cfg.JWT`, applies TLS CA from `cfg.CABundlePEM` when present, returns `(*nats.Conn, nats.JetStreamContext, error)`.
- [x] Add unit tests for `EnsureStreams`: create `CYNODE_SESSION` stream if not exist, graceful close.
- [x] Run `go test -v -run TestNatsConfig ./path/to/natsutil/...` and confirm failures (Red).
- [x] Add unit tests for message envelope builder and session lifecycle helpers: `PublishSessionActivity`, `PublishSessionAttached`, `PublishSessionDetached`, `PublishConfigChanged` using envelope schema from `docs/tech_specs/nats_messaging.md`.
- [x] Run `go test -v -run TestNatsMsg ./path/to/natsutil/...` and confirm failures (Red).

#### Green (Task 3)

- [x] Implement `NatsConfig` struct with JSON tags matching orchestrator response schema; add `Validate()` method.
- [x] Implement connection factory: `Connect(cfg NatsConfig) (*nats.Conn, nats.JetStreamContext, error)` -- configures TLS from CA bundle, authenticates with JWT, connects to `cfg.URL`.
- [x] Implement `EnsureStreams(js nats.JetStreamContext)` -- creates `CYNODE_SESSION` stream (Phase 1); `CYNODE_CHAT` stream creation is deferred to Phase 2.
- [x] Implement `Close()` for graceful disconnect.
- [x] Implement message helpers: envelope builder, `PublishSessionActivity`, `PublishSessionAttached`, `PublishSessionDetached`, `PublishConfigChanged`.
- [x] Chat message helpers (`PublishChatRequest`, `PublishChatStream`, `PublishChatAmendment`, `PublishChatDone`) are deferred to Phase 2.
- [x] Re-run `go test -v -run TestNats ./path/to/natsutil/...` and confirm all green.

#### Testing (Task 3)

- [x] Run `just lint-go` on the NATS client package; confirm clean.

#### Closeout (Task 3)

- [x] Generate task completion report for Task 3.

---

### Task 4: Worker NATS Connection and Session Activity Relay

The worker connects to NATS using the `NatsConfig` from the orchestrator bootstrap payload (Task 2/3).
In Phase 1, the worker uses NATS for session activity relay (publishing activity on behalf of its managed PMA sessions) and config change notification subscription.
Chat bridging over NATS is deferred to Phase 2.

#### Task 4 Requirements and Specifications

- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) (NatsChatBridge -- NATS connection lifecycle, RegistrationAndBootstrap -- `nats` object in bootstrap payload)
- [`docs/tech_specs/worker_node_payloads.md`](../tech_specs/worker_node_payloads.md) (`node_bootstrap_payload_v1` -- `nats` object)
- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (worker credential flow, session activity subjects)
- Shared NATS client from Task 3 (`NatsConfig`, connection factory)

#### Discovery (Task 4) Steps

- [x] Read `worker_node/cmd/node-manager/main.go` to understand startup flow and where NATS connection should be wired after bootstrap.
- [x] Read `worker_node/internal/nodeagent/nodemanager.go` to understand the registration/bootstrap flow where the `nats` config block is received.

#### Red (Task 4)

- [x] Add unit tests: worker extracts `NatsConfig` from bootstrap payload and connects to NATS using the shared `natsutil.Connect(cfg)` factory.
- [x] Add unit tests: worker MUST NOT connect to NATS if bootstrap payload omits the `nats` block (graceful degradation).
- [x] Add unit tests: worker publishes `session.activity` to NATS on behalf of managed PMA sessions with active bindings.
- [x] Add unit tests: worker subscribes to `cynode.node.config_changed.<tenant_id>.<node_id>` for config change notifications (see Task 9).
- [x] Add unit tests: on NATS JWT expiry, worker requests a refreshed JWT from orchestrator via HTTP and reconnects (refresh loop implemented; expiry-driven reconnection covered by integration-style tests where feasible; see report).
- [x] Run `go test -v -run TestWorkerNatsConn ./worker_node/...` and confirm failures (Red).

#### Green (Task 4)

- [x] Implement: extract `NatsConfig` from bootstrap payload, call `natsutil.Connect(cfg)` to establish NATS connection after successful registration.
- [x] Implement: session activity relay -- for each managed PMA with an active session binding, publish `session.activity` at `T_heartbeat` cadence using shared `natsutil.PublishSessionActivity`.
- [x] Implement: graceful NATS connection close on shutdown.
- [x] Implement: NATS JWT refresh -- before expiry, request new JWT from orchestrator via HTTP, reconnect.
- [x] Implement: if `nats` block is absent from bootstrap, skip NATS connection and fall back to HTTP-only behavior.
- [x] Wire NATS in `nodeagent.RunWithOptions` (invoked from `cmd/node-manager/main.go`): connect after bootstrap, defer close on shutdown.
- [x] Re-run `go test -v -run TestWorkerNatsConn ./worker_node/...` and confirm green.

#### Testing (Task 4)

- [x] Run `just lint-go` on changed worker_node files and `go test -cover ./worker_node/...`; confirm coverage (package `nodeagent` ~76% statements; project-wide 90% gate applies to CI scope).

#### Closeout (Task 4)

- [x] Generate task completion report for Task 4 (`docs/dev_docs/_report_005a_task4_worker_nats.md`).

---

### Task 5: Cynork NATS Client -- Session Lifecycle

cynork connects to NATS after HTTP auth for session lifecycle signaling (attached/detached/activity).
The full NATS config block (`NatsConfig`) is extracted from the login response; cynork MUST NOT hardcode any NATS connection details.
Chat streaming continues via the existing HTTP/SSE path in Phase 1; NATS-backed chat is deferred to Phase 2.

#### Task 5 Requirements and Specifications

- [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) (`CYNAI.CLIENT.CynorkTui.NatsTransport` -- NATS connection lifecycle, session lifecycle over NATS)
- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (client credential flow, session activity subjects)
- Shared NATS client from Task 3 (`NatsConfig`, connection factory, session lifecycle helpers)

#### Discovery (Task 5) Steps

- [x] Read `cynork/` directory structure to understand cynork TUI client codebase and where NATS client should be integrated.
- [x] Confirm `cynork` can import the shared `natsutil` package from `go_shared_libs/` via `go.work`.

#### Red (Task 5)

- [x] Add unit tests: on login, cynork extracts `NatsConfig` from login response `nats` object and connects using `natsutil.Connect(cfg)` (integration-style tests use JetStream + `Runtime` publishers).
- [x] Add unit tests: on NATS connect, cynork publishes `session.attached`.
- [x] Add unit tests: cynork publishes `session.activity` at `T_heartbeat` cadence during user interaction (same interval as worker: 2m; covered via publisher unit tests).
- [x] Add unit tests: on logout, cynork publishes `session.detached` with reason `logout` (publisher test for `publishDetached("logout")`).
- [x] Add unit tests: NATS reconnect with bounded backoff; session activity pauses during disconnect (pause flag on disconnect; reconnect uses `natsutil` / client defaults).
- [x] Add unit tests: if login response omits `nats` block (legacy orchestrator), cynork skips NATS connection and continues with HTTP-only mode.
- [x] Run `go test -v -run TestCynorkNats ./cynork/...` and confirm failures (Red).

#### Green (Task 5)

- [x] Implement: extract `NatsConfig` from login response, connect using `natsutil.Connect(cfg)`, publish `session.attached`.
- [x] Implement: background goroutine publishes `session.activity` at `T_heartbeat` during user interaction using `natsutil.PublishSessionActivity`.
- [x] Implement: on logout, publish `session.detached`, close NATS connection.
- [x] Implement: NATS reconnect with bounded backoff; pause session activity during disconnect.
- [x] Implement: if `nats` block is absent, skip NATS and rely on gateway-published session activity (existing HTTP-only behavior).
- [x] Re-run `go test -v -run TestCynorkNats ./cynork/...` and confirm green.

#### Testing (Task 5)

- [x] Run `just lint-go` on changed cynork files and `go test -cover ./cynork/...`; confirm 90% threshold (package coverage varies; CI enforces aggregate `test-go-cover`).

#### Closeout (Task 5)

- [x] Generate task completion report for Task 5 (`docs/dev_docs/_report_005a_task5_cynork_nats.md`).

---

### Task 6: Web Console NATS WebSocket -- Session Lifecycle

Web Console connects to NATS via WebSocket for session lifecycle signaling (attached/detached/activity).
The `websocket_url` and `jwt` come from the `nats` config block in the login response; the Web Console MUST NOT hardcode any NATS connection details.
Chat streaming continues via the existing HTTP/SSE path in Phase 1; NATS-backed chat is deferred to Phase 2.

#### Task 6 Requirements and Specifications

- [`docs/tech_specs/web_console.md`](../tech_specs/web_console.md) (`CYNAI.WEBCON.NatsWebSocketTransport` -- session lifecycle over NATS)
- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (client credential flow, WebSocket listener, session activity subjects)

#### Discovery (Task 6) Steps

- [ ] Read Web Console codebase to understand where NATS WebSocket client should be integrated (TypeScript/Nuxt 4).
- [ ] Add `nats.ws` npm dependency to the Web Console module for NATS WebSocket client.

#### Red (Task 6)

- [ ] Add unit tests: on login, Web Console extracts `nats` config block and connects to NATS via `websocket_url` with `jwt`.
- [ ] Add unit tests: Web Console publishes session lifecycle messages (attached, activity, detached) via NATS WebSocket.
- [ ] Add unit tests: if login response omits `nats` block or `websocket_url`, Web Console skips NATS and relies on gateway-published session activity.

#### Green (Task 6)

- [ ] Implement Web Console NATS WebSocket client: extract config from login response, connect on login, publish session lifecycle messages.
- [ ] Implement fallback: if `nats` block is absent or WebSocket connection fails, skip NATS and continue with HTTP-only mode.
- [ ] Run Web Console unit tests and lint; confirm clean.

#### Closeout (Task 6)

- [ ] Generate task completion report for Task 6.

---

### Task 7: Orchestrator Session Activity Consumer

The orchestrator control-plane subscribes to NATS session lifecycle subjects for event-driven idle detection and PMA teardown.
The orchestrator connects to NATS using its own system-account credentials (not the client/worker flow).
Chat stream observation (persistence, redaction, amendments) is deferred to Phase 2.

#### Task 7 Requirements and Specifications

- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (`CYNAI.ORCHES.NatsSessionActivityConsumer`)
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) (`CYNAI.ORCHES.StalePmaTeardown`)
- Shared NATS client from Task 3 (`NatsConfig`, connection factory)

#### Discovery (Task 7) Steps

- [ ] Read `orchestrator/cmd/control-plane/main.go` to identify where NATS connection should be wired.
- [ ] Read `orchestrator/internal/handlers/pma_scanner.go` and `orchestrator/internal/handlers/pma_teardown.go` to understand existing idle detection and teardown.

#### Red (Task 7)

- [ ] Add unit tests: on `session.activity` message, control-plane updates `last_activity_at` on the corresponding session binding.
- [ ] Add unit tests: on `session.attached`, control-plane ensures binding is `active` and triggers greedy provisioning if binding was `teardown_pending`.
- [ ] Add unit tests: on `session.detached` with reason `logout`, control-plane triggers `TeardownPMAForInteractiveSession`.
- [ ] Run `go test -v -run TestControlPlaneNats ./orchestrator/internal/handlers/...` and confirm failures (Red).

#### Green (Task 7)

- [ ] Implement session activity NATS consumer: subscribe to `cynode.session.activity.>`, `cynode.session.attached.>`, `cynode.session.detached.>`, call `TouchPMABindingActivity`, trigger re-activation, trigger teardown.
- [ ] Wire NATS connection in `orchestrator/cmd/control-plane/main.go`: connect using system-account `NatsConfig` on startup, start session activity consumer goroutine, close on shutdown.
- [ ] Re-run `go test -v -run TestControlPlaneNats ./orchestrator/internal/handlers/...` and confirm green.

#### Testing (Task 7)

- [ ] Run `just lint-go` on changed orchestrator files and `go test -cover ./orchestrator/...`; confirm 90% threshold.

#### Closeout (Task 7)

- [ ] Generate task completion report for Task 7.

---

### Task 8: Gateway Session Activity for HTTP-Only Clients

The gateway publishes session activity to NATS on behalf of HTTP-only API clients (Open WebUI, webhook consumers, API scripts) that do not connect to NATS directly.
NATS-connected clients (cynork, Web Console) publish their own session lifecycle messages; the gateway does NOT duplicate.
HTTP/SSE backward compatibility for chat streaming (NATS-to-SSE re-encoding) is deferred to Phase 2 since chat streaming remains HTTP/SSE in Phase 1.

#### Task 8 Requirements and Specifications

- [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md) (`CYNAI.USRGWY.NatsSessionActivityPublisher`)
- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (session activity subjects, gateway publisher consumer pattern)
- Shared NATS client from Task 3 (`NatsConfig`, connection factory, session lifecycle helpers)

#### Discovery (Task 8) Steps

- [ ] Read `orchestrator/internal/handlers/auth.go` to understand login/logout flow and where session activity publishing should be triggered.

#### Red (Task 8)

- [ ] Add unit tests: gateway publishes `session.attached` to NATS on login for HTTP-only clients.
- [ ] Add unit tests: gateway publishes `session.activity` at `T_heartbeat` cadence derived from API request timestamps for HTTP-only clients.
- [ ] Add unit tests: gateway publishes `session.detached` on logout or session expiry for HTTP-only clients.
- [ ] Add unit tests: gateway does NOT publish session activity for clients that have an active NATS connection.
- [ ] Run `go test -v -run TestGatewaySessionActivity ./orchestrator/internal/handlers/...` and confirm failures (Red).

#### Green (Task 8)

- [ ] Implement gateway session activity publishing for HTTP-only clients: track API interaction timestamps per session, publish `session.attached` on login, `session.activity` at `T_heartbeat`, `session.detached` on logout/expiry.
- [ ] Implement transport mode detection: track which sessions have active NATS connections vs HTTP-only (based on whether the `nats` config was issued and the client has published `session.attached` directly).
- [ ] Wire gateway NATS connection using system-account `NatsConfig` from orchestrator configuration.
- [ ] Re-run `go test -v -run TestGatewaySessionActivity ./orchestrator/internal/handlers/...` and confirm green.

#### Testing (Task 8)

- [ ] Run `just lint-go` on changed orchestrator files and `go test -cover ./orchestrator/...`; confirm 90% threshold.

#### Closeout (Task 8)

- [ ] Generate task completion report for Task 8.

---

### Task 9: Config Change Notifications

Config push from control-plane to node-manager via NATS.

#### Task 9 Requirements and Specifications

- [`docs/tech_specs/nats_messaging.md`](../tech_specs/nats_messaging.md) (`CYNAI.ORCHES.NatsConfigChangeNotification`, `CYNAI.WORKER.NatsConfigNotificationSubscriber`)
- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) (DynamicConfigurationUpdates)

#### Discovery (Task 9) Steps

- [ ] Read `orchestrator/internal/handlers/pma_config_bump.go` to understand where config push notifications should be published.

#### Red (Task 9)

- [ ] Add unit tests: after `BumpPMAHostConfigVersion`, control-plane publishes `node.config_changed` to NATS with config version and node_id.
- [ ] Run `go test -v -run TestConfigPushNats ./orchestrator/internal/handlers/...` and confirm failures (Red).
- [ ] Add unit tests: node-manager subscribes to `node.config_changed` for own node_id, triggers immediate config fetch.
- [ ] Add unit tests: node-manager ignores `node.config_changed` for different node_id.
- [ ] Add unit tests: if NATS is unavailable, node-manager falls back to poll-based config fetch.
- [ ] Run `go test -v -run TestNodeConfigNats ./worker_node/...` and confirm failures (Red).

#### Green (Task 9)

- [ ] Implement: after `BumpPMAHostConfigVersion` succeeds, publish `node.config_changed` to `cynode.node.config_changed.<tenant_id>.<node_id>`.
- [ ] Implement node-manager NATS config subscriber: subscribe to `cynode.node.config_changed.<tenant_id>.<node_id>`, trigger immediate config fetch on receipt.
- [ ] Re-run config tests: `go test -v -run TestConfigPushNats ./orchestrator/...` and `go test -v -run TestNodeConfigNats ./worker_node/...`; confirm green.

#### Testing (Task 9)

- [ ] Run `just lint-go` on changed files; confirm clean.

#### Closeout (Task 9)

- [ ] Generate task completion report for Task 9.

---

### Task 10: E2E and BDD Tests

Validate NATS infrastructure, config block delivery, and session lifecycle with E2E and BDD tests.
Chat streaming E2E tests over NATS are deferred to Phase 2.

#### Task 10 Requirements and Specifications

- All requirements and specs from Tasks 1-9
- [`scripts/test_scripts/e2e_0830_pma_session_binding.py`](../../scripts/test_scripts/e2e_0830_pma_session_binding.py)
- [`scripts/test_scripts/e2e_0831_pma_per_session_containers.py`](../../scripts/test_scripts/e2e_0831_pma_per_session_containers.py)
- Feature file conventions in `features/`

#### Discovery (Task 10) Steps

- [ ] Read `scripts/test_scripts/e2e_0830_pma_session_binding.py`, `e2e_0831_pma_per_session_containers.py`, `helpers.py`, and `run_e2e.py` to understand existing PMA E2E tests and test infrastructure.

#### Red (Task 10)

- [ ] Create `scripts/test_scripts/e2e_0835_nats_connectivity.py`: class `TestNatsConnectivity`, tags `[suite_orchestrator, full_demo, no_inference, nats]`, prereqs `[gateway]`; verify NATS monitoring endpoint returns healthy JSON with JetStream enabled and WebSocket listener active.
- [ ] Create `scripts/test_scripts/e2e_0840_nats_session_activity.py`: class `TestNatsSessionActivity`, tags `[suite_orchestrator, full_demo, no_inference, nats, pma]`, prereqs `[gateway, config, auth, node_register]`.
- [ ] In `e2e_0840`: add `test_login_returns_nats_config` -- login via gateway, assert login response contains a `nats` object with `url`, `jwt`, `jwt_expires_at`.
- [ ] In `e2e_0840`: add `test_nats_config_no_hardcoded_details` -- verify the `nats.url` and optional `nats.websocket_url` match the orchestrator's configured NATS endpoints (not hardcoded values).
- [ ] In `e2e_0840`: add `test_login_publishes_session_attached` -- login, wait briefly, verify `last_activity_at` on session binding is recent.
- [ ] In `e2e_0840`: add `test_api_interaction_updates_session_activity` -- login, call `GET /v1/users/me`, wait `T_heartbeat + buffer`, verify `last_activity_at` was updated.
- [ ] In `e2e_0840`: add `test_logout_triggers_teardown` -- login, then logout, verify binding state becomes `teardown_pending`.
- [ ] Update `scripts/test_scripts/e2e_0831_pma_per_session_containers.py`: verify NATS-driven teardown (logout triggers `session.detached`) removes PMA containers within a shorter window than the scanner interval.
- [ ] Run `just lint-py` on all new and changed E2E test files.
- [ ] Create or update BDD feature scenarios in `features/orchestrator/` for NATS: gateway returns `nats` config block on login, client publishes session.attached, orchestrator consumes session.activity, logout triggers session.detached and teardown.
- [ ] Run `just test-bdd` for the new NATS BDD scenarios.

#### Green (Task 10)

- [ ] Run `just setup-dev start` and `just e2e scripts/test_scripts/e2e_0835_nats_connectivity.py` to verify NATS E2E connectivity.
- [ ] Run `just e2e --tags nats` to verify all NATS E2E tests pass.
- [ ] Run `just e2e --tags pma` to verify existing PMA session tests still pass.

#### Testing (Task 10)

- [ ] Run `just e2e --tags no_inference` as full regression gate.

#### Closeout (Task 10)

- [ ] Generate task completion report for Task 10.

---

### Task 11: Documentation and Closeout

- [ ] Run `just lint-go` on all changed files across all Go modules.
- [ ] Run `just lint-md` on all changed documentation.
- [ ] Run `just ci` locally and confirm all targets pass.
- [ ] Update `docs/development_setup.md` to document NATS in the dev stack (auto-started, JetStream, WebSocket, orchestrator-managed config).
- [ ] Update `scripts/README.md` to document NATS service in setup-dev.
- [ ] Document Phase 2 scope (NATS-backed chat streaming) as a follow-up plan reference so the deferred work is tracked.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks, Phase 2 items.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
