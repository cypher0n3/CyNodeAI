# Orchestrator Bootstrap Configuration

- [Document Overview](#document-overview)
- [Bootstrap Goal](#bootstrap-goal)
- [Bootstrap Source and Precedence](#bootstrap-source-and-precedence)
- [Bootstrap Contents](#bootstrap-contents)
- [Standalone Operation Mode](#standalone-operation-mode)

## Document Overview

This document defines how the orchestrator can load a bootstrap configuration at startup from a YAML file.
Bootstrap configuration is used to seed PostgreSQL and configure external service integration.

## Bootstrap Goal

- Provide a repeatable way to initialize an orchestrator deployment.
- Seed required preferences, access control rules, and external service configuration into PostgreSQL.
- Support running the orchestrator as the sole service with no worker nodes.

## Bootstrap Source and Precedence

Bootstrap YAML is an import mechanism, not the source of truth.
The source of truth for configuration and preferences is PostgreSQL.

Normative requirements

- On startup, the orchestrator MAY load a bootstrap YAML file when configured to do so.
- Bootstrap import MUST be idempotent and SHOULD support updates using versioned keys.
- After import, agents and systems MUST read configuration and preferences from PostgreSQL via MCP tools or the User API Gateway.

## Bootstrap Contents

Bootstrap YAML SHOULD support seeding:

- System preferences (entries in `preference_entries` with `scope_type` system)
- Access control rules and default policy
- External MCP servers to connect to, with allowlists
- Sandbox image registry configuration (registry URL, mode, policy defaults)
- Model management defaults (cache limits and download policy)
- External model routing defaults (allowed providers and fallback order)
- Orchestrator-side agent external provider defaults (Project Manager and Project Analyst routing preferences)

Secrets

- Secrets SHOULD NOT be stored directly in YAML.
- If secrets must be provisioned at bootstrap time, they SHOULD be provided via environment variables or an external secrets manager and written to PostgreSQL encrypted.

## Standalone Operation Mode

The orchestrator SHOULD support running as the sole service with zero worker nodes.

Recommended behavior

- If there are no registered worker nodes, the orchestrator MAY route model calls to external providers when policy allows it.
- External model calls MUST use the API Egress Server so API keys are not exposed to agents.
- Sandbox execution SHOULD be disabled or restricted when no worker nodes are available.
