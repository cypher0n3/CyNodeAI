# Task 12 Completion - PMA Keep-Warm, Secret Scan, Overwrite NDJSON (REQ-PMAGNT-0124/0125/0129)

- [Summary](#summary)
- [Files](#files)
- [Validation](#validation)
- [Deviations](#deviations)

## Summary

- **Keep-warm (0129):** `StartKeepWarm` runs after HTTP serve in `agents/cmd/cynode-pma/main.go`.
  Node-local backends only (`localhost`, `127.0.0.1`, `http+unix:`).
  Interval from `PMA_KEEP_WARM_INTERVAL_SEC` (default **300**).
  Disable with `PMA_DISABLE_KEEP_WARM=1`.
  Initial ping plus `time.Ticker` loop until process context ends; ping uses minimal non-streaming `/api/chat`.
- **Secret scan + overwrite (0125/0124):** `redactKnownSecrets` detects OpenAI-style `sk-…` and `Bearer …` tokens.
  `writeLangchainNDJSONStream`, Ollama NDJSON scan path, and `streamingLLM` emit redacted stream text and, when raw buffers still contain detectable secrets, an `{"overwrite":{...}}` NDJSON line (`reason: secret_redaction`, `scope: iteration` where applicable).
- **Wire format:** `encodeOverwriteNDJSON` in `overwrite_ndjson.go`; turn scope omits `iteration`.

## Files

- `agents/internal/pma/keepwarm.go`, `keepwarm_test.go`
- `agents/internal/pma/secret_scan.go`, `secret_scan_test.go`
- `agents/internal/pma/overwrite_ndjson.go`
- `agents/internal/pma/langchain.go`, `chat.go`, `streaming.go`
- `agents/cmd/cynode-pma/main.go`

## Validation

- `go test -cover ./agents/...` - `internal/pma` >= **90%** (required for `just test-go-cover`).
- `just lint-go` - pass.
- BDD tags `@req_PMAGNT_0124` etc.: not verified in this pass (same constraint as other tasks when tags absent in `features/`).

## Deviations

- Full `secret.Do` wrapping (REQ-PMAGNT-0126) not added; scan/redact remains best-effort at string level.
- `just e2e --tags pma_inference,streaming` was run after `just setup-dev restart` (2026-03-30); plan item `imm-214` **completed**.
