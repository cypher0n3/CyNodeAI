# CyNodeAI MVP Phase 1 Status Report

<!-- Date: 2026-02-20. Branch: mvp/phase-1. Status: In progress -->

- [Summary of Implementation](#summary-of-implementation)
- [Files Created](#files-created)
- [Test Coverage Status](#test-coverage-status)
- [Known Issues and Remaining Work](#known-issues-and-remaining-work)
- [Code Review (2026-02-20)](#code-review-2026-02-20)
- [Running the System Locally](#running-the-system-locally)
- [Architecture Overview](#architecture-overview)
- [Contact](#contact)

## Summary of Implementation

Phase 1 establishes the foundational architecture for CyNodeAI - a distributed task
orchestration system with secure code execution.
The MVP implements:

- **Go Module Structure**: Clean separation between orchestrator API and node components
- **PostgreSQL Schema**: Complete migrations for authentication and task/job management
- **JWT Authentication**: Secure token-based auth with refresh token rotation
- **Password Security**: Argon2id hashing with configurable rate limiting
- **REST APIs**: Full orchestrator endpoints for node registration and job dispatch
- **User Gateway**: Authentication, task submission, and result retrieval endpoints
- **Node Worker**: Worker API framework for sandbox job execution
- **BDD Tests**: Feature file for single-node happy path scenario
- **Node-aware dispatch**: Dispatcher uses per-node `worker_api_target_url` and bearer token from config delivery (not global env).
  Only nodes that are active and have acknowledged config with Worker API details are dispatchable.
- **Config-delivered token**: Worker API bearer token and target URL are delivered via node config payload; persisted per-node when config is served so dispatch can use them.
  E2E flow: control-plane has `WORKER_API_TARGET_URL` and `WORKER_API_BEARER_TOKEN`; node manager fetches config and starts worker-api with the delivered token (no manual token on the node).

---

## Files Created

Key artifacts and entry points for Phase 1.

### Command Entry Points

| File                                     | Description                             |
| ---------------------------------------- | --------------------------------------- |
| `orchestrator/cmd/control-plane/main.go` | Control-plane API, migrations, dispatch |
| `orchestrator/cmd/user-gateway/main.go`  | User-facing API (auth, tasks)           |
| `orchestrator/cmd/mcp-gateway/main.go`   | MCP gateway (scaffold)                  |
| `orchestrator/cmd/api-egress/main.go`    | API egress (scaffold)                   |
| `worker_node/cmd/worker-api/`            | Worker API (jobs:run)                   |
| `worker_node/cmd/node-manager/main.go`   | Node registration and capability        |

### Database Migrations

| File                                                | Description                              |
| --------------------------------------------------- | ---------------------------------------- |
| `orchestrator/migrations/0001_identity_auth.sql`    | Users, credentials, sessions, audit logs |
| `orchestrator/migrations/0002_tasks_jobs_nodes.sql` | Tasks, jobs, nodes, capabilities         |

### Shared and Internal Packages

| Location                           | Description                                                               |
| ---------------------------------- | ------------------------------------------------------------------------- |
| `go_shared_libs/contracts/`        | workerapi, nodepayloads, problem (shared by orchestrator and worker_node) |
| `orchestrator/internal/auth`       | JWT, Argon2id, rate limiting                                              |
| `orchestrator/internal/config`     | Environment-based configuration                                           |
| `orchestrator/internal/database`   | DB operations, migrations                                                 |
| `orchestrator/internal/handlers`   | HTTP handlers                                                             |
| `orchestrator/internal/middleware` | HTTP middleware                                                           |
| `orchestrator/internal/models`     | Data models                                                               |
| `orchestrator/internal/testutil`   | Test utilities                                                            |

### BDD Features

| File                                      | Description                         |
| ----------------------------------------- | ----------------------------------- |
| `features/e2e/single_node_happy_path.feature` | Single node task execution scenario (E2E) |
| `features/orchestrator/node_registration_and_config.feature` | Node registration, config fetch, config ack |
| `features/orchestrator/orchestrator_task_lifecycle.feature` | Task create, status, result; dispatcher per-node |
| `features/orchestrator/orchestrator_startup.feature` | Orchestrator fail-fast (inference path) |
| `features/orchestrator/initial_auth.feature` | User login, refresh, logout |
| `features/worker_node/node_manager_config_startup.feature` | Node manager config fetch and fail-fast |
| `features/worker_node/worker_node_sandbox_execution.feature` | Worker API auth, sandbox run, workspace/env |

### Dev Docs

| File | Description |
| ---- | ----------- |
| `dev_docs/mvp_phase1_completion_plan.md` | Phase 1 chunks, acceptance checklist, requirement scope |
| `dev_docs/mvp_phase1_code_review_report.md` | In-depth code review vs specs, feature/BDD gaps (2026-02-20) |
| `dev_docs/PHASE1_STATUS.md` | This status report |
| `dev_docs/mvp_specs_gaps_closure_status.md` | Spec gaps closure status |

---

## Test Coverage Status

| Package                            | Coverage | Status            |
| ---------------------------------- | -------- | ----------------- |
| `orchestrator/internal/config`     | 100.0%   | Excellent         |
| `orchestrator/internal/testutil`   | 90%+     | Good              |
| `orchestrator/internal/database`   | 92.7%    | Good              |
| `orchestrator/internal/auth`       | 88.5%    | Good              |
| `orchestrator/internal/middleware` | 88.7%    | Good              |
| `orchestrator/internal/handlers`   | 86.8%    | Good              |
| `orchestrator/cmd/*`               | 0.0%     | Main entry points |

**Overall:** Core packages have good test coverage (82-100%).
Main entry points are excluded as they primarily wire dependencies.

---

## Known Issues and Remaining Work

Notes and open items for MVP completion.

### Known Issues

1. **golangci-lint**: Use `.golangci.yml` v2; run `just install-go-tools` if the installed linter is outdated.
2. **Markdown lint**: `just ci` runs markdownlint; ensure `.venv` is not in the repo root when linting, or fix all markdown files to pass.
3. **Full CI**: Run `just venv` before `just ci` if Python lint is required; all Go lint, tests (with coverage), and vuln check must pass.

### Remaining Work for MVP Completion

- [x] End-to-end integration test with actual PostgreSQL (see "E2E and Compose" below)
- [x] Docker/Podman compose for local development
- [ ] Sandbox container implementation for secure code execution
- [ ] WebSocket support for real-time job status updates
- [ ] Node heartbeat and health monitoring
- [ ] Comprehensive error handling and retry logic
- [ ] API documentation (OpenAPI/Swagger)
- [ ] Production configuration and deployment guides

### Spec Gaps From Code Review (2026-02-20)

See `dev_docs/mvp_phase1_code_review_report.md` for full detail.

- **config_version:** Spec (`node_payloads.md`) requires ULID (26-char Crockford Base32) for config version; implementation uses literal `"1"`.
  Should implement ULID generation when serving config.
- **Ollama image:** Spec says node starts "the single Ollama container specified by the orchestrator"; implementation uses env `OLLAMA_IMAGE`.
  Either add image to config payload or document as Phase 1 env-only and defer.
- **Worker API readyz:** Spec requires `GET /readyz`; only `GET /healthz` is implemented.
- **Orchestrator fail-fast:** Feature `orchestrator_startup.feature` asserts orchestrator fails fast when no inference path; spec places fail-fast on the node.
  Clarify scope or implement orchestrator readiness check.

---

## Running the System Locally

How to run Postgres, orchestrator, and nodes on your machine.

### Local Prerequisites

- Go 1.25+ (`just install-go`)
- PostgreSQL 15+ running locally or via container
- Just command runner (`brew install just` or equivalent)

### Quick Start

```bash
# 1. Install dependencies
just setup

# 2. Set environment variables
export DATABASE_URL="postgres://user:pass@localhost:5432/cynodeai?sslmode=disable"
export JWT_SECRET="your-256-bit-secret-key-here"
export NODE_REGISTRATION_PSK="node-registration-psk"

# 3. Run database migrations
# (Migrations run automatically on orchestrator startup)

# 4. Start the orchestrator API
go run ./orchestrator/cmd/control-plane

# 5. In another terminal, start a node (control-plane is on 8082)
export ORCHESTRATOR_URL="http://localhost:8082"
export NODE_SLUG="node-001"
go run ./worker_node/cmd/node-manager
# Worker API (optional, for job execution): go run ./worker_node/cmd/worker-api
```

### Running Tests

```bash
# Run all tests with coverage
just test-go

# Or directly with go
go test ./... -cover

# Run specific package tests
go test ./orchestrator/internal/auth/... -v
```

### Development Commands

```bash
just              # List all available commands
just fmt-go       # Format Go code
just lint-go      # Run linter
just test-go      # Run tests with coverage
just build-go     # Build binaries
just e2e          # E2E: start Postgres + orchestrator + node, run happy path
```

### E2E and Compose

**One-command E2E:** From repo root, run `just e2e`.
This starts PostgreSQL (Podman/Docker),
builds binaries, starts control-plane, user-gateway, worker-api, and node-manager, then runs
the happy path (login, create task, get task, get result).
Requires `jq` and podman or docker.

#### Start Everything Locally With Compose

1. **Orchestrator stack** (Postgres + control-plane + user-gateway):
   `cd orchestrator && podman compose up -d`
2. **Worker node** (worker-api + node-manager): from repo root,
   `cd worker_node && ORCHESTRATOR_URL=http://host.containers.internal:8082 \
    WORKER_API_BEARER_TOKEN=dev-worker-api-token-change-me \
    NODE_REGISTRATION_PSK=dev-node-psk-secret \
    podman compose up -d`

Alternatively use `./scripts/setup-dev.sh start` (starts Postgres, control-plane, user-gateway;
build first with `./scripts/setup-dev.sh build`), then start node manually or run `just e2e`
which runs the full demo including the node.

**BDD:** Run `just test-bdd` to run the orchestrator and worker_node Godog suites.
The orchestrator suite starts Postgres via testcontainers when `POSTGRES_TEST_DSN` is unset; set `SKIP_TESTCONTAINERS=1` to run without a DB (scenarios that need the DB will skip).
The scenarios in `features/e2e/single_node_happy_path.feature` are exercised by the same E2E flow; run `just e2e` or `./scripts/setup-dev.sh test-e2e` (with services already started).

### Code Review (2026-02-20)

An in-depth code review of the Phase 1 implementation versus the technical specs and completion plan is in `dev_docs/mvp_phase1_code_review_report.md`.
Summary:

- **Implemented:** Node-aware dispatch (per-node URL and token from config), config delivery and config ack, node manager startup order (register -> fetch config -> start Worker API -> start Ollama -> config ack), sandbox network policy and workspace/env, user auth and task APIs.
- **Gaps:** `config_version` not ULID per spec; Ollama image from env rather than orchestrator config; Worker API missing `GET /readyz`; capability report schema in code is minimal vs full spec; orchestrator fail-fast scenario scope unclear (node vs orchestrator).
- **Feature/BDD:** Feature files for orchestrator and worker_node cover registration, config, dispatcher, sandbox, and node manager; orchestrator BDD starts Postgres via testcontainers when DSN is unset for full Phase 1 coverage.

### Recent Updates (Phase 1 CI and Refactor)

- **Database:** Shared helpers in `orchestrator/internal/database` (queryRow, execContext, scanAllRows); table-driven getter/exec/list tests.
- **Testutil:** Unified `runWithLock`, `getByKey`, and invalidate helpers in `orchestrator/internal/testutil/mock_db.go`; table-driven GetNotFound tests.
- **tmp/old review:** `dev_docs/tmp_old_review.md` compares `tmp/old/` with current orchestrator and `go_shared_libs`; no gaps.
- **E2E:** `just e2e` runs full demo (Postgres, control-plane, user-gateway, node, happy path); `scripts/setup-dev.sh full-demo` includes node.
- **Worker node:** Lint fixes in `worker_node/cmd/node-manager` (hugeParam, errcheck, paramTypeCombine) and `worker_node/cmd/worker-api` (gocognit via extracted handlers).

---

## Architecture Overview

```mermaid
flowchart TB
  subgraph clients["User Clients"]
    rest["REST API / WebSocket"]
  end

  subgraph orchestrator["Orchestrator API"]
    auth["Auth"]
    handlers["Handlers"]
    middleware["Middleware"]
    db["Database"]
  end

  subgraph nodes["Worker Nodes"]
    n1["Node 1\nWorker / Sandbox"]
    n2["Node 2\nWorker / Sandbox"]
    nN["Node N\nWorker / Sandbox"]
  end

  clients --> orchestrator
  orchestrator --> n1
  orchestrator --> n2
  orchestrator --> nN
```

---

## Contact

For questions or issues, please open a GitHub issue or contact the development team.
