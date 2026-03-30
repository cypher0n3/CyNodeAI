# Task 6 Completion Report - Encrypt `worker_api_bearer_token` at Rest

## Summary

`worker_api_bearer_token` is encrypted with **AES-GCM** using a key derived from the orchestrator **`JWTSecret`** (`orchestrator/internal/fieldcrypt/worker_bearer.go`).

The database layer encrypts before persist and decrypts after load (`orchestrator/internal/database/` node helpers), with **`ApplyWorkerBearerEncryptionAtStartup`** configuring the field key and **`MigratePlaintextWorkerBearerTokens`** re-encrypting legacy plaintext rows on upgrade.

Binary entrypoints that open the orchestrator DB call **`ApplyWorkerBearerEncryptionAtStartup`** during startup (`user-gateway`, `api-egress`, `control-plane` when using `*database.DB`, `mcp-gateway`); BDD steps use the same helper.

## Validation

- `go test -v -run TestTokenEncryption ./orchestrator/internal/handlers/...`
- `just lint-go` and `just test-go-cover` (orchestrator packages meet configured thresholds)
- `just e2e --tags worker,no_inference`

## Plan

YAML `st-057`-`st-068` and Task 6 markdown checklists marked completed in `docs/dev_docs/_plan_003_short_term.md`.
