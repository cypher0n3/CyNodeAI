# Phase 4 Validation: Node-Local Secure Store and Agent Token Lifecycle

- [Metadata](#metadata)
- [Spec and Requirement IDs](#spec-and-requirement-ids)
- [Acceptance Criterion -> Coverage](#acceptance-criterion---coverage)
- [Feature File and Tags](#feature-file-and-tags)
- [BDD Step Status](#bdd-step-status)
- [Test Inventory (Phase 4)](#test-inventory-phase-4)
- [Sign-Off](#sign-off)

## Metadata

- **Date:** 2026-03-06
- **Scope:** Phase 4 of worker proxy spec reconciliation (node-local secure store, agent token lifecycle, process boundary, FIPS).
- **Purpose:** Map acceptance criteria to specs/requirements, feature scenarios, and tests.

## Spec and Requirement IDs

- **Spec / Req:** CYNAI.WORKER.NodeLocalSecureStore
  - description: Encrypted-at-rest store under `<state_dir>/secrets`, master key precedence, no plaintext key on disk.
- **Spec / Req:** CYNAI.WORKER.AgentTokenStorageAndLifecycle
  - description: Tokens stored in secure store; never passed to agent/managed-service containers.
- **Spec / Req:** CYNAI.WORKER.SecureStoreProcessBoundary
  - description: Document writer (Node Manager) vs reader (Worker API internal proxy); secrets path never in containers.
- **Spec / Req:** REQ-WORKER-0165
  - description: Store orchestrator-issued secrets in node-local secure store; encrypt at rest.
- **Spec / Req:** REQ-WORKER-0166
  - description: Master key MUST NOT be stored in plaintext on disk or written to logs.
- **Spec / Req:** REQ-WORKER-0167
  - description: Master key precedence; env fallback supported; startup warning when using env fallback.
- **Spec / Req:** REQ-WORKER-0168
  - description: MUST NOT mount or expose secure store into sandbox or managed-service containers.
- **Spec / Req:** REQ-WORKER-0169
  - description: Secure store distinct from telemetry SQLite; not exposed by any API.
- **Spec / Req:** REQ-WORKER-0170
  - description: FIPS mode: FIPS-approved algorithms only; reject env key when FIPS on/unknown.
- **Spec / Req:** REQ-WORKER-0171
  - description: Resolve `agent_token_ref` on config apply; fail closed on failure; never expose ref/token to containers.
- **Spec / Req:** REQ-WORKER-0172
  - description: Trusted boundary when Node Manager and Worker API are separate; document writer vs reader.

## Acceptance Criterion -> Coverage

- **Criterion:** Secrets stored under `<state_dir>/secrets`, encrypted at rest
  - spec/req: 0165, NodeLocalSecureStore
  - feature scenario: Worker stores orchestrator-issued secrets encrypted at rest
  - unit / functional test: `TestPutAgentToken_EncryptedAtRest`, BDD steps (securestore in-process)
- **Criterion:** Master key not on disk / not in logs
  - spec/req: 0166
  - feature scenario: (implied by store design)
  - unit / functional test: Store uses env/TPM/OS/system only; no key file; code review
- **Criterion:** Master key precedence; env fallback; startup warning
  - spec/req: 0167
  - feature scenario: Worker warns when using env var master key fallback
  - unit / functional test: `TestOpen_EnvMasterKey`, `TestOpen_SystemCredentialPreferred`, FIPS tests; BDD env-warning step
- **Criterion:** No secure store mount in containers
  - spec/req: 0168
  - feature scenario: (new) No secure store mount in managed-service containers
  - unit / functional test: `TestBuildManagedServiceRunArgs_NoSecretsMount`, BDD step (run-args assertion)
- **Criterion:** Secure store distinct from telemetry; not in API
  - spec/req: 0169
  - feature scenario: (new) Secure store distinct from telemetry and not exposed by API
  - unit / functional test: Design: separate dirs and no telemetry routes to secrets; unit tests for store path
- **Criterion:** FIPS: approved algo only; reject env when FIPS on
  - spec/req: 0170
  - feature scenario: (new) FIPS mode rejects env master key
  - unit / functional test: `TestOpen_FIPSModeRejectsEnvFallback`, `TestOpen_FIPSModeEnvOverride_*`, `TestOpen_FIPSModeUnknownFailClosed`
- **Criterion:** Resolve agent_token_ref; fail closed; never expose to container
  - spec/req: 0171
  - feature scenario: Worker holds agent token and does not pass it to managed-service containers
  - unit / functional test: `TestResolveAgentTokenRef_*`, `TestSyncManagedServiceAgentTokens_*`; BDD no-token-in-container step
- **Criterion:** Process boundary documented; writer vs reader
  - spec/req: 0172
  - feature scenario: (new) Process boundary documented
  - unit / functional test: `docs/dev_docs/2026-03-06_secure_store_process_boundary.md`; `TestBuildManagedServiceRunArgs_NoSecretsMount`

## Feature File and Tags

- **File:** `features/worker_node/worker_secure_store.feature`
- **Scenarios:** REQ-WORKER-0165 (encrypted at rest), 0164/0167 (agent token not in container; env warning), plus scenarios added for 0168, 0169, 0170, 0171, 0172 where applicable.

## BDD Step Status

- **Implemented:** Steps that use in-process secure store (store under state_dir, encrypted at rest), and run-args assertion for no-secrets mount (via nodemanager run-args helper).
- **Env warning / proxy attaches token:** Covered by unit tests and integration; BDD steps assert or mark coverage.

## Test Inventory (Phase 4)

- **Test:** `securestore/store_test.go`: `TestOpen_EnvMasterKey`, `TestOpen_InvalidMasterKey`, `TestOpen_NoMasterKey`
  - what it covers: Master key resolution, env fallback.
- **Test:** `TestOpen_FIPSModeRejectsEnvFallback`, `TestOpen_FIPSModeEnvOverride_*`, `TestOpen_FIPSModeUnknownFailClosed`
  - what it covers: FIPS fail-closed, env override.
- **Test:** `TestOpen_SystemCredentialPreferred`
  - what it covers: Precedence (system credential over env).
- **Test:** `TestPutAgentToken_EncryptedAtRest`
  - what it covers: Ciphertext on disk; not plaintext JSON.
- **Test:** `TestGetAgentToken_*`, `TestPutAgentToken_InvalidInputs`, envelope/decode tests
  - what it covers: Lifecycle and validation.
- **Test:** `nodemanager/nodemanager_test.go`: `TestSyncManagedServiceAgentTokens_*`, `TestResolveAgentTokenRef_*`
  - what it covers: Config-apply token write/rotate/delete; ref resolution.
- **Test:** `cmd/node-manager/main_test.go`: `TestBuildManagedServiceRunArgs_NoSecretsMount`
  - what it covers: Run args never mount secrets path.

## Sign-Off

- Phase 4 implementation: completed (reconciliation plan checklist).
- This validation doc: maps criteria to specs, requirements, feature scenarios, and tests; BDD steps added for secure store and no-mount; additional feature scenarios added for 0168-0172 where needed.
