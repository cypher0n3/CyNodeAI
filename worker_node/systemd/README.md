# Worker Node Systemd Units (Podman)

- [Overview](#overview)
- [Generate Units](#generate-units)
- [Install](#install)
- [Node-Manager on the Host](#node-manager-on-the-host)

## Overview

Generated unit files for running the worker-api container under systemd.
Node-manager is intended to run on the host (not in a container) so it can manage podman/docker for PMA and sandboxes; see [Node-Manager on the Host](#node-manager-on-the-host).

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
```

Ensure `WORKER_API_BEARER_TOKEN` and (if using the API) other env are set in the environment or in the generated unit file.

## Node-Manager on the Host

For full node behavior (register with orchestrator, fetch config, start PMA and sandbox containers), run node-manager on the host so it can call podman/docker.
Use the repo `just setup-dev start` flow (orchestrator + node-manager binary) or run the node-manager binary with the right env; it will start worker-api and manage containers.
The worker_node compose stack is worker-api only.
