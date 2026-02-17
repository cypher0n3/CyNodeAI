# CyNodeAI Development Guide

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [Manual Setup Steps](#manual-setup-steps)
- [API Endpoints](#api-endpoints)
  - [Authentication Endpoints](#authentication-endpoints)
  - [User Endpoints](#user-endpoints)
  - [Task Endpoints](#task-endpoints)
  - [Node Endpoints](#node-endpoints)
- [E2E Demo Flow](#e2e-demo-flow)
  - [Running E2E Tests](#running-e2e-tests)
- [Environment Variables](#environment-variables)
  - [Orchestrator (Control-Plane, User-Gateway)](#orchestrator-control-plane-user-gateway)
  - [Worker Node](#worker-node)
- [Development Workflow](#development-workflow)
  - [Running Tests](#running-tests)
  - [Linting and Formatting](#linting-and-formatting)
  - [Full CI Pipeline](#full-ci-pipeline)
- [Troubleshooting](#troubleshooting)
  - [PostgreSQL Connection Issues](#postgresql-connection-issues)
  - [Migration Issues](#migration-issues)
  - [Port Conflicts](#port-conflicts)
- [Architecture Overview](#architecture-overview)

## Prerequisites

This guide describes how to set up and run CyNodeAI locally for development and testing.

- **Go 1.25+**: Required for building the applications
- **Container Runtime**: Either Podman (preferred) or Docker
- **jq**: For JSON processing in test scripts (optional, for E2E tests)

## Quick Start

```bash
# Start all services (postgres, control-plane, user-gateway, worker-api, node-manager)
./scripts/dev-setup.sh start
./scripts/dev-setup.sh test

# Or: setup-dev.sh start then setup-dev.sh test-e2e
# Stop: dev-setup.sh stop or setup-dev.sh stop
```

### Manual Setup Steps

Steps to run without the Quick Start script.

#### 1. Start PostgreSQL

```bash
# Using the setup script
./scripts/setup-dev.sh start-db

# Or manually with podman/docker
podman run -d \
    --name cynodeai-postgres-dev \
    -e POSTGRES_USER=cynodeai \
    -e POSTGRES_PASSWORD=cynodeai-dev-password \
    -e POSTGRES_DB=cynodeai \
    -p 5432:5432 \
    postgres:16-alpine
```

#### 2. Build Binaries

```bash
./scripts/setup-dev.sh build

# Or manually
go build -o bin/control-plane ./orchestrator/cmd/control-plane
go build -o bin/user-gateway ./orchestrator/cmd/user-gateway
go build -o bin/worker-api ./worker_node/cmd/worker-api
go build -o bin/node-manager ./worker_node/cmd/node-manager
```

#### 3. Run Services

Control-plane runs migrations on startup.
Start: control-plane, then user-gateway; optionally worker-api and node-manager.

```bash
export DATABASE_URL="postgres://cynodeai:cynodeai-dev-password@localhost:5432/cynodeai?sslmode=disable"
export JWT_SECRET=dev-jwt-secret
export NODE_REGISTRATION_PSK=dev-node-psk
export BOOTSTRAP_ADMIN_PASSWORD=admin123
export MIGRATIONS_DIR=./orchestrator/migrations

./bin/control-plane   # port 8082
# In another terminal:
export USER_GATEWAY_LISTEN_ADDR=:8080 && ./bin/user-gateway
# Optional: ./bin/worker-api (8081), ./bin/node-manager (ORCHESTRATOR_URL=http://localhost:8082)
```

#### 4. Docker Compose

```bash
cd orchestrator && podman compose up -d
cd worker_node && ORCHESTRATOR_URL=http://host.containers.internal:8082 \
  WORKER_API_BEARER_TOKEN=dev-token NODE_REGISTRATION_PSK=dev-psk podman compose up -d
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

### Task Endpoints

| Method | Endpoint                | Description               |
| ------ | ----------------------- | ------------------------- |
| POST   | `/v1/tasks`             | Create a new task         |
| GET    | `/v1/tasks/{id}`        | Get task details          |
| GET    | `/v1/tasks/{id}/result` | Get task result with jobs |

### Node Endpoints

| Method | Endpoint               | Description              |
| ------ | ---------------------- | ------------------------ |
| POST   | `/v1/nodes/register`   | Register a node with PSK |
| POST   | `/v1/nodes/capability` | Report node capabilities |

## E2E Demo Flow

The end-to-end demo tests the following flow:

1. **Admin Auto-Creation**: On first startup, an admin user is created
2. **Login**: Authenticate with admin credentials
3. **Create Task**: Submit a task with a prompt
4. **Check Task Status**: Retrieve task details
5. **Node Registration**: Register a node with PSK
6. **Capability Reporting**: Node reports its capabilities
7. **Token Refresh**: Refresh the access token
8. **Logout**: End the session

### Running E2E Tests

```bash
# With services running
./scripts/setup-dev.sh test-e2e

# Or full demo (starts services, runs tests)
./scripts/setup-dev.sh full-demo
```

## Environment Variables

Configuration options by component.

### Orchestrator (Control-Plane, User-Gateway)

| Variable                    | Default    | Description                                       |
| --------------------------- | ---------- | ------------------------------------------------- |
| `DATABASE_URL`              | -          | PostgreSQL connection string                      |
| `JWT_SECRET`                | -          | Secret for JWT signing                            |
| `NODE_REGISTRATION_PSK`     | -          | Pre-shared key for node registration              |
| `BOOTSTRAP_ADMIN_PASSWORD`  | -          | Admin user password (control-plane creates admin) |
| `USER_GATEWAY_LISTEN_ADDR`  | :8080      | User gateway listen address                       |
| `CONTROL_PLANE_LISTEN_ADDR` | :8082      | Control-plane listen address                      |
| `MIGRATIONS_DIR`            | migrations | Path to SQL migrations (control-plane)            |
| `WORKER_API_URL`            | -          | Worker API URL (control-plane dispatcher)         |
| `WORKER_API_BEARER_TOKEN`   | -          | Token for dispatcher -> worker-api                |

### Worker Node

| Variable                  | Default | Description                            |
| ------------------------- | ------- | -------------------------------------- |
| `ORCHESTRATOR_URL`        | -       | Control-plane URL (node-manager)       |
| `NODE_REGISTRATION_PSK`   | -       | Must match control-plane               |
| `NODE_SLUG`               | -       | Unique identifier for the node         |
| `WORKER_API_BEARER_TOKEN` | -       | Must match control-plane (worker-api)  |
| `CONTAINER_RUNTIME`       | podman  | Runtime for job sandboxes (worker-api) |

## Development Workflow

Common commands for day-to-day development.

### Running Tests

```bash
# All tests
just test-go

# With coverage
go test -cover ./...

# Specific package
go test -v ./orchestrator/internal/handlers/...
```

### Linting and Formatting

```bash
# Quick lint
just lint-go

# Full CI lint
just lint-go-ci
```

### Full CI Pipeline

```bash
just ci
```

## Troubleshooting

Common issues and resolutions.

### PostgreSQL Connection Issues

```bash
# Check if container is running
podman ps | grep cynodeai-postgres

# View container logs
podman logs cynodeai-postgres-dev

# Test connection
psql "postgres://cynodeai:cynodeai-dev-password@localhost:5432/cynodeai"
```

### Migration Issues

If migrations fail, try resetting the database:

```bash
./scripts/setup-dev.sh clean-db
./scripts/setup-dev.sh start-db
./scripts/setup-dev.sh migrate
```

### Port Conflicts

If port 8080 or 5432 are in use:

```bash
ORCHESTRATOR_PORT=8081 POSTGRES_PORT=5433 ./scripts/setup-dev.sh start
```

## Architecture Overview

High-level component diagram:

```mermaid
flowchart LR
  subgraph client[" "]
    A[User/Client]
  end
  subgraph gateway["user-gateway :8080"]
    B[user-gateway]
  end
  subgraph db[" "]
    C[(PostgreSQL)]
  end
  subgraph node[" "]
    D[node-manager\nregister]
  end
  subgraph plane["control-plane :8082"]
    E[dispatcher]
  end
  subgraph worker[" "]
    F[worker-api :8081]
  end
  A --> B --> C
  B --> E
  D --> E --> F
```

See `docs/tech_specs/` for detailed architecture documentation.
