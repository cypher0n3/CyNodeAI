# Worker Node Secure Store: Decided Design (Prescriptive)

## Purpose

This document defines the **decided** design for how the worker holds orchestrator-issued secrets (agent tokens, pull credentials, orchestrator bearer token) securely.
It is written in prescriptive language for inclusion in or traceability from the tech spec ([spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md)).

Traces To: [REQ-WORKER-0164](../requirements/worker.md#req-worker-0164), [REQ-WORKER-0128](../requirements/worker.md#req-worker-0128), [REQ-WORKER-0132](../requirements/worker.md#req-worker-0132).

## Decided Path: Envelope Encryption + File-Based Ciphertext Store

The node-local secure store SHALL be implemented as follows.

### Scope and Boundaries

- **Node-local** means: the store lives on the host where the Node Manager and worker run; it SHALL NOT be inside any sandbox or managed-service container; it SHALL NOT be on a remote volume or network share unless the spec explicitly defines that deployment option.
- The store holds **orchestrator-issued secrets** for this node: pull credentials, orchestrator bearer token, agent tokens (and any capability leases).
  Agent tokens SHALL be stored in the same store under a logical namespace keyed by managed-service identity (e.g. `service_id`).
- **No mount into containers:** No path, filesystem partition, or volume that is part of the secure store SHALL be mounted into any sandbox or managed-service container.
  The store SHALL remain host-only and SHALL NOT be accessible from inside containers.
- **Distinct from telemetry DB:** The secure store SHALL NOT be the same database or file as the Worker Telemetry API SQLite database (`telemetry/telemetry.db`).
  No API (including the Worker Telemetry API) SHALL query or expose the secure store.

### Backing: File-Based Ciphertext Store

- **Location:** The worker SHALL store secret ciphertext under the node state directory.
  The path SHALL be `${storage.state_dir}/secrets/` (or `/var/lib/cynode/state/secrets/` when `storage.state_dir` is unset).
  A single file or a directory of files SHALL be used exclusively for secrets; no other component SHALL use this path for non-secret data.
- **Ownership and permissions:** Store files SHALL be owned by the same user or group that runs the Node Manager (or the worker process that requires read access).
  File permissions SHALL be `0600` (owner read/write only).
  If a directory is used, directory permissions SHALL be `0700` so only the owning process can traverse it.
- **Format:** Secret values SHALL be encrypted before being written.
  The worker SHALL use a post-quantum resistant symmetric encryption algorithm (e.g. AES-256-GCM; or a NIST PQC-approved symmetric algorithm when standardized).
  A per-record nonce SHALL be used.
  The master key SHALL NOT be stored in plaintext in any file under the store path or alongside the ciphertext.
- **Persistence:** The store SHALL be durable across process restarts so that after a restart the worker can resume using the same secrets until configuration is refreshed or secrets are removed.

### Go 1.26 and Secure Secret Handling

- The worker SHALL be built with Go 1.26 or later.
- The worker SHALL be built with the Go experiment that enables `runtime/secret` (e.g. `GOEXPERIMENT=secret` at build time).
- The implementation SHALL use the experimental **`runtime/secret`** package (Go 1.26) to wrap any code that touches the master key or decrypted secret plaintext, so that temporaries (registers, stack, heap used during the operation) are securely erased before returning.
  Use `secret.Do(f)` for such operations; do not retain plaintext secrets in goroutine stacks or heap longer than necessary.
- `runtime/secret` is experimental and currently supported on Linux/amd64 and Linux/arm64; on other platforms the implementation SHALL perform best-effort erasure (e.g. zeroing buffers) and SHALL NOT log or retain plaintext.

### FIPS Mode

- When FIPS mode is enabled on the system (e.g. kernel flag, crypto policy, or platform equivalent), the worker SHALL use only FIPS-approved cryptographic algorithms and SHALL use FIPS-validated cryptographic modules where required by the platform.
- The chosen symmetric algorithm (e.g. AES-256-GCM) SHALL be used in a FIPS-approved manner when FIPS mode is enabled; the worker SHALL NOT use non-FIPS algorithms or non-validated modules in that case.

### Encryption at Rest and Master Key

- **Encryption at rest:** All secret values (agent tokens, pull credentials, orchestrator bearer token) SHALL be encrypted before being written to the store and decrypted only when the authorized component (Node Manager for write, worker proxy for read) needs the plaintext.
- **Master key:** The worker SHALL obtain a single 256-bit master key used to encrypt and decrypt secrets.
  The master key itself SHALL NOT be written to disk in plaintext.

**Master key source precedence (highest to lowest).**
The worker SHALL use the first available source:

1. **TPM-sealed key** (when supported and configured).
2. **OS key store** (Linux kernel keyring, Windows DPAPI, macOS Keychain; platform-specific list SHALL be specified in the tech spec).
3. **System service credentials** (e.g. a systemd credential when running under systemd).
4. **Environment variable fallback:** base64-encoded 256-bit key in `CYNODE_SECURE_STORE_MASTER_KEY_B64`.
   This SHALL be used only when all higher-precedence sources are unavailable.

#### Requirements for `CYNODE_SECURE_STORE_MASTER_KEY_B64`

- The decoded value SHALL be exactly 32 bytes (256-bit key).
- The worker SHALL reject invalid base64 or wrong-length values and SHALL fail closed (no secret store access).
- The worker SHALL NOT log the env var value, decoded bytes, or any derived key material.
- The worker SHALL NOT pass this env var into any container.

### Startup Warnings for Less-Secure Master Key Backends

- The worker SHALL emit a startup warning when using a less-secure master-key backend.
  Warnings SHALL NOT include any secret values.
- The worker SHALL warn when:
  - `CYNODE_SECURE_STORE_MASTER_KEY_B64` is used (env var fallback), or
  - The master key is obtained from any source that is not TPM, OS key store, or system service credentials.
- The warning SHALL include the backend name (e.g. `env_b64`, `systemd_credential`, `os_keystore`, `tpm`) and a remediation hint (e.g. "configure OS key store or TPM; avoid env var key fallback").

### Access Control

- **Write:** The Node Manager (or the single process that applies node configuration) SHALL write to the secure store when applying configuration.
  It SHALL NOT log, echo, or expose secret values when writing.
- **Read:** Only the worker component that performs agent-to-orchestrator forwarding (the worker proxy) SHALL read agent tokens from the store.
  No other process (e.g. logging daemon, telemetry, debug endpoints) SHALL read token values.
  If Node Manager and worker proxy are separate processes, they SHALL share a trusted boundary (e.g. same user or documented capability/socket); the spec SHALL state this explicitly.
- **No container access:** No path, env var, or volume that exposes the store or any secret value SHALL be mounted or passed into sandbox or managed-service containers.
  Agent tokens SHALL NOT be passed to agents.

### Agent-Token Keying and Lifecycle

- **Key:** Each agent token SHALL be stored under a key that uniquely identifies the managed service (e.g. `service_id` from `managed_services.services[]`).
  The worker proxy SHALL look up the token by this key when handling agent-originated requests.
- **Write:** On configuration apply, for each managed service that has `orchestrator.agent_token` or resolvable `agent_token_ref`, the worker SHALL write or overwrite the token for that service's key.
  The worker SHALL NOT pass the token to the container or agent.
- **Read:** When the worker proxy handles an agent-originated request, it SHALL determine the service identity, load the corresponding agent token from the store, attach it to the outbound request, and forward.
  It SHALL NOT cache the token in a way visible to other components (e.g. no global log or debug buffer) and SHALL NOT log or expose the token.
- **Delete:** On configuration update that removes a managed service or removes or rotates its token, the worker SHALL remove or overwrite the corresponding key so the old token is no longer available.
- **Expiry:** When an expiry (e.g. `agent_token_expires_at`) is present, the worker SHALL treat an expired token as invalid and SHALL NOT use it when forwarding; the worker SHALL request a configuration refresh where applicable.

### Observability and No-Logging

- Agent tokens SHALL NOT appear in logs, metrics, audit payloads (beyond opaque identifiers such as `service_id` or agent identity), or any debug or telemetry output.
  Redaction SHALL NOT be relied upon; token values SHALL NOT be present in any such data.

## Algorithm (Normative)

1. On node configuration apply, for each `managed_services.services[]` entry that includes `orchestrator.agent_token` or `agent_token_ref`, resolve the token value (if ref, obtain per `agent_token_ref` spec) and store it in the node-local secure store keyed by service identity.
   Do not pass the token to the container or agent.
2. When the worker proxy receives an agent-originated request (e.g. to MCP gateway or callback), determine the requesting service identity, load the corresponding agent token from the secure store, attach it to the outbound request, and forward.
   Do not log or expose the token.
3. On configuration update or removal of a managed service, remove or invalidate that service's agent token from the store.
4. If a token is expired (when expiry is provided), do not use it; treat the request as unauthorized and trigger configuration refresh where applicable.

## Spec Items to Add

- **CYNAI.WORKER.NodeLocalSecureStore** (Rule): adopt the definition above as the single source of truth for the node-local secure store; trace to REQ-WORKER-0128, REQ-WORKER-0132.
- **CYNAI.WORKER.AgentTokenStorageAndLifecycle** (Rule): agent tokens SHALL be stored in the node-local secure store per NodeLocalSecureStore, keyed by service identity; lifecycle and access as above; trace to REQ-WORKER-0164.
  Include an Algorithm subsection with step anchors for requirements/feature traceability.
- **worker_node_payloads.md:** In CYNAI.WORKER.PayloadSecurity (or adjacent), state that `orchestrator.agent_token` and any resolved value from `agent_token_ref` are secrets and SHALL be handled per the agent token storage and lifecycle Rule (link to Spec ID).
