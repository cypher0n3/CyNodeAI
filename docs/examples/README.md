# Example Configuration Files

- [Overview](#overview)

## Overview

Example YAML configuration files for orchestrator and worker (node) bootstrap.
These are for reference and local development; do not commit secrets.

- **Orchestrator bootstrap:** [`orchestrator_bootstrap_example.yaml`](orchestrator_bootstrap_example.yaml)
  - Loaded by the orchestrator at startup to seed PostgreSQL (user task-execution preferences, access control, node registration).
- **Node (worker) bootstrap:** [`node_bootstrap_example.yaml`](node_bootstrap_example.yaml)
  - Node startup config: orchestrator URL, registration PSK reference, node identity, Worker API and sandbox settings.

Specs: [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md), [`docs/tech_specs/node.md`](../tech_specs/node.md).
