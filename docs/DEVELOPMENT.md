# CyNodeAI Development Guide

This guide describes how to set up and run CyNodeAI locally for development and testing.

## Prerequisites

- **Go 1.25+**: Required for building the applications
- **Container Runtime**: Either Podman (preferred) or Docker
- **jq**: For JSON processing in test scripts (optional, for E2E tests)

## Quick Start

### Using the Setup Script

The easiest way to run the full demo is:

```bash
# Full demo: start services, run migrations, test E2E flow
./scripts/setup-dev.sh full-demo

# Stop all services when done
./scripts/setup-dev.sh stop
```

### Manual Setup

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
go build -o bin/orchestrator-api ./cmd/orchestrator-api
go build -o bin/node ./cmd/node
```

#### 3. Run Migrations

```bash
./scripts/setup-dev.sh migrate

# Or manually
export DATABASE_URL="postgres://cynodeai:cynodeai-dev-password@localhost:5432/cynodeai?sslmode=disable"
go run ./cmd/orchestrator-api -migrate-only
```

#### 4. Start Orchestrator API

```bash
export DATABASE_URL="postgres://cynodeai:cynodeai-dev-password@localhost:5432/cynodeai?sslmode=disable"
export SERVER_PORT=8080
export JWT_SECRET_KEY=dev-jwt-secret
export NODE_REGISTRATION_PSK=dev-node-psk
export ADMIN_HANDLE=admin
export ADMIN_PASSWORD=admin123

./bin/orchestrator-api
```

#### 5. Start a Node (Optional)

```bash
export ORCHESTRATOR_URL=http://localhost:8080
export NODE_PSK=dev-node-psk
export NODE_SLUG=dev-node-1

./bin/node
```

## API Endpoints

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/auth/login` | Login with handle/password |
| POST | `/v1/auth/refresh` | Refresh access token |
| POST | `/v1/auth/logout` | Logout (invalidate session) |

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/users/me` | Get current user info |

### Tasks

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/tasks` | Create a new task |
| GET | `/v1/tasks/{id}` | Get task details |
| GET | `/v1/tasks/{id}/result` | Get task result with jobs |

### Nodes

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/nodes/register` | Register a node with PSK |
| POST | `/v1/nodes/capability` | Report node capabilities |

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

### Orchestrator API

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | - | PostgreSQL connection string |
| `SERVER_PORT` | 8080 | HTTP server port |
| `JWT_SECRET_KEY` | - | Secret for JWT signing |
| `NODE_REGISTRATION_PSK` | - | Pre-shared key for node registration |
| `ADMIN_HANDLE` | admin | Admin user handle |
| `ADMIN_PASSWORD` | - | Admin user password (required for auto-creation) |

### Node

| Variable | Default | Description |
|----------|---------|-------------|
| `ORCHESTRATOR_URL` | - | URL of the orchestrator API |
| `NODE_PSK` | - | Pre-shared key for registration |
| `NODE_SLUG` | - | Unique identifier for the node |
| `CONTAINER_RUNTIME` | podman | Container runtime (podman/docker) |

## Development Workflow

### Running Tests

```bash
# All tests
just test-go

# With coverage
go test -cover ./...

# Specific package
go test -v ./internal/handlers/...
```

### Linting

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

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────┐
│   User/Client   │────▶│ Orchestrator API │────▶│  PostgreSQL  │
└─────────────────┘     └──────────────────┘     └──────────────┘
                               │
                               │ PSK Auth
                               ▼
                        ┌──────────────┐
                        │    Node(s)   │
                        │  (Workers)   │
                        └──────────────┘
```

See `docs/tech_specs/` for detailed architecture documentation.
