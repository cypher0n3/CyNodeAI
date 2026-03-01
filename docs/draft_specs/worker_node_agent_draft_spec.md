# Worker Node Agent (WNA) - Draft Spec

- [Document Overview](#document-overview)
  - [Related Specs](#related-specs)
- [Scope and Goals](#scope-and-goals)
- [Definitions](#definitions)
- [Worker Node Agent vs Container SBA](#worker-node-agent-vs-container-sba)
- [Job Acquisition and Worker API](#job-acquisition-and-worker-api)
- [WNA-Only Execution Mode](#wna-only-execution-mode)
- [Single Job at a Time and Status Reporting](#single-job-at-a-time-and-status-reporting)
- [Worker Node Agent Configuration](#worker-node-agent-configuration)
  - [Configuration Source and Schema](#configuration-source-and-schema)
  - [Restricted Commands](#restricted-commands)
  - [Elevation Allowed](#elevation-allowed)
  - [Other Configurable Constraints](#other-configurable-constraints)
- [Configuration Application and Refresh](#configuration-application-and-refresh)
- [Security and Enforcement](#security-and-enforcement)
- [Traceability](#traceability)

## Document Overview

This draft spec defines the **Worker Node Agent (WNA)**: the host-level worker agent running **on the host directly** as a long-lived agent on that worker node.
It still obtains jobs (and related context) from the CyNode ecosystem (Worker API / orchestrator) but executes on the host instead of inside a sandbox container.
Because the host has no container boundary, the WNA MUST receive and enforce a **restricted commands** list and other policy delivered via configuration.

Status: Draft in `dev_docs`; not yet promoted to `docs/tech_specs/` or `docs/requirements/`.

### Related Specs

- [cynode_sba.md](../tech_specs/cynode_sba.md): SBA behavior, job spec, result contract, MCP tools
- [worker_node.md](../tech_specs/worker_node.md): Node Manager, configuration delivery, Worker API
- [worker_api.md](../tech_specs/worker_api.md): Job dispatch, run job contract
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md): Node configuration payload, capability report

## Scope and Goals

- **In scope:** Definition of the Worker Node Agent (WNA) mode; how it obtains jobs; configuration schema and semantics for restricted commands, elevation, and other WNA policy; how config is sourced and refreshed.
- **Out of scope:** Scheduling policy (which jobs go to WNA vs sandbox); implementation details of the SBA binary's host vs container detection; MCP gateway changes specific to WNA identity (may be covered elsewhere).
- **Goals:** A single place to specify WNA behavior and configuration so that operators can deploy a worker node agent with clear, auditable restrictions (commands, paths, timeouts, elevation, etc.) and so implementers can verify compliance.

## Definitions

- **Worker Node Agent (WNA):** An instance of the SBA runner that runs as a process on the host (not inside a sandbox container).
  It is long-lived on that worker node and processes jobs one at a time to completion or failure, then may accept the next; as opposed to ephemeral per-job sandbox containers, which are destroyed after each job.
- **Restricted commands:** A list of commands (and/or patterns) that the WNA is **forbidden** to execute (denylist), or alternatively an allowlist of commands the agent **may** execute; see [Restricted Commands](#restricted-commands).
- **Worker API:** The HTTP API exposed by the node; the orchestrator dispatches jobs to the node via this API.
  The WNA obtains work through the same job pipeline (jobs received by the node via the Worker API and assigned to the WNA by the node, or by the WNA polling a node-/orchestrator-provided job endpoint).

## Worker Node Agent vs Container SBA

- Spec ID: `CYNAI.HOSTAG.HostVsContainer` <a id="spec-cynai-hostagent-hostvscontainer"></a>

- **Aspect:** Execution boundary
  - container sba: Inside sandbox container (rootless, isolated)
  - WNA: Directly on host process
- **Aspect:** Command/path policy
  - container sba: No allowlist inside container; boundary is the container
  - WNA: **MUST** enforce restricted-commands (and optional path) policy from config
- **Aspect:** Job source
  - container sba: Node starts container; job spec in `/job/job.json`
  - WNA: Same job contract; jobs from Worker API (node-assigned or agent-polled)
- **Aspect:** Lifecycle
  - container sba: Per-job container; destroyed after job completes or fails
  - WNA: Long-lived process; one job at a time to completion or failure, then may accept next
- **Aspect:** Inference
  - container sba: Via node-local proxy in pod/network
  - WNA: Host can use node-local inference or configured endpoint
- **Aspect:** MCP / egress
  - container sba: Via worker proxies (inference, web, API egress)
  - WNA: Same; WNA MUST use same controlled egress (no direct internet)

The WNA SHALL use the same job specification schema, result contract, and lifecycle signaling as the container SBA per [cynode_sba.md](../tech_specs/cynode_sba.md).
It SHALL comply with [REQ-SBAGNT-0102](../requirements/sbagnt.md#req-sbagnt-0102) (timeouts, output limits) and SHALL enforce **additional** host-specific constraints defined in this spec (restricted commands, elevation policy, and config-driven policy).
When `elevation_allowed` is false (default), the WNA MUST run as non-root; when true, see [Elevation Allowed](#elevation-allowed).

## Job Acquisition and Worker API

- Spec ID: `CYNAI.HOSTAG.JobAcquisition` <a id="spec-cynai-hostagent-jobacquisition"></a>

The WNA MUST obtain jobs from the same ecosystem as sandbox execution:

- Jobs are dispatched by the orchestrator to the **node** via the **Worker API** (see [worker_api.md](../tech_specs/worker_api.md)).
- The **node** MAY assign one or more jobs to the WNA (e.g. by passing job payloads to the WNA process, or by the WNA polling a node-owned endpoint that returns the next job for this agent).
- Alternatively, the WNA MAY be configured with an endpoint (node or orchestrator) to **poll** for jobs assigned to it; that endpoint and assignment policy are out of scope for this draft but MUST still use the same job payload shape and versioning as the Worker API run-job contract.

The WNA MUST NOT bypass the Worker API / orchestrator to accept arbitrary job payloads from untrusted sources; job input MUST come from the configured node or orchestrator and MUST be authenticated/authorized per existing Worker API and orchestrator contracts.

## WNA-Only Execution Mode

- Spec ID: `CYNAI.HOSTAG.WnaOnlyMode` <a id="spec-cynai-hostagent-wnaonlymode"></a>

When a node is configured with a WNA (e.g. `worker_node_agent.enabled` is true in the node configuration payload or equivalent), the node MUST use **only** the WNA for any jobs it receives.

- The node MUST NOT start sandbox containers for job execution when WNA is enabled.
- All jobs dispatched to that node MUST be executed by the WNA; the node MUST NOT offer or use sandbox-based SBA (or step-executor) runners for those jobs.
- If the WNA is unavailable (e.g. not running or not ready), the node MUST either refuse the job (e.g. return an error or status that allows the orchestrator to retry or reassign) or queue it until the WNA is ready, as defined by the Worker API and node implementation contract.

## Single Job at a Time and Status Reporting

- Spec ID: `CYNAI.HOSTAG.SingleJobAndStatus` <a id="spec-cynai-hostagent-singlejobandstatus"></a>

All worker agents (container SBA and WNA) run **one job at a time** to completion or failure and MUST report status back to the orchestrator (via the node) as defined by the job lifecycle and Worker API.

- **Single job at a time:** A worker agent MUST NOT run more than one job concurrently.
  It MUST accept the next job only after the current job has reached a terminal state (completed, failed, or timeout) and the result (or failure) has been reported.
- **Status reporting:** The agent MUST report status per [cynode_sba.md](../tech_specs/cynode_sba.md) and [worker_api.md](../tech_specs/worker_api.md): e.g. in progress (after accepting and validating the job), then completed or failed (with result contract or error).
  The node MUST relay or derive these status updates to the orchestrator so that job state is accurate and the orchestrator can schedule other work accordingly.
- **Container SBA lifecycle:** A sandbox-based SBA runs inside a per-job container; that container is **destroyed** after the job completes or fails (and the result is reported).
  There is no long-lived agent process for container SBA; each job gets a new container that is torn down when the job ends.
- **WNA lifecycle:** The WNA is long-lived but still processes one job at a time.
  After reporting completion or failure for the current job, the WNA MAY accept the next job from the node.
  The WNA MUST NOT accept a new job until the current job has reached a terminal state and status has been reported.

## Worker Node Agent Configuration

- Spec ID: `CYNAI.HOSTAG.Configuration` <a id="spec-cynai-hostagent-configuration"></a>

The WNA MUST be driven by a **configuration** that defines at least:

- **Restricted commands** (denylist or allowlist); see [Restricted Commands](#restricted-commands).
- **Elevation allowed** (boolean, default false); see [Elevation Allowed](#elevation-allowed).
- **Other configurable constraints** (e.g. allowed paths, timeouts, allowed MCP tools, inference allowlist); see [Other Configurable Constraints](#other-configurable-constraints).

Configuration MAY be supplied by:

1. **Node configuration payload:** The orchestrator includes a `worker_node_agent` (or equivalent) section in the node configuration payload delivered to the node; the node passes the relevant subset to the WNA (e.g. on startup or on config refresh).
2. **Host-local config file:** A file on the host (e.g. YAML or JSON) that the WNA reads at startup and optionally watches for changes.
3. **Combination:** Defaults or overrides from host-local file; overlays or mandatory policy from node configuration payload when the node manages the WNA.

When both node configuration payload and host-local config exist, the spec does not mandate a single merge order; a future revision or implementation spec MUST define precedence (e.g. node payload overrides host file for security-critical fields such as restricted commands and `elevation_allowed`).

### Configuration Source and Schema

- Spec ID: `CYNAI.HOSTAG.ConfigSchema` <a id="spec-cynai-hostagent-configschema"></a>

A minimal schema for WNA configuration is defined below.
Payload names and structure are illustrative; the canonical wire format (e.g. inside `node_configuration_payload_v1`) MUST be defined when this spec is promoted and integrated with [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).

#### Suggested Top-Level Keys (YAML/JSON)

- `worker_node_agent` (object, optional at node level)
  - `enabled` (boolean, optional): When true, the node MUST use only the WNA for all jobs it receives (see [WNA-Only Execution Mode](#wna-only-execution-mode)); the node may start or manage the WNA.
    When false, the node MUST NOT assign jobs to a WNA.
  - `restricted_commands` (object): See [Restricted Commands](#restricted-commands).
    May include per-command allowed subcommands and allowed/denied switches.
  - `primary_working_directory` (string, optional): Absolute path on the host where the WNA executes its work.
    When set, the WNA MUST use this directory as the default working directory for job execution (e.g. for `run_command`, file tools).
    When absent, an implementation-defined default or per-job workspace may apply.
    See [Other Configurable Constraints](#other-configurable-constraints).
  - `allowed_paths` (array of strings, optional): Allowed filesystem path prefixes for read/write; when present, the agent MUST NOT read or write outside these prefixes.
  - `allowed_commands` (array of strings or objects, optional): If present and non-empty, treated as **allowlist**: only these commands (or patterns) may be executed; see [Restricted Commands](#restricted-commands).
    When entries are objects, each MAY include `command`, `allowed_subcommands`, `allowed_switches`, `denied_switches` to constrain subcommands and switches.
  - `elevation_allowed` (boolean, optional): When true, the WNA MAY run commands as root or via sudo (or equivalent); when false or absent, the WNA MUST NOT use elevation.
    Default: false.
    See [Elevation Allowed](#elevation-allowed).
  - `max_job_timeout_seconds` (int, optional): Override for per-job timeout.
  - `max_output_bytes` (int, optional): Max stdout/stderr capture per tool/command.
  - `allowed_mcp_tools` (array of strings, optional): Subset of sandbox-allowlist MCP tools permitted for this WNA; when absent, default is full sandbox allowlist per [cynode_sba.md](../tech_specs/cynode_sba.md).
  - `inference` (object, optional): Endpoint or allowlist for inference (e.g. `allowed_models`, `base_url`); when absent, use node default or job-defined inference.

### Restricted Commands

- Spec ID: `CYNAI.HOSTAG.RestrictedCommands` <a id="spec-cynai-hostagent-restrictedcommands"></a>

Because the WNA runs on the host without a container boundary, it MUST enforce a **restricted commands** policy from configuration.
The policy MUST support tracking **allowed subcommands** and **allowed or denied switches** per command (when configured).

- **Denylist mode:** Configuration supplies a list of **forbidden** commands (or patterns), and MAY supply per-command forbidden subcommands or switches.
  - Example: `restricted_commands.deny: ["rm", "mv", "dd", "mkfs.*", "/usr/sbin/*"]`
  - Example (subcommands/switches): deny `git push`, or deny any command that includes switch `--force` or `--unsafe-perm`.
  - The agent MUST refuse to execute any command that matches the denylist (e.g. exact binary name, or pattern match as defined by the implementation), and MUST refuse when a configured forbidden subcommand or switch is present.
- **Allowlist mode:** Configuration supplies a list of **allowed** commands (or patterns), and MAY supply per-command **allowed subcommands** and **allowed or denied switches**.
  - Example (simple): `allowed_commands: ["/usr/bin/python3", "/usr/bin/go", "git", "npm"]`
  - Example (with subcommands/switches): for `git`, allow only subcommands `status`, `diff`, `log`, `clone`; for `npm`, allow only subcommand `install` and deny switch `--unsafe-perm`.
  - When `allowed_commands` (or equivalent) is present and non-empty, the agent MUST execute **only** commands that match the allowlist; all others are denied.
  - When per-command `allowed_subcommands` is specified, the agent MUST refuse execution if the first argument (subcommand) is not in that list.
  - When per-command `allowed_switches` or `denied_switches` is specified, the agent MUST parse the command line for options/switches and MUST refuse execution if a denied switch is present or if only allowed switches are listed and an unknown switch is present.
- At least one of denylist or allowlist MUST be configurable; implementations MAY support both (e.g. allowlist with optional denylist exclusions).

Pattern semantics (glob vs regex, path normalization, and how subcommands/switches are parsed and matched) MUST be defined in the implementation spec or in a later revision of this document (e.g. prefix match for paths, exact binary name vs path, first token as subcommand, long/short form of switches).

### Elevation Allowed

- Spec ID: `CYNAI.HOSTAG.ElevationAllowed` <a id="spec-cynai-hostagent-elevationallowed"></a>

- **Default:** `elevation_allowed` MUST default to **false** when absent or unspecified.
- **When false:** The WNA MUST run as a non-root user and MUST NOT execute commands via sudo, su, or any equivalent privilege elevation.
  Any attempt to run an elevated command MUST be refused and MUST be logged with a clear warning (e.g. command, job_id, reason).
- **When true:** The WNA MAY run commands as root or via sudo (or equivalent) when the job or tool call requires it.
  Each use of elevation MUST be **warned** and **clearly logged**: the implementation MUST log at least command (or tool name), job_id, task_id, and a stable log level/event type (e.g. "elevation_used" or "wna_elevation") so operators can audit and alert on elevated execution.
  Logs SHOULD be structured (e.g. JSON or key-value) and SHOULD include timestamp and node/host identifier.
- **Security:** Enabling elevation increases risk; operators SHOULD use elevation only when necessary and SHOULD monitor logs for elevation events.

### Other Configurable Constraints

- Spec ID: `CYNAI.HOSTAG.OtherConstraints` <a id="spec-cynai-hostagent-otherconstraints"></a>

The WNA SHOULD respect the following when present in configuration:

- **Primary working directory:** `primary_working_directory` (or equivalent): When set, the WNA MUST use this absolute path as the default working directory for all job execution (e.g. commands run via `run_command`, file read/write tools).
  The directory MUST exist and be usable by the WNA process (read/write as appropriate).
  When `allowed_paths` is also set, the primary working directory SHOULD be within or consistent with the allowed path prefixes so that work stays within policy.
- **Allowed paths:** `allowed_paths` (or equivalent): Only allow read/write under these path prefixes; symlink escape outside these prefixes MUST be rejected.
- **Timeouts:** `max_job_timeout_seconds`, per-command or per-tool timeouts; the agent MUST enforce them and report timeout per [cynode_sba.md](../tech_specs/cynode_sba.md) result contract.
- **Output limits:** `max_output_bytes` for stdout/stderr capture; truncation and inclusion in result per existing SBA behavior.
- **MCP tools:** `allowed_mcp_tools` restricts which MCP tools the WNA may call; MUST be a subset of the sandbox allowlist; when absent, full sandbox allowlist applies.
- **Inference:** Inference endpoint and model allowlist may be narrowed for the WNA (e.g. only certain models or only node-local inference).

## Configuration Application and Refresh

- Spec ID: `CYNAI.HOSTAG.ConfigRefresh` <a id="spec-cynai-hostagent-configrefresh"></a>

- The WNA MUST load configuration at startup and MUST NOT execute jobs until a valid configuration (including at least restricted-commands policy) is applied.
- When configuration is delivered via the **node configuration payload**, the node SHOULD pass updates to the WNA when the node applies a new config version (e.g. on config refresh or poll).
- The WNA SHOULD apply new configuration without requiring a process restart when the update is non-breaking (e.g. tightening allowlist or denylist, or updating timeouts).
- If the new configuration cannot be applied safely in-place (e.g. structural change), the agent MAY require a restart or signal the node to restart it; the contract for that is out of scope for this draft.

## Security and Enforcement

- Spec ID: `CYNAI.HOSTAG.SecurityEnforcement` <a id="spec-cynai-hostagent-securityenforcement"></a>

- The WNA runs **on the host** and therefore MUST enforce **all** configured restrictions strictly; there is no container to fall back to.
- Restricted commands (denylist or allowlist), including allowed subcommands and allowed/denied switches when configured, MUST be enforced before executing any user-level command (e.g. in `run_command` or equivalent local tool).
- When `primary_working_directory` is configured, the WNA MUST execute work (commands, file operations) with that directory as the default working directory; job-specific overrides MAY be permitted within the same policy (e.g. subdirectories under the primary).
- Allowed paths MUST be enforced for file read/write and for working directory; symlink resolution MUST be applied and escape outside allowed paths MUST be rejected.
- **Elevation:** When `elevation_allowed` is false (default), the WNA MUST run as a **non-root** user and MUST NOT use sudo or equivalent; per [REQ-SBAGNT-0102](../requirements/sbagnt.md#req-sbagnt-0102).
  When `elevation_allowed` is true, the WNA MAY use root/sudo with mandatory warnings and clear logging per [Elevation Allowed](#elevation-allowed).
- Outbound access MUST remain via the same controlled channels as the container SBA (worker proxies for inference, web egress, API egress); the WNA MUST NOT have direct internet or unrestricted host network access beyond what is explicitly configured for proxy endpoints.

## Traceability

- **REQ-SBAGNT-0001, REQ-SBAGNT-0102:** [sbagnt.md](../requirements/sbagnt.md).
  WNA is the same SBA runner with additional host-side restrictions (restricted commands, paths, elevation policy, etc.).
- **Configuration delivery:** [worker_node.md#spec-cynai-worker-configurationdelivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery).
  WNA config may be carried in the node configuration payload and applied by the node to the WNA.
- **Worker API:** [worker_api.md](../tech_specs/worker_api.md).
  Jobs are obtained via the same job pipeline (orchestrator -> Worker API -> node -> WNA).
- **Payload schema:** [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).
  When promoted, a `worker_node_agent` (or equivalent) section in `node_configuration_payload_v1` will be defined there or in a linked spec.
