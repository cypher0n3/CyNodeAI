# CyNodeAI Development Setup Guide

This guide explains how to set up and run the CyNodeAI MVP Phase 1 development environment.

## Prerequisites

- **Go 1.25+**: Install via `just install-go` or from [go.dev](https://go.dev)
- **Container Runtime**: Either [Podman](https://podman.io) or [Docker](https://docker.com)
- **curl**: For testing API endpoints

## Quick Start

```bash
# Run full setup (starts postgres, migrations, orchestrator-api, and node)
./scripts/dev-setup.sh setup

# API will be available at http://localhost:8080
# Default admin credentials: admin / admin123
```

## Manual Setup Steps

### 1. Start PostgreSQL

```bash
# Using the dev script
./scripts/dev-setup.sh postgres

# Or manually with podman
podman run -d \
  --name cynodeai-postgres \
  -e POSTGRES_PASSWORD=cynodeai_dev_password \
  -e POSTGRES_DB=cynodeai \
  -p 5432:5432 \
  postgres:15-alpine
```

### 2. Run Database Migrations

```bash
# Using the dev script
./scripts/dev-setup.sh migrate

# Or manually
export DATABASE_URL="postgres://postgres:cynodeai_dev_password@localhost:5432/cynodeai?sslmode=disable"
for f in migrations/*.sql; do
  podman exec -i cynodeai-postgres psql -U postgres -d cynodeai < "$f"
done
```

### 3. Build Binaries

```bash
# Using the dev script
./scripts/dev-setup.sh build

# Or using Go directly
go build -o bin/orchestrator-api ./cmd/orchestrator-api
go build -o bin/node ./cmd/node
```

### 4. Start Orchestrator API

```bash
export DATABASE_URL="postgres://postgres:cynodeai_dev_password@localhost:5432/cynodeai?sslmode=disable"
export JWT_SECRET="dev-jwt-secret"
export NODE_REGISTRATION_PSK="dev-node-psk"
export ADMIN_PASSWORD="admin123"
export LISTEN_ADDR=":8080"
export LOG_LEVEL="debug"

./bin/orchestrator-api
```

### 5. Start Node

In a separate terminal:

```bash
export ORCHESTRATOR_URL="http://localhost:8080"
export NODE_SLUG="dev-node-1"
export NODE_NAME="Development Node 1"
export NODE_REGISTRATION_PSK="dev-node-psk"
export LOG_LEVEL="debug"

./bin/node
```

## Testing the End-to-End Flow

### 1. Admin Login

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"handle": "admin", "password": "admin123"}'
```

Response:
```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

### 2. Create a Task

```bash
export TOKEN="<access_token from login>"

curl -X POST http://localhost:8080/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "echo hello world"}'
```

### 3. Get Task Result

```bash
curl http://localhost:8080/v1/tasks/<task_id>/result \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Node Registration (automatic on node startup)

The node automatically registers with the orchestrator using the PSK:

```bash
curl -X POST http://localhost:8080/v1/nodes/register \
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

### Orchestrator API

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `JWT_SECRET` | Secret for JWT signing | Required |
| `NODE_REGISTRATION_PSK` | Pre-shared key for node registration | Required |
| `ADMIN_PASSWORD` | Initial admin user password | Required |
| `LISTEN_ADDR` | HTTP listen address | `:8080` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |

### Node

| Variable | Description | Default |
|----------|-------------|---------|
| `ORCHESTRATOR_URL` | Orchestrator API URL | Required |
| `NODE_SLUG` | Unique node identifier | Required |
| `NODE_NAME` | Human-readable node name | Optional |
| `NODE_REGISTRATION_PSK` | Pre-shared key for registration | Required |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |

## API Endpoints

### Authentication

- `POST /v1/auth/login` - Login with handle/password
- `POST /v1/auth/refresh` - Refresh access token
- `POST /v1/auth/logout` - Logout and invalidate tokens

### Users

- `GET /v1/users/me` - Get current user info

### Tasks

- `POST /v1/tasks` - Create a new task
- `GET /v1/tasks/{id}` - Get task by ID
- `GET /v1/tasks/{id}/result` - Get task result with jobs

### Nodes

- `POST /v1/nodes/register` - Register a new node (requires PSK)
- `POST /v1/nodes/capability` - Report node capabilities (requires node JWT)

## Development Commands

```bash
# Run all linters
just lint

# Run tests
just test-go

# Run tests with coverage
go test -cover ./...

# Format code
just fmt-go

# Run full CI check
just ci
```

## Troubleshooting

### PostgreSQL Connection Issues

1. Check if container is running:
   ```bash
   podman ps | grep cynodeai-postgres
   ```

2. Check PostgreSQL logs:
   ```bash
   podman logs cynodeai-postgres
   ```

3. Test connection:
   ```bash
   podman exec -it cynodeai-postgres psql -U postgres -d cynodeai -c "\dt"
   ```

### Orchestrator API Not Starting

1. Check environment variables are set
2. Check database connection
3. Review logs for errors

### Node Not Registering

1. Verify `NODE_REGISTRATION_PSK` matches orchestrator configuration
2. Check network connectivity to orchestrator
3. Review node logs for registration errors
