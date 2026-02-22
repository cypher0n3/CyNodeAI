# Worker Node Systemd Units (Podman)

- [Overview](#overview)
- [Generate Units](#generate-units)
- [Install](#install)

## Overview

Generated unit files for running worker-api and node-manager under systemd.

## Generate Units

From repo root:

```bash
podman compose -f worker_node/docker-compose.yml up -d
./scripts/podman-generate-units.sh worker_node
podman compose -f worker_node/docker-compose.yml down  # optional
```

## Install

Use rootless (user) or rootful install as below.

### Rootless Install

```bash
mkdir -p ~/.config/systemd/user
cp worker_node/systemd/container-*.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now container-cynodeai-worker-api.service
systemctl --user enable --now container-cynodeai-node-manager.service
```

Ensure `ORCHESTRATOR_URL` (and shared `WORKER_API_BEARER_TOKEN` / `NODE_REGISTRATION_PSK`) are set in the environment or in the generated unit files.
