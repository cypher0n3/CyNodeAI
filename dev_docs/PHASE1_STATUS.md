# CyNodeAI MVP Phase 1 Status Report

<!-- Date: 2026-02-16. Branch: mvp/phase-1. Status: In progress -->

- [Summary of Implementation](#summary-of-implementation)
- [Files Created](#files-created)
- [Test Coverage Status](#test-coverage-status)
- [Known Issues and Remaining Work](#known-issues-and-remaining-work)
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
| `features/single_node_happy_path.feature` | Single node task execution scenario |

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

**BDD:** The scenarios in `features/single_node_happy_path.feature` are exercised by the same
E2E flow; run `just e2e` or `./scripts/setup-dev.sh test-e2e` (with services already started).

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
