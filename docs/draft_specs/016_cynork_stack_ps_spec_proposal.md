# Proposed Spec: `cynork stack ps` (Local Stack Process and Container State)

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Relationship to Other Commands](#relationship-to-other-commands)
- [Proposed Command Surface](#proposed-command-surface)
  - [1 `stack` Group](#1-stack-group)
  - [2 `cynork stack ps` Operation](#2-cynork-stack-ps-operation)
- [Discovery Model](#discovery-model)
  - [Orchestrator Stack (Compose)](#orchestrator-stack-compose)
  - [Worker Node (Host)](#worker-node-host)
  - [Managed Containers (Node Manager)](#managed-containers-node-manager)
- [Proposed Output](#proposed-output)
  - [Example Stack Ps JSON Response](#example-stack-ps-json-response)
- [Flags and Configuration](#flags-and-configuration)
- [Exit Codes and Errors](#exit-codes-and-errors)
- [Security and Safety](#security-and-safety)
- [Implementation Notes (Go)](#implementation-notes-go)
- [Traceability](#traceability)
  - [Existing Requirements (Conceptual Fit)](#existing-requirements-conceptual-fit)
  - [Proposed New Requirement (For Incorporation Into `docs/requirements/client.md` When Promoted)](#proposed-new-requirement-for-incorporation-into-docsrequirementsclientmd-when-promoted)
  - [Specifications to Update When Promoting From Draft](#specifications-to-update-when-promoting-from-draft)
- [Open Points](#open-points)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-27
- Status: Proposal (`docs/draft_specs/`; not merged to `docs/tech_specs/`)
- Scope: Cynork subcommand to show **local** orchestrator compose services and **local** worker-node processes (node manager plus Worker API in one binary), and optionally containers the node manager supervises (for example Ollama), without calling the User API Gateway.

## Summary

Operators and developers today infer local stack state from `docker ps` / `podman ps`, compose project status, and ad hoc process checks for `cynodeai-wnm` (see [`worker_node/README.md`](../../worker_node/README.md)).
A single Cynork command, `cynork stack ps`, would present a **normalized, repo-aligned view**: which CyNodeAI-named containers are up, which host node process is listening on the expected Worker API port, and (when detectable) managed inference or agent-related containers, using the same default ports and naming conventions as [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md) and [`development_setup.md`](../development_setup.md).

## Goals

- One command answers: "Is my local CyNodeAI stack (orchestrator compose + node on this machine) up, and what is running?"
- Output works in **table** and **json** modes per existing Cynork global output rules ([`cynork_cli.md`](../tech_specs/cynork/cynork_cli.md#spec-cynai-client-clicommandsurface)).
- Behavior is **deterministic** when the container engine and paths are known; when something cannot be inspected (permissions, engine missing), the command reports `unknown` or a clear error rather than guessing.
- No gateway authentication required: the command is strictly **local introspection** (useful before login or when the gateway is down).

## Non-Goals

- Replacing `docker compose ps` or `podman compose ps` as the full source of truth for arbitrary compose projects; Cynork only projects CyNodeAI-relevant rows into its own format.
- Remote or multi-host cluster views (that remains orchestrator or `cynork nodes`-style data once those APIs exist).
- Replacing `cynork status` or a future gateway-backed detailed health endpoint (see [`012_status_command_detailed_health_spec_proposal.md`](012_status_command_detailed_health_spec_proposal.md)): `stack ps` is **local topology**, not **application health** or **authorized node registry** state.
- Defining a new Web Console page unless product later mandates parity for local-only diagnostics (currently treated as operator CLI convenience).

## Relationship to Other Commands

- **`cynork status`:** Continues to mean gateway (and optionally aggregated backend) health via HTTP. `stack ps` does not call `gateway_url` for its primary data path.
- **`cynork nodes` (when implemented):** Lists **registered** nodes and server-side fields. `stack ps` lists **this host's** processes and **this machine's** compose project, which may disagree with registration (for example node down but compose still up).

## Proposed Command Surface

This section defines the `stack` command group and the `ps` operation contract.

### 1 `stack` Group

- Proposed top-level command: `cynork stack` with subcommand `ps`.

### 2 `cynork stack ps` Operation

- Spec ID: `CYNAI.CLIENT.CliStackPs` <a id="spec-cynai-client-clistackps"></a>
- Kind: Operation (draft; not yet in canonical CLI spec).

#### 2.1 Operation Inputs

- Effective working directory: process CWD (no requirement to run from repo root unless discovery relies on relative paths; see [Discovery Model](#discovery-model)).
- Environment: inherits `PATH` for locating `docker` / `podman` / `docker compose` / `podman compose`.
- Optional flags (see [Flags and Configuration](#flags-and-configuration)).

#### 2.2 Operation Outputs

- Table or JSON document written to stdout per `-o` / `--output`.
- Human-readable status lines for non-fatal diagnostics MAY go to stderr in table mode; JSON mode MUST keep stdout as a single JSON value ([`cynork_cli.md`](../tech_specs/cynork/cynork_cli.md#spec-cynai-client-clicommandsurface)).

#### 2.3 Proposed Runtime Behavior

1. Resolve container engine (`auto`, `podman`, or `docker`) and compose CLI availability.
2. If orchestrator compose inspection is enabled, run an equivalent of `compose ps` against the configured compose file and project name; map services to logical **component** names (see [Proposed Output](#proposed-output)).
3. If local node inspection is enabled, detect a listening Worker API on the configured host/port (default loopback and port from [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md)); if listening, attempt to attribute the process to `cynodeai-wnm` (or build-specific binary name) via OS-specific APIs (Linux: `/proc`, `ss`/`lsof` fallback) without requiring root when possible.
4. Optionally query the same container engine for **well-known container names** the node manager creates (for example `cynodeai-ollama` per dev documentation), and merge into the result set with `kind=managed_container`.
5. Sort output: orchestrator compose services first (stable order by component name), then host process, then managed containers.
6. Exit 0 when the command completes and emits a result, even if some rows are `exited` or `unknown` (see [Exit Codes and Errors](#exit-codes-and-errors)).

#### 2.4 Draft Requirement Traces

- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004) (capability parity only if product adds a console equivalent; otherwise trace a future REQ-CLIENT entry proposed below).

## Discovery Model

The following subsections describe how implementations locate compose-backed services, the host node process, and node-manager-managed containers.

### Orchestrator Stack (Compose)

- Default compose file path: path relative to repository layout is **not** assumed unless the user passes `--compose-file` or sets an env var (proposed: `CYNORK_STACK_COMPOSE_FILE`).
  Otherwise implementations MAY search upward from CWD for `orchestrator/docker-compose.yml` up to a depth limit, or require an explicit path.
  **Open:** single canonical rule (see [Open Points](#open-points)).
- Default project name: align with how `just setup-dev` invokes compose (often directory-derived).
  Proposed env override: `CYNORK_STACK_COMPOSE_PROJECT`.
- Services of interest are those defined in the CyNodeAI orchestrator compose file with `container_name` prefixes or labels; at minimum map known names: `cynodeai-postgres`, `cynodeai-control-plane`, `cynodeai-user-gateway`, `cynodeai-ollama`, `cynodeai-minio`, `cynodeai-api-egress`, `cynodeai-mcp-gateway` (deprecated service may still appear when profile enabled).

### Worker Node (Host)

- Default Worker API listen address: `127.0.0.1:12090` unless overridden by `CYNORK_STACK_WORKER_API_URL` or flags (proposed `--worker-api-url`).
- Process identity: primary binary name `cynodeai-wnm`; dev builds may use `cynodeai-wnm-dev` or other suffix; matching SHOULD use executable basename prefix `cynodeai-wnm` to include dev variants.

### Managed Containers (Node Manager)

- Discovery via container engine listing filtered by `container_name` or compose labels the node manager sets.
  Exact names SHOULD be documented in [`worker_node.md`](../tech_specs/worker_node.md) when stabilized; until then implementations MAY hardcode the names used in current dev docs (for example `cynodeai-ollama`).

## Proposed Output

Each **row** represents one logical stack piece.
Proposed fields:

- `component` (string): stable logical id, for example `postgres`, `user_gateway`, `control_plane`, `ollama_compose`, `minio`, `api_egress`, `mcp_gateway_legacy`, `worker_node`, `ollama_node_managed`.
- `kind` (string enum): `compose_service` | `host_process` | `managed_container`.
- `display_name` (string): human label for table mode.
- `state` (string enum): `running` | `exited` | `paused` | `unknown` | `not_found` (not_found = expected component not present in compose listing and no container id).
- `engine` (string, optional): `docker` | `podman` when row comes from compose or container listing.
- `container_id` (string, optional): short id when available.
- `ports` (string, optional): host port mapping summary when available from compose ps (implementation may mirror compose column).
- `pid` (number, optional): host process id when `kind=host_process` and attribution succeeds.
- `listen` (string, optional): URL or host:port for Worker API when `kind=host_process`.
- `notes` (string, optional): short diagnostic, for example `compose_file_not_found` or `engine_not_in_path`.

**Table mode:** fixed column order: `COMPONENT`, `KIND`, `STATE`, `PORTS`, `PID`, `DETAIL` (DETAIL holds container id shortened or notes).

**JSON mode:** top-level object, for example:

### Example Stack Ps JSON Response

<a id="ref-json-cynork-stack-ps-response"></a>

```json
{
  "engine_resolved": "podman",
  "compose_file": "/abs/path/orchestrator/docker-compose.yml",
  "rows": [
    {
      "component": "user_gateway",
      "kind": "compose_service",
      "display_name": "User API Gateway",
      "state": "running",
      "engine": "podman",
      "container_id": "a1b2c3d4",
      "ports": "0.0.0.0:12080->12080/tcp",
      "pid": null,
      "listen": null,
      "notes": null
    },
    {
      "component": "worker_node",
      "kind": "host_process",
      "display_name": "Node manager / Worker API",
      "state": "running",
      "engine": null,
      "container_id": null,
      "ports": null,
      "pid": 12345,
      "listen": "127.0.0.1:12090",
      "notes": null
    }
  ]
}
```

## Flags and Configuration

All global flags from [`cynork_cli.md`](../tech_specs/cynork/cynork_cli.md#spec-cynai-client-clicommandsurface) apply (`--output`, `--quiet`, `--no-color`, `--config` ignored unless future stack config is stored in cynork config).

Proposed **command-specific** flags:

- `--engine` (string): `auto` (default), `docker`, `podman`.
  Auto prefers `podman` when both exist (align with project preference in [`system_reqs/common.md`](../system_reqs/common.md)).
- `--compose-file` (string): path to orchestrator `docker-compose.yml`.
- `--compose-project` (string): compose project name (`-p`).
- `--skip-compose` (bool): only inspect host node / managed containers.
- `--skip-node` (bool): only inspect compose.
- `--worker-api-url` (string): override URL for Worker API listen check (default `http://127.0.0.1:12090` or host:port derived from defaults).

Proposed **environment variables** (mirror flags when set):

- `CYNORK_STACK_ENGINE`
- `CYNORK_STACK_COMPOSE_FILE`
- `CYNORK_STACK_COMPOSE_PROJECT`
- `CYNORK_STACK_WORKER_API_URL`

## Exit Codes and Errors

Align with [`cynork_cli.md`](../tech_specs/cynork/cynork_cli.md#spec-cynai-client-cliexitcodes):

- `0`: Command completed and produced output (rows may include `exited` or `unknown`).
- `2`: Usage error (invalid flag combination, bad URL).
- `8`: Internal or environment error that prevents any useful inspection (for example no container engine found when compose inspection was requested and not skipped, or compose file path invalid and required).

Do **not** use gateway exit codes (`3`-`7`) for this command unless a future optional mode explicitly calls the gateway.

## Security and Safety

- The command MUST NOT print secrets (compose env files are not read; only `compose ps`-style data).
- Process attribution MUST NOT dump full command lines containing tokens; if argv is inspected, implementations SHOULD redact known flag patterns or skip argv and rely on binary path only.
- Local socket and port checks MUST default to loopback to avoid touching remote interfaces.

## Implementation Notes (Go)

- Prefer execing the compose CLI (same as operators) rather than linking Docker/Podman APIs, to match `just setup-dev` behavior and reduce coupling.
- Linux-first implementation is acceptable; other OS MAY return `unknown` for host process attribution with a note.
- Unit tests SHOULD use fake compose stdout fixtures; integration tests MAY be tagged and optional.

## Traceability

Links below map this draft to current requirements and to files that would change on promotion.

### Existing Requirements (Conceptual Fit)

- [REQ-CLIENT-0101](../requirements/client.md#req-client-0101): CLI management app scope; `stack ps` extends operator ergonomics without changing gateway contracts.

### Proposed New Requirement (For Incorporation Into `docs/requirements/client.md` When Promoted)

- REQ-CLIENT-0ZZZ (proposed): The CLI MUST provide a command to display local orchestrator compose service status and local worker-node process/listen status for the default development topology, without requiring gateway authentication.

### Specifications to Update When Promoting From Draft

- [`cynork_cli.md`](../tech_specs/cynork/cynork_cli.md): add `cynork stack ps` under command surface and link a small sub-doc or section for flags and JSON shape.
- [`cli_management_app_commands_core.md`](../tech_specs/cynork/cli_management_app_commands_core.md) or new `cli_management_app_commands_stack.md` for detail.

## Open Points

- Whether compose file discovery without flags MUST require running from inside the repo (simplest) or MUST search upward (more magic, easier UX).
- Whether `stack ps` should attempt HTTP `GET /healthz` on `127.0.0.1:12080` and `12082` as optional `--probe-http` without auth (blurs line with `cynork status`).
- Exact list of node-manager-created container names and labels for stable filtering across releases.
- CI story: BDD scenarios may need `podman`/`docker` skips when engine absent (similar to other environment-dependent tests).

## References

- [`docs/tech_specs/cynork_cli.md`](../tech_specs/cynork/cynork_cli.md)
- [`docs/tech_specs/ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md)
- [`docs/development_setup.md`](../development_setup.md)
- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md)
- [`orchestrator/docker-compose.yml`](../../orchestrator/docker-compose.yml)
- [`docs/draft_specs/012_status_command_detailed_health_spec_proposal.md`](012_status_command_detailed_health_spec_proposal.md)
