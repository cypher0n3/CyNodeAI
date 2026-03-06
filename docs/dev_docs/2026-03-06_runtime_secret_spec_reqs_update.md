# Runtime/Secret Spec and Requirements Update

## Metadata

- Date: 2026-03-06
- Scope: All specs and requirements that reference secrets, credentials, or tokens in Go code.
- Purpose: Require use of Go 1.26 `runtime/secret` (when available) for such code.

## Summary

A single cross-cutting requirement **REQ-STANDS-0133** was added, and all relevant specs/reqs were updated to require or reference it.

## Changes

Edits are listed by category below.

### New Requirement

- **docs/requirements/stands.md:** Added **REQ-STANDS-0133:** Go code that handles secrets, credentials, or tokens (including master keys and decrypted plaintext) MUST use `runtime/secret` (Go 1.26, via `GOEXPERIMENT=secret`) when available; when not available, MUST use best-effort secure erasure.

### New Spec Section

- **docs/tech_specs/go_rest_api_standards.md:** Added section **Secret Handling in Go** (CYNAI.STANDS.SecretHandling) tracing to REQ-STANDS-0133.

### Requirements Updated (Reference REQ-STANDS-0133)

- **docs/requirements/worker.md:** REQ-WORKER-0102 (tokens), REQ-WORKER-0165 (secure store), REQ-WORKER-0166 (master key), REQ-WORKER-0201 (telemetry tokens).
- **docs/requirements/apiegr.md:** REQ-APIEGR-0108 (decrypt credentials).
- **docs/requirements/client.md:** REQ-CLIENT-0103 (secrets), REQ-CLIENT-0105 (tokens).

### Tech Specs Updated

- **docs/tech_specs/worker_node.md:** Secure store section: worker MUST use `runtime/secret` when available (was SHOULD); added trace to REQ-STANDS-0133.
- **docs/tech_specs/api_egress_server.md:** Credential Storage: Go code that decrypts or holds credential plaintext MUST use runtime/secret per REQ-STANDS-0133.
- **docs/tech_specs/cynork_cli.md:** Implementation spec traces to REQ-STANDS-0133; Secrets and logging section: Go code handling token/secret MUST use runtime/secret per REQ-STANDS-0133.
- **docs/tech_specs/git_egress_mcp.md:** Credential Storage: Go code that retrieves or decrypts Git credentials MUST use runtime/secret per REQ-STANDS-0133.

## Validation

- `just docs-check` (markdownlint, link check, requirements validation, feature file validation): PASS.
