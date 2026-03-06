# Phase 4a Validation: Post-Quantum KEM for Secure Store Encryption at Rest

- [Metadata](#metadata)
- [Spec and Requirement IDs](#spec-and-requirement-ids)
- [Acceptance Criteria and Coverage](#acceptance-criteria-and-coverage)
- [Test Inventory](#test-inventory)
- [Sign-Off](#sign-off)

## Metadata

- **Date:** 2026-03-06
- **Scope:** Phase 4a of [worker proxy spec reconciliation plan](2026-03-05_worker_proxy_spec_reconciliation_plan.md): post-quantum KEM (ML-KEM-768) by default, AES-256-GCM-only fallback when PQ not permitted.
- **Purpose:** Confirm Phase 4a implementation and tests satisfy REQ-WORKER-0173 and NodeLocalSecureStore encryption-at-rest.

## Spec and Requirement IDs

- **Spec:** CYNAI.WORKER.NodeLocalSecureStore (encryption at rest bullets)
  - Default: post-quantum KEM to protect key material; strong symmetric AEAD for ciphertext; per-record nonce.
  - Fallback: FIPS-approved symmetric AEAD only when PQ not available or not permitted.
- **Req:** REQ-WORKER-0173
  - Encryption at rest MUST use post-quantum KEM when permitted; MUST fall back to FIPS-approved symmetric AEAD when not.

## Acceptance Criteria and Coverage

- **Default path uses PQ KEM + AES-256-GCM; per-record nonce**
  - Implementation: `worker_node/internal/securestore/store.go` envelope version 2, algorithm `ML-KEM-768+AES-256-GCM`; `encryptPQ` / `decryptPQ`; KEM keystore `.kem_keystore.enc` (v1 envelope) holds ML-KEM decapsulation key encrypted under master key.
  - Tests: `TestPutGetAgentToken_PQPath`, `TestGetAgentToken_LoadsKEMKeyFromFile`, `TestPutGetDeleteAgentToken` (when FIPS off).

- **Fallback when PQ not permitted (e.g. FIPS mode)**
  - Implementation: `isPQPermitted()` returns `!isFIPSMode()`; when false, `buildEncryptedEnvelope` and decrypt use v1 AEAD-only.
  - Tests: `TestPutGetAgentToken_AEADOnlyFallback` (FIPS file `1`, system credential key; Put/Get use v1 envelope).

- **Backward compatibility for existing v1 envelopes**
  - Implementation: `decryptEnvelope` accepts version 1 + AES-256-GCM and version 2 + ML-KEM-768+AES-256-GCM.
  - Tests: `TestGetAgentToken_ReadsV1Envelope`.

- **Envelope version/algorithm id for PQ vs AEAD-only**
  - Implementation: `encryptedEnvelope.Version` 1 (AEAD-only) and 2 (PQ); `Algorithm` distinguishes; v2 includes `kem_ciphertext_b64`.
  - Tests: `TestGetAgentToken_EnvelopeDecodeFailures` (unsupported version 99, v2 missing kem), `TestGetAgentToken_V2InvalidKEMCiphertext`, `TestGetAgentToken_CorruptKEMKeystore`, `TestGetAgentToken_V2WrongKey`, `TestGetAgentToken_UnsupportedEnvelopeAlgorithm`.

## Test Inventory

- **PQ path:** `TestPutGetAgentToken_PQPath`, `TestGetAgentToken_LoadsKEMKeyFromFile`, `TestPutAgentToken_EncryptedAtRest` (PQ when FIPS off).
- **AEAD-only fallback:** `TestPutGetAgentToken_AEADOnlyFallback`.
- **Backward compat:** `TestGetAgentToken_ReadsV1Envelope`.
- **Error / invalid envelope:** `TestGetAgentToken_EnvelopeDecodeFailures`, `TestGetAgentToken_V2InvalidKEMCiphertext`, `TestGetAgentToken_CorruptKEMKeystore`, `TestGetAgentToken_V2WrongKey`, `TestGetAgentToken_UnsupportedEnvelopeAlgorithm`.

Unit test package: `worker_node/internal/securestore`.
Coverage meets repo minimum for securestore (86%).

## Sign-Off

- Phase 4a change set implemented and tested.
- `just ci` passes (lint, tests, coverage, BDD).
- Reconciliation plan Phase 4a checklist updated to completed.
