# Proposal: Single Binary for Node Manager and Worker API (Host Binary Deployment)

<!-- Canonicalized (2026-03-12): Merged into requirements (REQ-WORKER-0262, REQ-WORKER-0263) and worker_node.md (Deployment Topologies, Single-Process Host Binary). Implementation is single process only; Worker API as a separate service or container is not retained. -->

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Design Decision: Option A](#design-decision-option-a)
- [Current State](#current-state)
- [Goals and Rationale](#goals-and-rationale)
- [Proposed Requirements](#proposed-requirements)
- [Proposed Tech Spec Content](#proposed-tech-spec-content)
- [Backward Compatibility and Migration](#backward-compatibility-and-migration)
- [Traceability](#traceability)
- [Open Points](#open-points)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-10
- Status: Proposal (draft_specs; not merged to requirements/specs)
- Scope: Host (binary) deployment where Node Manager (WNM) and Worker API run on the same system.
  This proposal **adopts Option A**: a single binary that runs both in one process for host binary deployment, and codifies deployment topologies and behavior in requirements and tech spec.

## Summary

When the worker node is deployed as binaries on the host (not containerized), the Node Manager and Worker API are today run as two processes: the Node Manager starts the Worker API as a subprocess (or as a container when `NODE_MANAGER_WORKER_API_IMAGE` is set).
This proposal adopts **Option A (single binary, single process)** for host binary deployment: one binary runs both Node Manager logic and the Worker API HTTP server in the same process.
The Worker API as container topology (`NODE_MANAGER_WORKER_API_IMAGE`) is retained for environments that require it.
Requirements and tech spec are extended to define deployment topologies and the single-process startup and shutdown behavior explicitly.

## Design Decision: Option A

The proposal aligns on **Option A: Single binary, single process (embedded Worker API)**.

- One binary (e.g. `cynodeai-wnm` or `cynodeai-node`) runs both Node Manager and Worker API in the same process when used for host binary deployment.
- Node Manager goroutines perform registration, config fetch, config apply, and start Ollama and managed services; the Worker API HTTP server listens on the configured port (e.g. 12090) in the same process.
- No subprocess is spawned for Worker API in this mode; the trusted boundary for secure store and telemetry is the single process (see [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary)).
- When `NODE_MANAGER_WORKER_API_IMAGE` is set, the implementation starts Worker API as a managed container instead of in-process; that topology remains unchanged.

Rejected alternatives (brief):

- **Option B (single binary, two modes):** Would still run two processes and require env forwarding and separate-process boundary; does not simplify lifecycle or secure store.
- **Option C (keep two binaries, codify only):** Would not simplify code paths or deployment; the opportunity to reduce complexity is foregone.

## Current State

- **Two entrypoints:** `worker_node/cmd/node-manager/main.go` (cynodeai-wnm) and `worker_node/cmd/worker-api/main.go` (worker-api).
- **Binary deployment:** Node Manager starts Worker API via `startWorkerAPIBinary()` (exec of worker-api with env: `WORKER_API_BEARER_TOKEN`, `INFERENCE_PROXY_IMAGE`, `OLLAMA_UPSTREAM_URL`, `CONTAINER_RUNTIME`, `WORKER_API_STATE_DIR`, and `NODE_SKIP_NODE_BOOT_RECORD=1`).
- **Telemetry:** Node Manager owns telemetry DB lifecycle (node_boot, retention, vacuum, shutdown event); Worker API must skip node_boot when started by Node Manager.
- **Secure store:** [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary) already defines behavior when "Node Manager and Worker API run in the same process" (trusted boundary) vs "separate processes" (must enforce and document boundary).
- **Controlled by Node Manager:** Registration, config fetch, config apply, starting Ollama and managed services (PMA); Worker API is started after config fetch and must receive bearer token and state dir from Node Manager.
- **Deployment:** systemd/launchd assume a Node Manager process that starts Worker API (e.g. repo `just start`; setup-dev runs node-manager binary which starts worker-api).

## Goals and Rationale

- **Simplify host deployment:** One binary to build, ship, and run; one process to monitor and restart.
- **Simplify code paths:** No subprocess spawn, no env forwarding, no `NODE_SKIP_NODE_BOOT_RECORD`; single telemetry lifecycle and single secure-store boundary (same process).
- **Codify deployment model:** Requirements and tech spec explicitly state supported deployment topologies and single-process startup/shutdown behavior so implementations and operators have a clear contract.
- **Preserve flexibility:** Retain Worker API as container (`NODE_MANAGER_WORKER_API_IMAGE`) for environments that need Worker API isolated or in a different image; inference-proxy remains a separate component in all cases.

## Proposed Requirements

The following requirement entries are proposed for addition to `docs/requirements/worker.md`.
Requirement IDs are placeholders (REQ-WORKER-0XXX); the actual numbers are assigned when the proposal is merged.

### Deployment Topologies

- **REQ-WORKER-0XXX (proposed):** The worker node MUST support at least one of the following deployment topologies for the Node Manager and Worker API:
  - **Single-process (host binary):** Node Manager and Worker API run in one process; one binary, one system service.
  - **Separate processes (host):** Node Manager runs as one process and starts Worker API as a separate process (same host, same or different binary).
  - **Worker API as container:** Node Manager runs on the host and starts Worker API as a managed container (e.g. when `NODE_MANAGER_WORKER_API_IMAGE` or equivalent is set).
  [CYNAI.WORKER.DeploymentTopologies] (proposed; to be added to [worker_node.md](../tech_specs/worker_node.md)).
  When merged, add a requirement anchor per requirements doc format.

- **REQ-WORKER-0XXX (proposed):** For the single-process (host binary) topology, the Node Manager and Worker API MUST share one process boundary; secure store and telemetry lifecycle MUST follow the same-process behavior defined in [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary) and node-manager-owned telemetry (node_boot, retention, vacuum, shutdown event).
  [CYNAI.WORKER.SingleProcessHostBinary] (proposed; to be added to [worker_node.md](../tech_specs/worker_node.md)).
  When merged, add a requirement anchor per requirements doc format.

### Single Binary as Default for Host

- **REQ-WORKER-0XXX (proposed):** For host binary deployment (non-containerized), the implementation MUST provide a single binary that runs both Node Manager and Worker API in one process as the default and recommended way to run the worker node on the host; the orchestrator-facing behavior (registration, config, capability reporting, Worker API contract) MUST be unchanged from the two-process deployment.
  [CYNAI.WORKER.SingleProcessHostBinary] (proposed; to be added to [worker_node.md](../tech_specs/worker_node.md)).
  When merged, add a requirement anchor per requirements doc format.

## Proposed Tech Spec Content

The following spec content is proposed for addition to `docs/tech_specs/worker_node.md`, in accordance with [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).
It is prescriptive, specific, and explicit so that implementers can verify compliance without inferring intent.

### 1. Deployment Topologies (New Section)

Insert after **Deployment and Auto-Start** (or as a new subsection under it) in `worker_node.md`.

#### 1.1 Deployment Topologies Rule

- Spec ID: `CYNAI.WORKER.DeploymentTopologies` <a id="spec-cynai-worker-deploymenttopologies"></a>

Traces To:

- REQ-WORKER-0XXX (deployment topologies; proposed)

##### 1.1.1 `CYNAI.WORKER.DeploymentTopologies` Scope

- The worker node supports exactly three deployment topologies for the Node Manager and Worker API.
- At least one topology MUST be supported by any conforming implementation.
- The implementation MUST document which topology or topologies it supports and how to select among them (e.g. environment variable, flag, or default).

##### 1.1.2 `CYNAI.WORKER.DeploymentTopologies` Outcomes

- **Single-process (host binary):** Node Manager and Worker API run in one process; one binary, one system service.
  The Worker API HTTP server is started in-process by the Node Manager after configuration is applied; no separate Worker API process is spawned.
- **Separate processes (host):** Node Manager runs as one process and starts Worker API as a separate process (e.g. via exec of the same or a different binary on the same host).
  The trusted boundary for secure store and telemetry MUST be enforced and documented per [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary).
- **Worker API as container:** Node Manager runs on the host and starts Worker API as a managed container (e.g. when `NODE_MANAGER_WORKER_API_IMAGE` or equivalent is set).
  Container lifecycle and identity follow [CYNAI.WORKER.ManagedServiceContainers](../tech_specs/worker_node.md#spec-cynai-worker-managedservicecontainers).

##### 1.1.3 `CYNAI.WORKER.DeploymentTopologies` Error Conditions

- If the implementation cannot satisfy the selected topology (e.g. binary not found for separate-process, image not available for container), it MUST fail startup with a clear error and non-zero exit code.

### 2. Single-Process Host Binary (New Spec Item)

Insert as a new section under the same **Deployment and Auto-Start** (or **Deployment topologies**) area.

#### 2.1 Single-Process Host Binary Rule

- Spec ID: `CYNAI.WORKER.SingleProcessHostBinary` <a id="spec-cynai-worker-singleprocesshostbinary"></a>

Traces To:

- REQ-WORKER-0XXX (single-process boundary; proposed)
- REQ-WORKER-0XXX (single binary default; proposed)
- [REQ-WORKER-0172](../requirements/worker.md#req-worker-0172) (secure store boundary when separate processes)
- [CYNAI.WORKER.NodeStartupProcedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
- [CYNAI.WORKER.NodeManagerShutdown](../tech_specs/worker_node.md#spec-cynai-worker-nodemanagershutdown)

This rule defines the required behavior when the worker node is run in the single-process (host binary) topology: one process runs both Node Manager and Worker API.

##### 2.1.1 `CYNAI.WORKER.SingleProcessHostBinary` Scope

- Applies when the deployment topology is single-process (host binary).
- The same process MUST perform: node registration, config fetch, config apply, telemetry DB lifecycle (node_boot, retention, vacuum, shutdown event), secure store writes (config apply) and reads (worker proxy), and Worker API HTTP server (healthz, jobs, telemetry, managed-service proxy, etc.).
- The Worker API HTTP server MUST be started in the same process as the Node Manager after configuration is applied; the implementation MUST NOT spawn a separate Worker API process for this topology.

##### 2.1.2 `CYNAI.WORKER.SingleProcessHostBinary` Preconditions

- Node startup YAML (or equivalent) and environment are loaded; orchestrator URL, node identity, and registration PSK are available.
- Container runtime (Podman or Docker) is available when the node is configured for sandbox or managed services.

##### 2.1.3 `CYNAI.WORKER.SingleProcessHostBinary` Outcomes

- The single process MUST open and own the telemetry SQLite database; MUST perform node_boot insert once per process start; MUST run retention and vacuum per [Worker Telemetry API](../tech_specs/worker_telemetry_api.md) spec; MUST record node manager shutdown on exit.
- The single process MUST apply node configuration (secure store writes) and MUST serve Worker API endpoints including the internal proxy that reads from the secure store; the process boundary is the trusted boundary per [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary).
- Worker API listen address and port (e.g. `0.0.0.0:12090`) MUST be taken from node startup YAML or environment; the same process MUST bind and serve the Worker API and internal proxy (e.g. UDS for managed agents) until shutdown.
- On shutdown (SIGTERM, SIGINT, or systemd stop), the process MUST follow [CYNAI.WORKER.NodeManagerShutdown](../tech_specs/worker_node.md#spec-cynai-worker-nodemanagershutdown): stop all managed containers and sandbox containers, then exit; the Worker API HTTP server MUST stop accepting new requests and MUST drain or close in coordination with that shutdown.

##### 2.1.4 `CYNAI.WORKER.SingleProcessHostBinary` Algorithm

<a id="algo-cynai-worker-singleprocesshostbinary-startup"></a>

1. Start the single process and load node startup YAML and environment. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-1"></a>
2. Open telemetry store (create directory if needed); run retention on startup; insert node_boot once; start background retention and vacuum goroutines per telemetry spec. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-2"></a>
3. Perform node startup checks (container runtime, sandbox mount root if applicable) per [Node Startup Checks and Readiness](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupchecks). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-3"></a>
4. Register with orchestrator and send capability report; obtain bootstrap data (JWT, report URL, config URL). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-4"></a>
5. Fetch node configuration from orchestrator. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-5"></a>
6. Apply configuration: resolve and write secrets to secure store; apply worker proxy config. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-6"></a>
7. Start the Worker API HTTP server in the same process (bind to configured listen address/port and optional internal UDS); do not spawn a separate process. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-7"></a>
8. Start local inference container (Ollama) only when no existing host inference is detected and config instructs, per [Node Startup Procedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure). <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-8"></a>
9. Start orchestrator-directed managed service containers (e.g. PMA) per config. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-9"></a>
10. Send config ack to orchestrator; run capability reporting loop until shutdown. <a id="algo-cynai-worker-singleprocesshostbinary-startup-step-10"></a>

The algorithm above defines the required startup order for the single-process topology and extends the [Node Startup Procedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure) by specifying that the Worker API is started in-process (step 7).

##### 2.1.5 `CYNAI.WORKER.SingleProcessHostBinary` Error Conditions

- If telemetry store cannot be opened or node_boot fails, the process MUST log the error and MAY continue without telemetry or MUST exit with non-zero exit code depending on implementation policy; the spec does not mandate exit for telemetry failure.
- If registration, config fetch, or config apply fails, the process MUST exit with non-zero exit code.
- If the Worker API HTTP server fails to bind or start (e.g. port in use), the process MUST exit with non-zero exit code.
- If startup checks fail, the process MUST NOT report ready and MUST exit or retry per [Node Startup Checks and Readiness](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupchecks).

##### 2.1.6 `CYNAI.WORKER.SingleProcessHostBinary` Observability

- The single process MUST emit logs with a source identifier (e.g. `node_manager` or a unified node source) so that telemetry and logs can attribute events to the node.
- Shutdown MUST be recorded in the telemetry store per [CYNAI.WORKER.TelemetryLifecycleEvents](../tech_specs/worker_telemetry_api.md#spec-cynai-worker-telemetrylifecycleevents).

### 3. Binary Name and Invocation (Informational)

- The implementation MUST document the single binary name (e.g. `cynodeai-wnm` or `cynodeai-node`) and invocation for host deployment.
- When Worker API is run in-process, the same binary is invoked once; no second binary or subcommand is required for normal operation.
- Selection between single-process and Worker API as container is implementation-defined (e.g. when `NODE_MANAGER_WORKER_API_IMAGE` is set, use container topology; otherwise use single-process).

## Backward Compatibility and Migration

- The previous two-binary deployment (Node Manager execs Worker API subprocess) MAY be supported behind a flag or environment variable (e.g. `CYNODE_LEGACY_WORKER_API=external` or `NODE_MANAGER_WORKER_API_BIN` set to a path) for a transition period.
- When supported, the default for host binary deployment SHOULD be single-process; E2E and setup-dev SHOULD use the single binary when running the node as binaries.
- When the legacy two-process mode is removed, requirements and spec SHOULD be updated to drop the "separate processes (host)" topology from the list of supported topologies if no longer implemented.

## Traceability

- [REQ-WORKER-0172](../requirements/worker.md#req-worker-0172) (secure store boundary when separate processes); [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary).
- [CYNAI.WORKER.NodeManagerShutdown](../tech_specs/worker_node.md#spec-cynai-worker-nodemanagershutdown), [CYNAI.WORKER.NodeStartupProcedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure).
- Worker API contract: [worker_api.md](../tech_specs/worker_api.md); Worker node payloads: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).
- Worker Telemetry API: [worker_telemetry_api.md](../tech_specs/worker_telemetry_api.md).

## Open Points

- **Binary naming:** Keep `cynodeai-wnm` for the combined binary or introduce `cynodeai-node` (or other) and deprecate the old names; document in implementation.
- **Optional modes:** Whether to support "manager-only" or "worker-api-only" in the same binary for debugging or tests; if supported, document as non-default and ensure secure store boundary is documented when run as separate processes.
- **Requirement ID assignment:** When merging this proposal into `docs/requirements/worker.md`, assign stable REQ-WORKER-0XXX IDs and add the requirement anchors; update this proposal's cross-references to use the final IDs.

## References

- [worker_node.md](../tech_specs/worker_node.md) (Node Manager, Secure Store Process Boundary, Node Startup Procedure, Deployment and Auto-Start).
- [worker_node/README.md](../../worker_node/README.md) (current two-cmd layout).
- [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md)
- [node_manager_restart_and_pma_redeploy_spec_proposal.md](node_manager_restart_and_pma_redeploy_spec_proposal.md) (independent node restart; single binary simplifies "restart node" to one process).
- [development_setup.md](../development_setup.md) (node-manager and worker-api in dev).
