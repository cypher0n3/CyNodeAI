# Requirements

- [1 Overview](#1-overview)
- [2 Conventions](#2-conventions)
- [3 Requirements Domains Index](#3-requirements-domains-index)

## 1 Overview

This directory contains the consolidated CyNodeAI requirements set.
Requirements documents are the canonical normative "what" for CyNodeAI.
Technical specifications under [`docs/tech_specs/`](../tech_specs/) describe the "how" (design and implementation guidance) and trace back to requirements.

## 2 Conventions

- Authoring standards:
  - [`docs/docs_standards/markdown_conventions.md`](../docs_standards/markdown_conventions.md)
  - [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs_standards/spec_authoring_writing_and_validation.md)
- Canonical domains:
  - [`docs/docs_standards/requirements_domains.md`](../docs_standards/requirements_domains.md)
- Requirement IDs:
  - Format: `REQ-<DOMAIN>-<NNNN>`.
  - Example: `REQ-ACCESS-0001`.
  - Allocation scheme:
    - `REQ-<DOMAIN>-0001` and up: coarse, capability-level requirements (high-level).
    - `REQ-<DOMAIN>-0100` and up: atomic, testable requirements (preferred granularity for BDD/Godog scenarios).
- Requirement anchors:
  - Format: `req-<domain>-<nnnn>`.
  - Example: `REQ-ACCESS-0001` => `req-access-0001`.
  - Anchors MUST appear on a continuation line after the `- REQ-<DOMAIN>-<NNNN>: ...` line and after any spec reference link lines, as `<a id="..."></a>`.
- Traceability:
  - Every requirement item MUST include at least one link into [`docs/tech_specs/`](../tech_specs/) (typically to `spec-*` anchors) describing implementation guidance.
  - Prefer Spec ID anchors when they exist.
  - When Spec ID anchors are not available, link to the most specific relevant tech spec document section.

## 3 Requirements Domains Index

Each domain maps to one file.
The filename is the domain tag lowercased.

- `ACCESS`: [access.md](./access.md)
- `IDENTY`: [identy.md](./identy.md)
- `USRGWY`: [usrgwy.md](./usrgwy.md)
- `DATAPI`: [datapi.md](./datapi.md)
- `ORCHES`: [orches.md](./orches.md)
- `WORKER`: [worker.md](./worker.md)
- `SANDBX`: [sandbx.md](./sandbx.md)
- `APIEGR`: [apiegr.md](./apiegr.md)
- `MCPGAT`: [mcpgat.md](./mcpgat.md)
- `MCPTOO`: [mcptoo.md](./mcptoo.md)
- `SCHEMA`: [schema.md](./schema.md)
- `MODELS`: [models.md](./models.md)
- `AGENTS`: [agents.md](./agents.md)
- `CLIENT`: [client.md](./client.md)
- `CONNEC`: [connec.md](./connec.md)
- `BROWSR`: [browsr.md](./browsr.md)
- `BOOTST`: [bootst.md](./bootst.md)
- `STANDS`: [stands.md](./stands.md)
- `SKILLS`: [skills.md](./skills.md)
