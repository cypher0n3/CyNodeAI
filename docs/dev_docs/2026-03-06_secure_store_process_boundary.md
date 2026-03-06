# Secure Store Process Boundary

- [Metadata](#metadata)
- [Summary](#summary)
- [Writer](#writer)
- [Reader](#reader)
- [Same-Process Deployment](#same-process-deployment)
- [Split-Process Deployment](#split-process-deployment)
- [FIPS Mode](#fips-mode)
- [Container Boundary](#container-boundary)

## Metadata

- Date: 2026-03-06
- Spec: [CYNAI.WORKER.SecureStoreProcessBoundary](../tech_specs/worker_node.md#spec-cynai-worker-securestoreprocessboundary)
- Requirement: [REQ-WORKER-0172](../requirements/worker.md#req-worker-0172)

## Summary

The node-local secure store holds orchestrator-issued secrets (e.g. agent tokens).
This doc states which component writes it, which reads it, and how the trusted boundary is enforced.

## Writer

- **Component:** Node Manager (config-apply path).
- **When:** On applying node configuration that includes `managed_services.services[]` with `orchestrator.agent_token` or `agent_token_ref`.
  Node Manager resolves tokens (including `agent_token_ref` when present), then calls the secure store to write/rotate/delete per `service_id`.
- **Where:** Writes go under `<state_dir>/secrets/agent_tokens/` (encrypted at rest).
  Node Manager and Worker API share the same `state_dir` (from env or default).

## Reader

- **Component:** Worker API internal proxy handler.
- **When:** On each request to `POST /v1/worker/internal/orchestrator/mcp:call` or `POST /v1/worker/internal/orchestrator/agent:ready`, the handler resolves caller identity from the identity-bound transport (per-service UDS), then loads the token for that `service_id` from the secure store and attaches it to the outbound request.
  No token is read from the request or from any agent container.
- **Where:** Reads from the same `<state_dir>/secrets/` store.

## Same-Process Deployment

When Node Manager and Worker API run in the same process (e.g. single binary or process group), that process is the trusted boundary.
Only that process has access to the decrypted master key and the store; no separate enforcement is required beyond file permissions (0600/0700) and not mounting the store into containers.

## Split-Process Deployment

When Node Manager and Worker API run as separate processes:

- **Enforcement:** Both use the same `state_dir`.
  The secure store directory and files are created with 0700 (dirs) and 0600 (files).
  Only the same user/group that runs Node Manager and Worker API should have access.
  The implementation does not pass the master key between processes; each process resolves the master key independently (TPM, OS key store, system credential, or env).
  So both processes must be configured with the same master key source (e.g. same systemd credential or same env in their respective units).
- **Documentation:** This doc satisfies the requirement to document which component writes and which reads.
  The trusted boundary is enforced by (1) only Node Manager applying config and writing tokens, (2) only Worker API internal proxy reading tokens for proxying, (3) no path under `state_dir/secrets/` being mounted into any managed-service or sandbox container (see worker_node.md Agent-To-Orchestrator UDS Binding; only `state_dir/run/managed_agent_proxy/<service_id>` is mounted).

## FIPS Mode

When the host is in FIPS mode (or FIPS status cannot be determined), the secure store rejects master key from env (`CYNODE_SECURE_STORE_MASTER_KEY_B64`).
Detection: Linux `/proc/sys/crypto/fips_enabled`, Windows `HKLM\...\Lsa\FipsAlgorithmPolicy\Enabled`.
On macOS and other platforms where we cannot detect, we **fail closed** (treat as FIPS on).
Operators can set `CYNODE_FIPS_MODE=1` or `CYNODE_FIPS_MODE=0` to override explicitly (e.g. on Mac set `0` to allow env fallback in non-FIPS deployments).

## Container Boundary

Managed-service containers MUST NOT have access to the secure store.
Node Manager mounts only `<state_dir>/run/managed_agent_proxy/<service_id>` at `/run/cynode/managed_agent_proxy` for each managed service.
It never mounts `<state_dir>/secrets/` or any path under it.
Unit tests assert that the generated container run args contain no volume mount whose host path contains `secrets`.
