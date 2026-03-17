# Functional Testing Gaps Assessment

- [Executive Summary](#executive-summary)
- [Test Inventory](#test-inventory)
- [Gap Analysis](#gap-analysis)
- [E2E Module to REQ Mapping](#e2e-module-to-req-mapping)
- [Uncovered Domains: Specific REQ Ids](#uncovered-domains-specific-req-ids)
- [Undefined BDD Steps (Exact Step Text)](#undefined-bdd-steps-exact-step-text)
- [Orchestrator Scenarios Requiring Database](#orchestrator-scenarios-requiring-database)
- [Behavior Coverage: E2E vs BDD](#behavior-coverage-e2e-vs-bdd)
- [Traceability and Consistency](#traceability-and-consistency)
- [Recommendations](#recommendations)
- [Appendix](#appendix)

<!-- Date: 2026-03-12; Type: Findings; Audience: Senior Go developers, maintainers -->

## Executive Summary

This report assesses functional testing gaps for CyNodeAI by comparing [docs/requirements](../requirements/) coverage to automated tests: Python E2E (`scripts/test_scripts/e2e_*.py`), Godog BDD (`*/_bdd`), and feature files in [features/](../../features/).
Validation used `just e2e --list`, `just check-e2e-requirements-traces`, and `just test-bdd`; all E2E modules have valid `# Traces:` REQ references and BDD suites complete (with some undefined steps in agents and cynork).

### Top-Level Gaps

- **15 of 24 requirement domains** have no E2E trace coverage (ACCESS, AGENTS, APIEGR, BOOTST, BROWSR, CONNEC, DATAPI, MCPGAT, MCPTOO, PROJCT, SCHEMA, STANDS, STEPEX, WEBCON, WEBPRX).
- **Four declared BDD suites** have no feature files (admin_web_console, api_egress_server, secure_browser_service, mcp_gateway).
- **Client parity (REQ-CLIENT-0004):** Web Console is not exercised by any automated E2E or BDD; only CLI and curl are covered.
- **BDD:** Agents suite has 2 scenarios with undefined steps (SBA inference feature); cynork has 1 scenario with undefined steps; orchestrator DB-dependent scenarios require `POSTGRES_TEST_DSN` and may be skipped without it.

## Test Inventory

Inventory of E2E modules, BDD suites, and feature files used in this assessment.

### E2E Modules and REQ Traces

All 36 E2E modules under `scripts/test_scripts/e2e_*.py` (excluding `e2e_state.py`, `e2e_tags.py`) declare at least one `REQ-<DOMAIN>-<NNNN>` in their `# Traces:` block; `just check-e2e-requirements-traces` passes.
A full module-to-REQ mapping appears in [E2E Module to REQ Mapping](#e2e-module-to-req-mapping) below.

### Requirement Domains Covered by E2E Traces

- CLIENT (0146, 0150, 0161, 0173)
- IDENTY (0103, 0104, 0106, 0122)
- MODELS (0008)
- ORCHES (0113, 0114, 0120, 0122, 0123, 0124, 0125, 0131, 0132, 0144, 0149, 0162)
- PMAGNT (0100; 0101 referenced in comments)
- SANDBX (0131)
- SBAGNT (0001, 0103, 0106, 0109)
- SKILLS (0106, 0115)
- USRGWY (0121, 0130, 0131)
- WORKER (0114, 0140, 0142, 0160, 0174, 0200, 0270)

E2E test count: 72 tests discovered via `just e2e --list` (unittest discovery from all `e2e_*.py` modules).

### BDD Suites and Feature Files

Suites that have both Gherkin feature files and Godog step implementations.

#### Suites With Feature Files and Godog Runners

- `suite_orchestrator`: [features/orchestrator/](../../features/orchestrator/) (7 files); [orchestrator/_bdd](../../orchestrator/_bdd)
- `suite_worker_node`: [features/worker_node/](../../features/worker_node/) (10 files); [worker_node/_bdd](../../worker_node/_bdd)
- `suite_agents`: [features/agents/](../../features/agents/) (8 files); [agents/_bdd](../../agents/_bdd)
- `suite_cynork`: [features/cynork/](../../features/cynork/) (6 files); [cynork/_bdd](../../cynork/_bdd)
- `suite_e2e`: [features/e2e/](../../features/e2e/) (2 files); [e2e/_bdd](../../e2e/_bdd)

#### BDD Run (2026-03-12)

`just test-bdd` completed successfully.
Orchestrator, worker_node, cynork, agents, and e2e modules ran; worker_node reported 44 scenarios (44 passed), 185 steps (185 passed).
Agents reported 16 scenarios (14 passed, 2 undefined), 83 steps (73 passed, 8 undefined, 2 skipped).
Cynork reported 36 scenarios (35 passed, 1 undefined), 202 steps (199 passed, 2 undefined, 1 skipped).
Orchestrator scenarios that need a database are skipped unless `POSTGRES_TEST_DSN` is set (per [justfile](../../justfile) and [ports_and_endpoints](../tech_specs/ports_and_endpoints.md)).

## Gap Analysis

Gaps between requirements, feature files, and automated tests.

### Requirements With No Functional Coverage (By Domain)

The following requirement domains appear in [docs/requirements](../requirements/) but have **no** REQ id referenced in any E2E `# Traces:` block.
BDD feature scenarios may still tag some of these via `@req_*`; this section focuses on E2E trace coverage as the project-mandated trace for Python E2E.

#### Domains With Zero E2E Trace Coverage

- **ACCESS** (access control, policy)
- **AGENTS** (high-level agents domain; PMA/SBA covered via PMAGNT/SBAGNT in E2E)
- **APIEGR** (API Egress Server; feature [api_egress_call.feature](../../features/orchestrator/api_egress_call.feature) exists and tags REQ-APIEGR-0110, 0119 but no E2E module traces APIEGR)
- **BOOTST** (bootstrap; BDD only, e.g. orchestrator_startup)
- **BROWSR** (Secure Browser Service)
- **CONNEC** (connector catalog, credentials, policy)
- **DATAPI** (data pipeline)
- **MCPGAT** (MCP gateway)
- **MCPTOO** (MCP tools)
- **PROJCT** (projects)
- **SCHEMA** (database schema)
- **STANDS** (standards; E2E references CYNAI.STANDS in comment but no REQ-STANDS in Traces)
- **STEPEX** (step execution / sandbox runner binary)
- **WEBCON** (Web Console)
- **WEBPRX** (Web Egress Proxy)

For specific REQ ids per uncovered domain, see [Uncovered Domains: Specific REQ Ids](#uncovered-domains-specific-req-ids).
For exact undefined step text, see [Undefined BDD Steps (Exact Step Text)](#undefined-bdd-steps-exact-step-text).
For which orchestrator scenarios need a DB, see [Orchestrator Scenarios Requiring Database](#orchestrator-scenarios-requiring-database).

### Feature Scenarios Without Full Automation

- **agents/sba_inference.feature:** Two scenarios use steps that have no Godog implementation in agents/_bdd.
  Exact step text and which steps are implemented vs undefined are listed in [Undefined BDD Steps (Exact Step Text)](#undefined-bdd-steps-exact-step-text).
  The suite still passes because undefined steps do not fail the run by default.
- **cynork:** One scenario and two steps are undefined; run `just test-bdd` and inspect the "You can implement step definitions for undefined steps with these snippets" section for the exact step text and feature file.
- **orchestrator:** All orchestrator feature files use "Given a running PostgreSQL database"; scenarios are skipped when `POSTGRES_TEST_DSN` is unset and testcontainers are not used.
  See [Orchestrator Scenarios Requiring Database](#orchestrator-scenarios-requiring-database) for the list of affected feature files.

### Suites With No Feature Files

[features/README.md](../../features/README.md) declares these suite tags in the registry:

- `@suite_admin_web_console`
- `@suite_api_egress_server`
- `@suite_secure_browser_service`
- `@suite_mcp_gateway`

There are **no** `.feature` files under any directory for these four suites.
So the suite registry is ahead of implementation: four major components have no Gherkin acceptance specs in the repo.

### Client Parity (REQ-CLIENT-0004)

REQ-CLIENT-0004 requires the Web Console and the CLI management app to provide capability parity for administrative operations ([client.md](../requirements/client.md), [meta.md](../../meta.md)).

All Python E2E tests and the documented BDD flows use the **cynork CLI** and **curl** against the user-gateway and control-plane.
There is no automated E2E or BDD coverage for the **Web Console** UI or for verifying that a given capability exists in both clients.
Functional testing of client parity is therefore **CLI-only**; Web Console behavior is not covered by automated functional tests.

## E2E Module to REQ Mapping

Each E2E module and the REQ ids traced in its `# Traces:` block (or Validates line).
Use this to see which tests cover which requirements and to add new traces when extending E2E.

- **e2e_0010_cli_version_and_status:** REQ-ORCHES-0120 (healthz); CYNAI.STANDS.CliCynork
- **e2e_0020_gateway_health_readyz:** REQ-ORCHES-0120
- **e2e_0030_auth_login:** REQ-IDENTY-0103, 0104
- **e2e_0040_auth_negative_whoami:** REQ-IDENTY-0122, 0103, 0104
- **e2e_0050_auth_whoami:** REQ-IDENTY-0103, 0104
- **e2e_0300_worker_api_health_readyz:** REQ-WORKER-0140, 0142
- **e2e_0310_worker_telemetry:** REQ-WORKER-0200, 0230, 0231, 0232, 0234
- **e2e_0320_worker_api_managed_service:** REQ-WORKER-0160, 0161
- **e2e_0330_node_manager_telemetry:** REQ-WORKER-0200, 0230
- **e2e_0340_uds_inference_routing:** REQ-WORKER-0270, REQ-SANDBX-0131, REQ-WORKER-0174
- **e2e_0380_control_plane_node_register:** REQ-ORCHES-0113, 0114
- **e2e_0390_control_plane_capability:** REQ-ORCHES-0114
- **e2e_0420_task_create:** REQ-ORCHES-0122, 0126
- **e2e_0430_task_list:** REQ-ORCHES-0125
- **e2e_0440_task_get:** REQ-ORCHES-0125
- **e2e_0450_task_result:** REQ-ORCHES-0124, 0125
- **e2e_0460_task_logs:** REQ-ORCHES-0124
- **e2e_0470_task_cancel:** REQ-ORCHES-0125
- **e2e_0480_task_list_status_filter:** REQ-ORCHES-0125
- **e2e_0490_skills_gateway:** REQ-CLIENT-0146; REQ-SKILLS-0106, 0115
- **e2e_0500_workflow_api:** REQ-ORCHES-0144, 0145, 0146
- **e2e_0510_task_inference:** REQ-WORKER-0114, 0270; REQ-ORCHES-0123
- **e2e_0520_task_prompt:** REQ-ORCHES-0122, 0126
- **e2e_0530_task_models_and_chat:** REQ-USRGWY-0121, 0127; REQ-CLIENT-0161
- **e2e_0540_chat_reliability:** REQ-ORCHES-0131, 0132
- **e2e_0550_chat_sequential_messages:** REQ-USRGWY-0130
- **e2e_0560_chat_simultaneous_messages:** REQ-ORCHES-0131, 0132
- **e2e_0570_pma_chat_context:** REQ-USRGWY-0131; REQ-CLIENT-0173
- **e2e_0580_pma_chat_capable_model:** REQ-MODELS-0008; REQ-PMAGNT-0100, 0101
- **e2e_0610_sse_streaming:** (streaming; see tech_specs openai_compatible_chat_api, chat_threads_and_messages)
- **e2e_0660_worker_pma_proxy:** REQ-ORCHES-0162 (PMA routing via worker)
- **e2e_0710_sba_task:** REQ-SBAGNT-0001, 0106
- **e2e_0720_sba_task_result_contract:** REQ-SBAGNT-0103
- **e2e_0730_sba_task_inference:** REQ-SBAGNT-0106, 0109
- **e2e_0740_sba_inference_reply:** REQ-SBAGNT-0103, 0109
- **e2e_0750_tui_pty:** (TUI PTY harness)
- **e2e_0760_tui_slash_commands:** (TUI slash commands)
- **e2e_0770_auth_refresh:** REQ-IDENTY-0104, 0105
- **e2e_0780_auth_logout:** REQ-IDENTY-0106; REQ-CLIENT-0150
- **e2e_0790_prescribed_startup_config_inference_backend:** REQ-ORCHES-0149, 0113, 0114

## Uncovered Domains: Specific REQ Ids

The following lists cite concrete REQ ids from [docs/requirements](../requirements/) that have **no** E2E `# Traces:` reference.
BDD feature scenarios may tag some via `@req_*`; E2E trace coverage is the gap.

### ACCESS (Access Control, Policy, RBAC)

- REQ-ACCESS-0001, 0002; 0100--0125 (groups, membership, role bindings, RBAC, vector retrieval, audit).
- Functionally testable at gateway: group/membership CRUD, role bindings, policy deny/allow, audit log presence.

### APIEGR (API Egress Server)

- REQ-APIEGR-0001, 0002; 0100--0127 (access control, subject identity, allow policy, credential handling, logging, sanity check, Git egress).
- Feature [api_egress_call.feature](../../features/orchestrator/api_egress_call.feature) tags REQ-APIEGR-0110, 0119 and has three scenarios (allowed provider 501, disallowed 403, missing bearer 401); no E2E module traces any REQ-APIEGR.

### BOOTST (Bootstrap)

- REQ-BOOTST-0001, 0002; 0100--0105 (bootstrap YAML, inference-capable path before ready, idempotent import, PM model default, auto-start).
- BDD: [orchestrator_startup.feature](../../features/orchestrator/orchestrator_startup.feature) tags REQ-BOOTST-0002; no E2E trace.

### BROWSR (Secure Browser Service)

- REQ-BROWSR-0001; 0100--0122 (untrusted content, preferences, robots.txt, redirects, access control, audit).
- No feature files and no E2E for this suite.

### CONNEC (Connector Catalog and Instances)

- REQ-CONNEC-0001; 0100--0121 (catalog, install/enable/disable, credentials, policy, audit, User API Gateway endpoints).
- Gateway surface: connector CRUD and credential management; no E2E traces any REQ-CONNEC.

### DATAPI (Data REST API)

- REQ-DATAPI-0001; 0100--0112 (User API Gateway, no raw SQL, authn/authz, audit, GORM, vector similarity).
- No E2E module exercises Data REST API or traces REQ-DATAPI.

### MCPGAT (MCP Gateway)

- REQ-MCPGAT-0001, 0002; 0100--0116 (MCP protocol, task/run/job-scoped args, audit, tool on/off, sandbox vs PM tools).
- Functionally testable via tool call flows and audit; no E2E trace.

### MCPTOO (MCP Tools)

- Domain exists in [docs/requirements](../requirements/) (mcptoo.md); no E2E or BDD scenario traced to REQ-MCPTOO in this assessment.

### PROJCT (Projects)

- REQ-PROJCT-0001; 0100--0111 (projects in PostgreSQL, default project, MCP tools for search, Git repo associations, plans).
- E2E touches project context (e.g. REQ-CLIENT-0173, REQ-USRGWY-0131) but no E2E module traces REQ-PROJCT directly.

### SCHEMA (Database Schema)

- REQ-SCHEMA-0001; 0100--0113 (GORM, AutoMigrate, pgvector, vector scope, RBAC columns).
- Schema is tested via integration and migration tests; no functional E2E trace.

### STEPEX (Step Executor, No-LLM Runner)

- REQ-STEPEX-0001; 0100--0105 (versioned job spec, validation, non-root, result contract, Worker API integration).
- Distinct from SBAGNT (agent runner); no E2E or BDD scenario traces REQ-STEPEX.

### WEBCON (Web Console)

- REQ-WEBCON-0001; 0100--0115 (no direct DB, gateway only, secrets write-only, least privilege, skills CRUD, Swagger UI, port 8080).
- Entire domain uncovered by E2E/BDD; see [Client Parity (REQ-CLIENT-0004)](#client-parity-req-client-0004).

### WEBPRX (Web Egress Proxy)

- REQ-WEBPRX-0100--0106 (sandbox web egress, default-deny, audit, task-scoped allowlist, redirect re-evaluation).
- No E2E or feature coverage.

### STANDS (Standards)

- REQ-STANDS-0001; 0100--0134 (Go REST APIs, timeouts, JSON, auth, healthz/readyz, GORM, secrets handling).
- E2E comments reference CYNAI.STANDS; no REQ-STANDS id in any E2E `# Traces:` block.

### AGENTS (High-Level Agents Domain)

- REQ-AGENTS-0001--0004; 0100--0136 (cloud workers, PM/PA, LangGraph checkpoint, MCP-only DB, preferences, no provider keys).
- PMA/SBA behavior is covered via PMAGNT/SBAGNT and USRGWY in E2E; no E2E module traces REQ-AGENTS-*.

## Undefined BDD Steps (Exact Step Text)

Scenarios that declare steps for which no Godog step definition exists in the corresponding `_bdd` package.
When run with `just test-bdd`, these appear as "undefined" and the runner prints snippet suggestions.

### Agents Suite

Feature file: [sba_inference.feature](../../features/agents/sba_inference.feature).
Two scenarios; the following steps are undefined in agents/_bdd.

#### Scenario 1: SBA Task With Inference Completes With `Sba_result_result` and User-Facing Reply

- `Given inference path is available and SBA runner is configured`
- `When I create a SBA task with inference and prompt "Reply back with the current time" and the task runs to terminal status`
- `Then the task status is "completed"` (implemented)
- `And the job result contains "sba_result"` (implemented)
- `And the job result has a user-facing reply (non-empty stdout or sba_result steps/reply)` (undefined)

#### Scenario 2: SBA Task With Inference That Fails is Treated as Product Failure

- `Given inference is required for the SBA task (not gated by skip-inference flag)` (undefined)
- `When I create a SBA task with inference and the task reaches status "failed"` (undefined)
- `Then the outcome is treated as product failure` (undefined)
- `And the failure is not treated as environmental skip` (undefined)
- `And the test or harness fails (does not skip)` (undefined)

Implementing these in [agents/_bdd/steps.go](../../agents/_bdd/steps.go) (or equivalent) would allow both scenarios to run as defined instead of being reported undefined.

### Cynork Suite

One scenario and two steps are reported undefined when running `just test-bdd`.
The exact step text and file are not enumerated here; run `just test-bdd` and inspect the output section "You can implement step definitions for undefined steps with these snippets" to obtain the suggested step definitions and the feature file path (e.g. `../../features/cynork/...`).

## Orchestrator Scenarios Requiring Database

All orchestrator feature files that use the step "Given a running PostgreSQL database" require either a real Postgres instance (`POSTGRES_TEST_DSN`) or testcontainers (podman) when `POSTGRES_TEST_DSN` is unset.
When `SKIP_TESTCONTAINERS=1` is set or podman is unavailable, DB-dependent scenarios are skipped.

### Feature Files and DB Usage

- [initial_auth.feature](../../features/orchestrator/initial_auth.feature): Background "Given a running PostgreSQL database"
- [orchestrator_startup.feature](../../features/orchestrator/orchestrator_startup.feature): Scenario with "Given a running PostgreSQL database"
- [node_registration_and_config.feature](../../features/orchestrator/node_registration_and_config.feature): Background and steps "the node is recorded in the database"
- [orchestrator_task_lifecycle.feature](../../features/orchestrator/orchestrator_task_lifecycle.feature): Background "Given a running PostgreSQL database"
- [workflow_start_resume_lease.feature](../../features/orchestrator/workflow_start_resume_lease.feature): Background "Given a running PostgreSQL database"
- [openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature): Two scenarios with "Given a running PostgreSQL database"

So all seven orchestrator feature files either use the DB in Background or in scenario steps; none of the orchestrator BDD scenarios run without a database unless steps are skipped.

## Behavior Coverage: E2E vs BDD

Which behaviors are covered only by E2E, only by BDD, or by both.
This helps prioritize where to add E2E traces or BDD steps.

**Auth (login, whoami, refresh, logout, token validation):** E2E (e2e_0030, 0040, 0050, 0770, 0780); BDD (initial_auth.feature).
Both layers cover auth; E2E traces REQ-IDENTY-0103, 0104, 0105, 0106, 0122.

**Task create/list/get/result/logs/cancel/status filter:** E2E (e2e_0420, 0430, 0440, 0450, 0460, 0470, 0480); BDD (orchestrator_task_lifecycle.feature, cynork_tasks.feature).
Both cover task lifecycle; E2E traces REQ-ORCHES-0122, 0124, 0125, 0126.

**Workflow start/resume/lease:** E2E (e2e_0500); BDD (workflow_start_resume_lease.feature).
Both; E2E traces REQ-ORCHES-0144, 0145, 0146.

**Node registration and config:** E2E (e2e_0380, 0790, 0390); BDD (node_registration_and_config.feature).
Both; E2E traces REQ-ORCHES-0113, 0114, 0149.

**Chat completion (one-shot, multi-turn, reliability):** E2E (e2e_0530, 0570, 0580, 0540, 0550, 0560); BDD (chat_completion_reliability.feature, chat_openai_compatible.feature).
Both; E2E traces REQ-ORCHES-0131, 0132, REQ-USRGWY-0121, 0130, 0131, REQ-CLIENT-0161, 0173.

**API Egress call (POST /v1/call, allow/deny, bearer):** BDD only ([api_egress_call.feature](../../features/orchestrator/api_egress_call.feature)); no E2E module.
E2E gap: add an e2e_* module that calls the API Egress endpoint and traces REQ-APIEGR-0110, 0119.

**Skills CRUD via gateway:** E2E (e2e_0490); BDD (cynork_skills.feature).
Both; E2E traces REQ-SKILLS-0106, 0115, REQ-CLIENT-0146.

**Worker telemetry, node-manager telemetry, worker API health/readyz:** E2E (e2e_0310, 0300, 0320, 0330); BDD (worker_telemetry.feature, worker_api_managed_service.feature, etc.).
Both.

**SBA task and result contract:** E2E (e2e_0710, 0720, 0730, 0740); BDD (sba_contract.feature, sba_inference.feature, worker_node_sba.feature).
Both; SBA inference scenarios in BDD have undefined steps (see [Undefined BDD Steps](#undefined-bdd-steps-exact-step-text)).

**PMA chat and proxy:** E2E (e2e_0570, 0580, 0660); BDD (pma_chat_and_context.feature, worker_pma_proxy coverage in worker_node).
Both.

**UDS inference routing (REQ-WORKER-0270, REQ-SANDBX-0131):** E2E (e2e_0340); BDD (worker_inference_proxy.feature, worker_node_sandbox_execution.feature).
Both.

**Connector catalog/instances (REQ-CONNEC-*):** Neither E2E nor BDD.
Full gap.

**Web Console (REQ-WEBCON-*, REQ-CLIENT-0004 parity):** Neither E2E nor BDD.
Full gap.

**MCP gateway tool call and audit (REQ-MCPGAT-*):** No E2E; BDD coverage is indirect via agent flows.
E2E gap for explicit MCP call and audit checks.

## Traceability and Consistency

How E2E Traces and BDD tags align with requirements.

- **E2E:** Every E2E module has a `# Traces:` block containing at least one REQ id; format is validated by [.ci_scripts/check_e2e_requirements_traces.py](../../.ci_scripts/check_e2e_requirements_traces.py) (invoked via `just check-e2e-requirements-traces`).
- **BDD:** Feature scenarios use `@req_<domain>_<nnnn>` tags that map to REQ ids; these are not automatically validated against a central REQ list.
  Consistency between E2E Traces and BDD tags is manual: e.g. REQ-IDENTY-0104 is traced in E2E (e2e_0030, e2e_0050, e2e_0770) and tagged in features (initial_auth, single_node_happy_path).
- **Missing traces:** No E2E module traces REQ-APIEGR, REQ-CONNEC, REQ-WEBCON, REQ-MCPGAT, or the other uncovered domains listed above; adding E2E tests for those areas would require new `# Traces:` lines and possibly new e2e_* modules.

## Recommendations

1. **Add E2E for API Egress (REQ-APIEGR-0110, 0119):** [api_egress_call.feature](../../features/orchestrator/api_egress_call.feature) already defines three scenarios (allowed provider 501, disallowed 403, missing bearer 401).
   Add a new E2E module (e.g. `e2e_2XX_api_egress_call.py`) that exercises POST to the API Egress `/v1/call` endpoint with bearer and body, asserts status and JSON shape, and declares `# Traces: REQ-APIEGR-0110, 0119`.
2. **Implement undefined steps in [agents/_bdd](../../agents/_bdd) for [sba_inference.feature](../../features/agents/sba_inference.feature):** Implement the five undefined step patterns listed in [Undefined BDD Steps (Exact Step Text)](#undefined-bdd-steps-exact-step-text) so both SBA inference scenarios run without "undefined" (e.g. wire to a test harness that creates an SBA task and waits for terminal status, or skip when inference is unavailable with a clear reason).
3. **Resolve cynork BDD undefined scenario:** Run `just test-bdd`, copy the suggested step snippets for the cynork undefined scenario, implement them in [cynork/_bdd](../../cynork/_bdd), and tag the scenario with the appropriate `@req_*` if not already present.
4. **Add feature files or adjust suite registry:** For each of `@suite_admin_web_console`, `@suite_api_egress_server`, `@suite_secure_browser_service`, `@suite_mcp_gateway`, either add at least one `.feature` file under a corresponding directory (and a Godog runner if desired) or remove the suite tag from [features/README.md](../../features/README.md) until features exist.
5. **Client parity (REQ-CLIENT-0004):** Define how parity is verified: (a) document manual verification, or (b) add Web Console E2E (e.g. Playwright) that exercises the same administrative capabilities as the CLI and tag tests with REQ-CLIENT-0004.
   Ensure any new CLI capability in cynork has a matching E2E and, if (b), a corresponding Web Console test or explicit waiver.
6. **Orchestrator BDD and DB:** In CI or [orchestrator/README.md](../../orchestrator/README.md), document that full orchestrator BDD requires `POSTGRES_TEST_DSN` or testcontainers (podman); run DB-backed BDD in CI so the seven orchestrator feature files are exercised consistently.
7. **Optional: E2E traces for CONNEC, MCPGAT, PROJCT:** If connector or MCP tool endpoints become available via the User API Gateway, add E2E modules that trace REQ-CONNEC-0100/0114/0115, REQ-MCPGAT-0107/0108, or REQ-PROJCT-0105 so coverage aligns with [docs/requirements](../requirements/).

## Appendix

Short reference for targets and documentation.

### Justfile Targets Used

- `just e2e --list` - list E2E test names
- `just check-e2e-requirements-traces` - validate E2E `# Traces:` REQ references
- `just test-bdd` - run Godog BDD for orchestrator, worker_node, cynork, agents, e2e

### Appendix References

- [scripts/test_scripts/README.md](../../scripts/test_scripts/README.md) - E2E layout, numbering, tags, state
- [features/README.md](../../features/README.md) - suite tags, traceability tags, testing overview
- [ai_files/ai_coding_instructions.md](../../ai_files/ai_coding_instructions.md) - BDD/TDD and E2E in Red phase
- [docs/requirements/README.md](../requirements/README.md) - requirement domains and conventions
- [docs/tech_specs/ports_and_endpoints.md](../tech_specs/ports_and_endpoints.md) - E2E and BDD port usage
