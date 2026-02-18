# CyNodeAI Development Setup Guide

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Service Layout](#service-layout)
- [Manual Setup](#manual-setup)
- [Testing the API](#testing-the-api)
- [Environment Variables](#environment-variables)
- [Systemd (Podman)](#systemd-podman)
- [Troubleshooting](#troubleshooting)

## Prerequisites

This guide explains how to set up and run the CyNodeAI MVP Phase 1 development environment with the multi-service layout.

- **Go 1.25+**: Install via [go.dev](https://go.dev)
- **Container Runtime**: Either [Podman](https://podman.io) or [Docker](https://docker.com)
- **curl**, **jq**: For testing API endpoints

## Quick Start

```bash
# From repo root: start postgres, control-plane, user-gateway
./scripts/setup-dev.sh start

# User API: http://localhost:8080  Control-plane: http://localhost:8082
# Default admin: admin / admin123
```

For full E2E including worker and dispatcher, start worker-api and node-manager in separate terminals, or use Docker Compose (see below).

## Service Layout

| Service       | Port | Role                                             |
| ------------- | ---- | ------------------------------------------------ |
| user-gateway  | 8080 | Auth, users, tasks, results                      |
| control-plane | 8082 | Migrations, node register/capability, dispatcher |
| worker-api    | 8081 | Run jobs (Worker API)                            |
| node-manager  | -    | Registers node, reports capabilities             |

## Manual Setup

Follow these steps to run services locally without Docker Compose.

### 1. Start PostgreSQL

```bash
./scripts/setup-dev.sh start-db

# Or manually (from repo root)
podman run -d --name cynodeai-postgres-dev \
  -e POSTGRES_USER=cynodeai -e POSTGRES_PASSWORD=cynodeai-dev-password \
  -e POSTGRES_DB=cynodeai -p 5432:5432 \
  -v cynodeai-postgres-data:/var/lib/postgresql/data \
  postgres:16-alpine
```

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

### 4. Docker Compose (Orchestrator Only)

```bash
cd orchestrator
docker compose up -d   # or: podman compose up -d
# Postgres, control-plane, user-gateway. Optional: --profile optional for mcp-gateway, api-egress.
```

### 5. Docker Compose (Worker Node)

```bash
# Set ORCHESTRATOR_URL and shared WORKER_API_BEARER_TOKEN / NODE_REGISTRATION_PSK
cd worker_node
WORKER_API_BEARER_TOKEN=dev-worker-api-token NODE_REGISTRATION_PSK=dev-node-psk \
  ORCHESTRATOR_URL=http://host.containers.internal:8082 \
  docker compose up -d
```

## Testing the API

Use the following examples to verify the API (services must be running).

### User API (Port 8080)

```bash
# Login
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"handle": "admin", "password": "admin123"}'

# Create task (use access_token from login)
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

## Environment Variables

Key variables by component:

### Orchestrator (Control-Plane, User-Gateway)

| Variable                    | Description                        | Default    |
| --------------------------- | ---------------------------------- | ---------- |
| `DATABASE_URL`              | PostgreSQL connection string       | -          |
| `JWT_SECRET`                | JWT signing secret                 | -          |
| `NODE_REGISTRATION_PSK`     | Node registration PSK              | -          |
| `BOOTSTRAP_ADMIN_PASSWORD`  | Admin user password                | -          |
| `USER_GATEWAY_LISTEN_ADDR`  | User gateway listen address        | :8080      |
| `CONTROL_PLANE_LISTEN_ADDR` | Control-plane listen address       | :8082      |
| `MIGRATIONS_DIR`            | Path to migrations (control-plane) | migrations |

### Control-Plane Dispatcher

| Variable                  | Description              |
| ------------------------- | ------------------------ |
| `WORKER_API_URL`          | Worker API base URL      |
| `WORKER_API_BEARER_TOKEN` | Token for calling worker |

### Worker Node

| Variable                  | Description                      |
| ------------------------- | -------------------------------- |
| `ORCHESTRATOR_URL`        | Control-plane URL (node-manager) |
| `NODE_REGISTRATION_PSK`   | Must match control-plane         |
| `WORKER_API_BEARER_TOKEN` | Must match control-plane         |

## Systemd (Podman)

See `orchestrator/systemd/README.md` and `worker_node/systemd/README.md`.
Generate units with:

```bash
podman compose -f orchestrator/docker-compose.yml up -d
./scripts/podman-generate-units.sh orchestrator
```

## Troubleshooting

- **Postgres**: `podman ps | grep cynodeai-postgres`
- **Migrations**: Control-plane runs them on startup.
  For migrate-only: `./bin/control-plane -migrate-only`
- **Ports**: User API 8080, control-plane 8082, worker-api 8081
- **Go coverage**: Run `just test-go-cover-podman` (or `just ci`). For the orchestrator, this starts a Postgres container with Podman (`pgvector/pgvector:pg16` on port 15432), sets `POSTGRES_TEST_DSN`, runs tests, then removes the container. No Docker socket or testcontainers needed.
