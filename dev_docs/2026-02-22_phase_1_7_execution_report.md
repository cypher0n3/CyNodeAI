# Phase 1.7 Execution Report

- [Summary](#summary)
- [Deliverables](#deliverables)
- [How to run](#how-to-run)
- [Notes](#notes)
- [Validation](#validation)

## Summary

**Date:** 2026-02-22. **Scope:** P1.7-03 (cynode-pma binary and agents module), P1.7-04 (PMA startup in orchestrator).

Phase 1.7 work from `docs/mvp_plan.md` is implemented: the `cynode-pma` agent binary exists in a new `agents/` Go module, has a Containerfile, and the control-plane starts it as a subprocess when enabled.

## Deliverables

Completed items for P1.7-03 and P1.7-04 follow.

### P1.7-03: CyNode-Pma Binary and Agents Module

- **agents/ Go module**
  - `agents/go.mod` - new module `github.com/cypher0n3/cynodeai/agents`.
  - `agents/cmd/cynode-pma/main.go` - binary with flags: `--role` (project_manager | project_analyst), `--instructions-root`, `--instructions-project-manager`, `--instructions-project-analyst`, `--listen`.
    Env overrides: `PMA_ROLE`, `PMA_INSTRUCTIONS_ROOT`, `PMA_LISTEN_ADDR`, etc. Precedence: flag over env.
  - `agents/internal/pma/config.go` - role and instructions path config; `InstructionsPath()` returns role-specific bundle path per spec (default `instructions/project_manager` or `instructions/project_analyst`).
  - `agents/internal/pma/instructions.go` - `LoadInstructions(dir)` reads .md/.txt from a directory for the role bundle.
  - Default layout under `agents/instructions/`: `project_manager/`, `project_analyst/` with placeholder READMEs.
- **Containerfile**
  - `agents/cmd/cynode-pma/Containerfile` - multi-stage build (Alpine), minimal go.work with `./agents` only, copies `agents/instructions` into image.
    Build from repo root: `podman build -f agents/cmd/cynode-pma/Containerfile -t cynodeai-cynode-pma:dev .`
- **go.work**
  - `./agents` added to workspace so `go build ./agents/cmd/cynode-pma` works from root.

### P1.7-04: PMA Startup in Orchestrator

- **Orchestrator config** (`orchestrator/internal/config/config.go`)
  - New fields: `PMAEnabled` (bool, env `PMA_ENABLED`, default false), `PMABinaryPath` (`PMA_BINARY`, default `cynode-pma`), `PMAListenAddr` (`PMA_LISTEN_ADDR`, default `:8090`), `PMAInstructionsRoot` (`PMA_INSTRUCTIONS_ROOT`).
    Added `getBoolEnv()` and tests.
- **PMA subprocess** (`orchestrator/internal/pmasubprocess/`)
  - `Start(cfg, logger)` builds `exec.Cmd` for cynode-pma with `--role=project_manager` and `--listen=...`; returns `(nil, nil)` when `PMAEnabled` is false.
    Caller is responsible for stopping the process (Signal + Wait).
    Unit tests: disabled, missing binary, empty binary (default name), and success with `true` binary.
- **Control-plane** (`orchestrator/cmd/control-plane/main.go`)
  - After starting the HTTP server and dispatcher, if `cfg.PMAEnabled` then `pmasubprocess.Start(cfg, logger)`; on shutdown, defer sends SIGTERM and `Wait()` so cynode-pma exits with the control-plane.
    New test `TestRun_PMAStartedAndStopped` runs with PMA enabled and a quick-exit binary to cover the start/stop path.

## How to Run

- **Build cynode-pma (from repo root)**  
  `go build -o bin/cynode-pma ./agents/cmd/cynode-pma`
- **Run standalone**  
  `./bin/cynode-pma --role=project_manager` (or set `PMA_ROLE`).
    Listens on `:8090` by default; `GET /healthz` returns 200.
- **Run with control-plane**  
  Set `PMA_ENABLED=true` and `PMA_BINARY` to the path of the cynode-pma binary (or leave default if it is in PATH).
    Start the control-plane; it will start cynode-pma and stop it on shutdown.

## Notes

- **Justfile:** The `go_modules` variable does not include `agents`.
  To run agents tests and build from the justfile, add `agents` to `go_modules` and a `build-cynode-pma` recipe when desired.
- **Markdown lint:** `agents/instructions/**` was added to the markdownlint ignores (`.markdownlint-cli2.jsonc`) so placeholder instruction READMEs are not required to satisfy the no-h1-content rule.
- **Doc link:** The broken link in `docs/tech_specs/cynode_sba.md` to `../draft_specs/cynode-agent_rough_spec.md` was replaced with plain text (file not in repo) so `just validate-doc-links` passes.
- **Chat surface:** Implemented post-Phase 1.7: user-gateway now exposes `GET /v1/models` and `POST /v1/chat/completions` with model-based routing (`cynodeai.pm` to cynode-pma, other to direct inference); legacy `POST /v1/chat` removed.

## Validation

- `go build ./agents/cmd/cynode-pma` and `go test ./agents/...` pass.
- `just test-go-cover` passes (orchestrator and pmasubprocess above 90%; mcp-gateway >=90% with testcontainers).
- `just test-bdd` passes.
- `just lint-containerfiles` passes (including `agents/cmd/cynode-pma/Containerfile`).
- `just lint-md` and `just validate-doc-links` pass.
- `just ci` and `just docs-check` pass.

## Post-Phase 1.7 / Current State

- **E2E full-demo:** `just e2e` / `./scripts/setup-dev.sh full-demo` passes end-to-end, including Test 5d (GET /v1/models, POST /v1/chat/completions with model `cynodeai.pm`).
  List-models and chat completion are exercised against the live stack.
- **Compose routing:** user-gateway is configured with `PMA_BASE_URL: http://cynode-pma:8090`; cynode-pma is configured with `OLLAMA_BASE_URL: http://ollama:11434` and `INFERENCE_MODEL` so the PMA can reach Ollama for chat completions.
  user-gateway depends on cynode-pma (service_healthy); cynode-pma depends on ollama (service_started).
- **Orchestrator BDD:** Chat scenario uses OpenAI-compatible endpoints (GET /v1/models, POST /v1/chat/completions); step "I send a chat message" calls `/v1/chat/completions` with OpenAI-format body; assertion checks `choices[0].message.content`.
- **Chat routing:** Implementation in `orchestrator/internal/handlers/openai_chat.go` is aligned with `docs/tech_specs/openai_compatible_chat_api.md` (effective model default `cynodeai.pm`; exactly `cynodeai.pm` -> PM agent; other -> direct inference).
  Comments in code reference the spec section.
