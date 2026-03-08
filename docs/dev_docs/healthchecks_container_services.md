# Container Health Checks

- [Orchestrator Stack (Compose)](#orchestrator-stack-compose)
- [Worker Node (Compose)](#worker-node-compose)
- [Worker Managed Services (Node-Manager)](#worker-managed-services-node-manager)
- [Setup-Dev (Python)](#setup-dev-python)

## Orchestrator Stack (Compose)

Summary: health check support so containerized services report `(healthy)` when run via compose, worker managed services, or setup-dev.

**File:** `orchestrator/docker-compose.yml`

- **postgres** - unchanged (already had `pg_isready` healthcheck).
- **control-plane** - `GET http://localhost:12082/healthz` (wget), interval 10s, start_period 5s.
- **user-gateway** - `GET http://localhost:12080/healthz`, same tuning.
- **mcp-gateway** - `GET http://localhost:12083/healthz`, same tuning.
- **api-egress** - `GET http://localhost:12084/healthz`, same tuning.
- **ollama** (profile `ollama`) - `GET http://localhost:11434/`, interval 15s, start_period 30s.

All use `CMD-SHELL` with `wget -q -O /dev/null <url> || exit 1`.
Alpine-based images use busybox wget.

## Worker Node (Compose)

**File:** `worker_node/docker-compose.yml`

- **worker-api** - `GET http://localhost:12090/healthz`, interval 10s, start_period 5s.

## Worker Managed Services (Node-Manager)

When the node-manager starts a managed service container (e.g. PMA) and the orchestrator sends a `healthcheck` in the config (path + expected_status), the node adds **Podman-only** health check options to the `run` command so `podman ps` shows `(healthy)`.

- **Implementation:** `worker_node/internal/nodeagent/runargs.go` - `BuildManagedServiceRunArgs(..., runtime)`; when `runtime == "podman"` and `svc.Healthcheck != nil`, appends `--health-cmd`, `--health-interval`, `--health-timeout`, `--health-retries`, `--health-start-period`.
  Uses wget to hit the configured path (default `/healthz`) on the service's default port (e.g. 8090 for PMA).
- **Docker:** `docker run` does not support inline health checks; managed services started with Docker do not get a runtime healthcheck (behavior unchanged).

Orchestrator already sends PMA healthcheck in node config (`/healthz`, 200) per `orchestrator/internal/handlers/nodes.go`.

## Setup-Dev (Python)

**File:** `scripts/setup_dev_impl.py`

- **Postgres (standalone `start-db`):** When the container runtime is **podman**, the `run` command now includes `--health-cmd pg_isready -U ... -d ...`, `--health-interval 2s`, `--health-timeout 2s`, `--health-retries 30`.
  When runtime is Docker, no health args are added (Docker run does not support them).

Orchestrator stack started by setup-dev uses the same `orchestrator/docker-compose.yml`, so all compose-based services get their healthchecks from the compose file.
