# Orchestrator Systemd Units (Podman)

Generated unit files allow running the orchestrator stack under systemd (e.g. rootless user services).

## Generate Units

From repo root, after containers exist (e.g. started once with compose):

```bash
# Start stack so containers exist
podman compose -f orchestrator/docker-compose.yml up -d

# Generate systemd units
./scripts/podman-generate-units.sh orchestrator

# Optional: stop compose-managed containers so systemd can own them
podman compose -f orchestrator/docker-compose.yml down
```

## Install

### Rootless (Recommended):

```bash
mkdir -p ~/.config/systemd/user
cp orchestrator/systemd/container-*.service ~/.config/systemd/user/
systemctl --user daemon-reload
# Start in dependency order: postgres first, then control-plane, user-gateway
systemctl --user enable --now container-cynodeai-postgres.service
systemctl --user enable --now container-cynodeai-control-plane.service
systemctl --user enable --now container-cynodeai-user-gateway.service
```

### Rootful:

```bash
sudo cp orchestrator/systemd/container-*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now container-cynodeai-postgres.service
# ... etc.
```

## Order

Start postgres first, then control-plane (runs migrations), then user-gateway.
Optional: mcp-gateway, api-egress.
