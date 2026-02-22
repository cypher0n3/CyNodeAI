# Canonical Requirements Domains

## Overview

This document is the **single source of truth** for CyNodeAI requirements domains.
Spec IDs (`CYNAI.<Domain>.<Path>`) and requirement IDs (`REQ-<DOMAIN>-NNNN`) MUST use only domains listed here.

## Domain Tag Convention

Domain tags use a **fixed-length pattern**: exactly **6 uppercase letters** (A-Z), no underscores.
All canonical domain tags in this document follow this pattern.

## Domain List

Each domain maps to a single requirements file under `docs/requirements/`.
The filename is the domain tag lowercased.

- `ACCESS`: Authorization, access control, RBAC, groups, scopes, and policy enforcement.
  - Requirements file: [`docs/requirements/access.md`](../requirements/access.md)
- `IDENTY`: Identity and authentication, including local user accounts and session lifecycle.
  - Requirements file: [`docs/requirements/identy.md`](../requirements/identy.md)
- `USRGWY`: User-facing API gateway behavior and contracts.
  - Requirements file: [`docs/requirements/usrgwy.md`](../requirements/usrgwy.md)
- `DATAPI`: Data REST API behavior, contracts, and data access semantics.
  - Requirements file: [`docs/requirements/datapi.md`](../requirements/datapi.md)
- `ORCHES`: Orchestrator control-plane behavior, task lifecycle, dispatch, and state management.
  - Requirements file: [`docs/requirements/orches.md`](../requirements/orches.md)
- `PROJCT`: Project entity, storage, schema, user-facing title and description, and project-scoped scope model.
  - Requirements file: [`docs/requirements/projct.md`](../requirements/projct.md)
- `WORKER`: Worker-node behavior and the worker API contract for job execution and reporting.
  - Requirements file: [`docs/requirements/worker.md`](../requirements/worker.md)
- `SANDBX`: Sandbox execution model, container constraints, and isolation.
  - Requirements file: [`docs/requirements/sandbx.md`](../requirements/sandbx.md)
- `APIEGR`: Controlled egress services, including API egress and git egress behavior.
  - Requirements file: [`docs/requirements/apiegr.md`](../requirements/apiegr.md)
- `WEBPRX`: Web Egress Proxy: allowlist-based HTTP(S) forward proxy for sandbox dependency downloads.
  - Requirements file: [`docs/requirements/webprx.md`](../requirements/webprx.md)
- `MCPGAT`: MCP gateway enforcement, auditing, and policy controls for tool invocation.
  - Requirements file: [`docs/requirements/mcpgat.md`](../requirements/mcpgat.md)
- `MCPTOO`: MCP tooling, tool catalog conventions, and SDK installation and integration.
  - Requirements file: [`docs/requirements/mcptoo.md`](../requirements/mcptoo.md)
- `SCHEMA`: Data persistence requirements, schema-level invariants, and database constraints.
  - Requirements file: [`docs/requirements/schema.md`](../requirements/schema.md)
- `MODELS`: Model lifecycle requirements, external model routing, and model management.
  - Requirements file: [`docs/requirements/models.md`](../requirements/models.md)
- `AGENTS`: Agent behaviors, responsibilities, and workflow integration.
  - Requirements file: [`docs/requirements/agents.md`](../requirements/agents.md)
- `CLIENT`: User-facing management surfaces, including CLI and admin web console behavior.
  - Requirements file: [`docs/requirements/client.md`](../requirements/client.md)
- `CONNEC`: Connector framework requirements and external connector integration patterns.
  - Requirements file: [`docs/requirements/connec.md`](../requirements/connec.md)
- `BROWSR`: Secure browser service requirements, rules, and deterministic sanitization.
  - Requirements file: [`docs/requirements/browsr.md`](../requirements/browsr.md)
- `BOOTST`: Bootstrap configuration requirements for orchestrator and node startup and configuration delivery.
  - Requirements file: [`docs/requirements/bootst.md`](../requirements/bootst.md)
- `STANDS`: Cross-cutting standards and conventions (for example REST API standards) that apply across components.
  - Requirements file: [`docs/requirements/stands.md`](../requirements/stands.md)
- `SKILLS`: AI skills file storage, tracking, and exposure to inference models that support skills.
  - Requirements file: [`docs/requirements/skills.md`](../requirements/skills.md)
