# CyNodeAI Development Setup

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Service Layout](#service-layout)
- [Manual Setup](#manual-setup)
  - [1. Start PostgreSQL](#1-start-postgresql)
  - [2. Build Binaries](#2-build-binaries)
  - [3. Run Services](#3-run-services)
  - [4. Docker Compose (Orchestrator Stack)](#4-docker-compose-orchestrator-stack)
  - [5. Docker Compose (Worker Node)](#5-docker-compose-worker-node)
- [Testing the API](#testing-the-api)
  - [User API (Port 8080)](#user-api-port-8080)
  - [Node Registration (Port 8082)](#node-registration-port-8082)
- [API Endpoints](#api-endpoints)
  - [Authentication endpoints](#authentication-endpoints)
  - [User endpoints](#user-endpoints)
  - [Tasks endpoints](#tasks-endpoints)
  - [Node (Control-Plane)](#node-control-plane)
- [E2E Demo and Tests](#e2e-demo-and-tests)
- [Environment Variables](#environment-variables)
  - [Orchestrator (Control-Plane, User-Gateway)](#orchestrator-control-plane-user-gateway)
  - [Worker Node](#worker-node)
- [Development Workflow](#development-workflow)
- [Systemd (Podman)](#systemd-podman)
- [Troubleshooting](#troubleshooting)
- [Architecture Overview](#architecture-overview)

## Prerequisites

This guide explains how to set up and run the CyNodeAI MVP Phase 1 development environment.

- **Go 1.25+**: Install via [go.dev](https://go.dev)
- **Container Runtime**: Either [Podman](https://podman.io) or [Docker](https://docker.com) with Compose (e.g. `podman compose` or `docker compose`)
- **curl**, **jq**: For testing API endpoints and E2E scripts

Use the project [justfile](../justfile) for tooling; run `just` to list targets.

## Quick Start

The default path uses Docker Compose for the orchestrator stack (Postgres, control-plane, user-gateway, Ollama).

### Start Orchestrator Stack

```bash
# From repo root: build binaries and start orchestrator stack (compose)
./scripts/setup-dev.sh start
# User API: http://localhost:8080  Control-plane: http://localhost:8082  Ollama: http://localhost:11434
# Default admin: admin / admin123
```

**Full E2E** (orchestrator stack, inference-proxy image, node-manager, and tests): run `just e2e`.

**Stop all services** (orchestrator stack and node processes): run `just e2e-stop`.

## Service Layout

| Service       | Port  | Role                                             |
| ------------- | ----- | ------------------------------------------------ |
| user-gateway  | 8080  | Auth, users, tasks, results                      |
| control-plane | 8082  | Migrations, node register/capability, dispatcher |
| worker-api    | 8081  | Run jobs (Worker API)                            |
| ollama        | 11434 | Inference (Ollama API; used by inference proxy)  |
| node-manager  | -     | Registers node, reports capabilities             |

When using `./scripts/setup-dev.sh start` or `just e2e`, the orchestrator stack (postgres, control-plane, user-gateway, ollama) runs in containers via Compose.
For a single source of truth on default ports and overrides, see [Ports and endpoints](tech_specs/ports_and_endpoints.md).

## Manual Setup

Follow these steps to run services locally without Docker Compose.

### 1. Start PostgreSQL

The orchestrator stack (see [Docker Compose](#4-docker-compose-orchestrator-stack)) includes Postgres.
For a standalone Postgres container (e.g. for manual runs):

```bash
./scripts/setup-dev.sh start-db
```

The script uses the `pgvector/pgvector:pg16` image so the vector extension is available for schema migrations.
To reset the DB: `just clean-db` or `./scripts/setup-dev.sh clean-db`.

### 2. Build Binaries

```bash
./scripts/setup-dev.sh build

# Or manually
go build -o bin/control-plane ./orchestrator/cmd/control-plane
go build -o bin/user-gateway ./orchestrator/cmd/user-gateway
go build -o bin/worker-api ./worker_node/cmd/worker-api
go build -o bin/node-manager ./worker_node/cmd/node-manager
```

### 3. Run Services

The recommended path is [Docker Compose (Orchestrator Stack)](#4-docker-compose-orchestrator-stack).
To run control-plane and user-gateway as local binaries instead:

Control-plane runs migrations on startup.
Start order: control-plane, then user-gateway.

```bash
export DATABASE_URL="postgres://cynodeai:cynodeai-dev-password@localhost:5432/cynodeai?sslmode=disable"
export MIGRATIONS_DIR=./orchestrator/migrations
export JWT_SECRET=dev-jwt-secret
export NODE_REGISTRATION_PSK=dev-node-psk
export BOOTSTRAP_ADMIN_PASSWORD=admin123

# Terminal 1: control-plane
./bin/control-plane

# Terminal 2: user-gateway
export USER_GATEWAY_LISTEN_ADDR=:8080
./bin/user-gateway
```

Optional (for job execution): start worker-api and node-manager with the same `WORKER_API_BEARER_TOKEN` and `NODE_REGISTRATION_PSK` as the control-plane.

### 4. Docker Compose (Orchestrator Stack)

The orchestrator stack runs Postgres, control-plane, user-gateway, and Ollama in containers.
Ports: 5432, 8082, 8080, 11434.

```bash
# From repo root (required for build context)
docker compose -f orchestrator/docker-compose.yml up -d
# or: podman compose -f orchestrator/docker-compose.yml up -d
```

The setup script uses this for `./scripts/setup-dev.sh start` and `just e2e`.
To stop: `just e2e-stop` or `./scripts/setup-dev.sh stop`.

### 5. Docker Compose (Worker Node)

```bash
cd worker_node
WORKER_API_BEARER_TOKEN=dev-worker-api-token NODE_REGISTRATION_PSK=dev-node-psk \
  ORCHESTRATOR_URL=http://host.containers.internal:8082 \
  docker compose up -d   # or podman compose up -d
```

## Testing the API

Use the following examples to verify the API (services must be running).

### User API (Port 8080)

```bash
# Login
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"handle": "admin", "password": "admin123"}'

# Create task (use access_token from login). Optional: "use_inference": true for inference-in-sandbox jobs.
curl -X POST http://localhost:8080/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "echo hello world"}'

# Get result
curl http://localhost:8080/v1/tasks/<task_id>/result -H "Authorization: Bearer $TOKEN"
```

### Node Registration (Port 8082)

```bash
curl -X POST http://localhost:8082/v1/nodes/register \
  -H "Content-Type: application/json" \
  -d '{
    "psk": "dev-node-psk",
    "capability": {
      "version": 1,
      "node": {"node_slug": "test-node"},
      "platform": {"os": "linux", "arch": "amd64"},
      "compute": {"cpu_cores": 8, "ram_mb": 16384}
    }
  }'
```

## API Endpoints

Reference for the main REST endpoints.

### Authentication Endpoints

| Method | Endpoint           | Description                 |
| ------ | ------------------ | --------------------------- |
| POST   | `/v1/auth/login`   | Login with handle/password  |
| POST   | `/v1/auth/refresh` | Refresh access token        |
| POST   | `/v1/auth/logout`  | Logout (invalidate session) |

### User Endpoints

| Method | Endpoint       | Description           |
| ------ | -------------- | --------------------- |
| GET    | `/v1/users/me` | Get current user info |

### Tasks Endpoints

| Method | Endpoint                | Description               |
| ------ | ----------------------- | ------------------------- |
| POST   | `/v1/tasks`             | Create a new task         |
| GET    | `/v1/tasks/{id}`        | Get task details          |
| GET    | `/v1/tasks/{id}/result` | Get task result with jobs |

### Node (Control-Plane)

| Method | Endpoint               | Description              |
| ------ | ---------------------- | ------------------------- |
| POST   | `/v1/nodes/register`   | Register a node with PSK  |
| POST   | `/v1/nodes/capability` | Report node capabilities |

## E2E Demo and Tests

The end-to-end demo tests: admin auto-creation, login, create task, task status, node registration, capability reporting, token refresh, logout (and optionally inference-in-sandbox).

```bash
# Full demo: build, start orchestrator stack (compose), build inference-proxy image, start node, run tests
just e2e

# With orchestrator stack and node already running
./scripts/setup-dev.sh test-e2e

# Stop all services (orchestrator stack and node processes)
just e2e-stop
```

BDD: run `just test-bdd` for the orchestrator, worker_node, and cynork Godog suites.

## Environment Variables

Key variables by component.

### Orchestrator (Control-Plane, User-Gateway)

| Variable                    | Description                        | Default    |
| --------------------------- | ---------------------------------- | ---------- |
| `DATABASE_URL`              | PostgreSQL connection string       | -          |
| `JWT_SECRET`                | JWT signing secret                 | -          |
| `NODE_REGISTRATION_PSK`     | Node registration PSK              | -          |
| `BOOTSTRAP_ADMIN_PASSWORD`  | Admin user password                | -          |
| `USER_GATEWAY_LISTEN_ADDR`  | User gateway listen address        | :8080      |
| `CONTROL_PLANE_LISTEN_ADDR` | Control-plane listen address       | :8082      |
| `WORKER_API_TARGET_URL`     | Worker API URL (control-plane)     | -          |
| `MIGRATIONS_DIR`            | Path to migrations (control-plane) | migrations |

Compose uses `POSTGRES_*`, `ORCHESTRATOR_PORT`, `CONTROL_PLANE_PORT`, `OLLAMA_IMAGE`; see [orchestrator/docker-compose.yml](../orchestrator/docker-compose.yml).

### Worker Node

| Variable                     | Description                                      |
| ---------------------------- | ------------------------------------------------ |
| `ORCHESTRATOR_URL`           | Control-plane URL (node-manager)                 |
| `NODE_REGISTRATION_PSK`      | Must match control-plane                         |
| `WORKER_API_BEARER_TOKEN`    | Must match control-plane                         |
| `NODE_MANAGER_WORKER_API_BIN`| Path to worker-api binary (e.g. when using script)|
| `OLLAMA_UPSTREAM_URL`        | For inference proxy (e.g. <http://host.containers.internal:11434>) |
| `INFERENCE_PROXY_IMAGE`      | Image for inference proxy (inference-in-sandbox)  |

## Development Workflow

- **Tests**: `just test-go` or `just test-go-cover` for coverage
- **Lint**: `just lint-go` or `just lint-go-ci`
- **Full CI**: `just ci` (run before every commit)
- **CLI (cynork)**: `just build-cynork`; run against localhost with default gateway URL `http://localhost:8080` (see [cynork/README.md](../cynork/README.md))

## Systemd (Podman)

See [orchestrator/systemd/README.md](../orchestrator/systemd/README.md) and [worker_node/systemd/README.md](../worker_node/systemd/README.md).

## Troubleshooting

- **Containers**: `podman ps` or `docker ps`; orchestrator stack uses `cynodeai-postgres`, `cynodeai-control-plane`, `cynodeai-user-gateway`, `cynodeai-ollama`.
- **Migrations**: Control-plane runs them on startup.
  Migrate-only (when running binary directly): `./bin/control-plane -migrate-only`.
- **Ports**: User API 8080, control-plane 8082, worker-api 8081, Ollama 11434.
  See [Ports and endpoints](tech_specs/ports_and_endpoints.md) for the full table and overrides.
- **CI/coverage**: Run `just test-go-cover` or `just ci`.
  For orchestrator DB tests, set `POSTGRES_TEST_DSN` if not using testcontainers.
- **Connection issues**: Set `POSTGRES_TEST_DSN` to use a real DB; use `SKIP_TESTCONTAINERS=1` if needed.
- **Port conflicts**: The script runs `compose down` and removes existing orchestrator containers before `compose up`.
  To use different ports: `ORCHESTRATOR_PORT=9080 CONTROL_PLANE_PORT=9082 ./scripts/setup-dev.sh start`.
- **Reset DB**: `just clean-db` or `./scripts/setup-dev.sh clean-db`.
  Then start again with `./scripts/setup-dev.sh start`.
- **Stop everything**: `just e2e-stop` or `./scripts/setup-dev.sh stop`.

## Architecture Overview

See [tech_specs/_main.md](tech_specs/_main.md) and the specs under [tech_specs/](tech_specs/) for detailed architecture.

High-level: user-gateway (:8080) and control-plane (:8082) talk to PostgreSQL; nodes register with the control-plane; the dispatcher sends jobs to worker-api (:8081) on registered nodes.
Ollama (:11434) provides inference; jobs with `use_inference` run in a pod with an inference proxy so the sandbox can call the model.
