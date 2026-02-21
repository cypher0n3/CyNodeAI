# Scripts

- [Overview](#overview)
- [Available Scripts](#available-scripts)
- [Cross-References](#cross-references)

## Overview

This directory holds helper scripts for development and deployment.
Prefer the root [justfile](../justfile) for day-to-day commands; these scripts support one-off or advanced setups.

## Available Scripts

- **dev-setup.sh** - Sets up and runs the end-to-end development environment (Postgres, orchestrator, worker node).
  Use from repo root; see [docs/development_setup.md](../docs/development_setup.md) for full setup options.
- **setup-dev.sh** - Alternative development setup script for the E2E demo; requires Podman or Docker, Go 1.25+, and psql.
- **podman-generate-units.sh** - Generates systemd unit files for Podman containers from docker-compose.
  Run from repo root after bringing up the stack (e.g. `podman compose up -d`).
  Usage: `./scripts/podman-generate-units.sh [orchestrator|worker_node|all]`.
  See [orchestrator/systemd/README.md](../orchestrator/systemd/README.md) and [worker_node/systemd/README.md](../worker_node/systemd/README.md).

## Cross-References

- Documentation: [docs/README.md](../docs/README.md).
- Development setup: [docs/development_setup.md](../docs/development_setup.md).
- Root project: [README.md](../README.md), [justfile](../justfile).
