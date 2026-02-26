# E2E and CI Status (2026-02-26)

## Summary

- **`just ci`**: Passes (lint, coverage >=90%, BDD, vulncheck, docs-check, containerfiles).
- **`just e2e`**: Fails in current run due to **Ollama model pull** (`registry.ollama.ai` i/o timeout), not due to control-plane or node startup.

## Changes Made

The following updates were made so E2E stack startup and node registration succeed.

### Setup Script (`scripts/setup-dev.sh`)

- Added `wait_for_control_plane_listening()`: polls `http://127.0.0.1:12082/readyz` until response is 200 or 503 (server listening).
  Accepting 503 avoids chicken-and-egg (readyz 200 requires a registered node).
- In `full-demo`, replaced fixed `sleep 2` before `start_node` with `wait_for_control_plane_listening` so the node is not started until the control-plane accepts connections.

### Compose File (`orchestrator/docker-compose.yml`)

- Set `PMA_ENABLED: "false"` for the control-plane service.
  In compose, cynode-pma runs as a separate container; the control-plane was exiting with "exec: cynode-pma: executable file not found in $PATH" when it tried to start PMA as a subprocess.

## E2E Result

- Stack starts; control-plane stays up; wait passes; node-manager starts and registers.
- Failure occurs later when the E2E script pulls the Ollama model (tinyllama) from `registry.ollama.ai` (network timeout in this environment).
- To get a full E2E pass: run in an environment with outbound access to registry.ollama.ai, or pre-pull the model (e.g. `podman exec cynodeai-ollama ollama pull tinyllama`) before running the E2E test steps.

## MVP Plan Continuation

Per docs/mvp_plan.md, **Remaining (Order)**: Phase 2 (P2-01, P2-03, full P2-02 allow path; P2-09, P2-10; LangGraph P2-04--P2-08), then Phase 3, Phase 4.
