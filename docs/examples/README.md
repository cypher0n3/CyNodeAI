# Example Configuration Files

- [Overview](#overview)

## Overview

Example YAML configuration files for orchestrator and worker (node) bootstrap.
These are for reference and local development; do not commit secrets.

- **Orchestrator bootstrap:** [`orchestrator_bootstrap_example.yaml`](orchestrator_bootstrap_example.yaml)
  - Loaded by the orchestrator at startup to seed PostgreSQL with system-scoped defaults and integration configuration.
- **Node (worker) bootstrap:** [`node_bootstrap_example.yaml`](node_bootstrap_example.yaml)
  - Node startup config including orchestrator URL, registration PSK reference, node identity, Worker API settings, and sandbox defaults.

See [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md), [worker_node.md](../tech_specs/worker_node.md), and [docs/README.md](../README.md).
