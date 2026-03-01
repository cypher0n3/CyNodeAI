# Draft: Job Spec Persona and SBA LLM Prompt Construction

- [0. Document Meta](#0-document-meta)
- [1. Purpose](#1-purpose)
- [2. Persona Model](#2-persona-model)
- [3. Job Spec Update: Persona on the Job](#3-job-spec-update-persona-on-the-job)
- [4. Personas Storage and Schema](#4-personas-storage-and-schema)
- [5. Personas CRUD From Client Tools](#5-personas-crud-from-client-tools)
- [6. PMA/PAA Assigning Persona to a Job for SBA](#6-pmapaa-assigning-persona-to-a-job-for-sba)
- [7. SBA LLM Prompt Construction](#7-sba-llm-prompt-construction)
  - [7.1 Spec: SBA LLM Prompt Construction](#71-spec-sba-llm-prompt-construction)
  - [7.2 Context Included in Every SBA LLM Request](#72-context-included-in-every-sba-llm-request)
  - [7.3 Tools Presented to the LLM](#73-tools-presented-to-the-llm)
  - [7.4 Summary Table: Context Order and Source](#74-summary-table-context-order-and-source)
- [8. Requirements to Add or Update (Summary)](#8-requirements-to-add-or-update-summary)
- [9. Traceability](#9-traceability)
- [10. Next Steps](#10-next-steps)

## 0. Document Meta

- **Date:** 2026-02-28
- **Status:** Draft for review.
  No code changes; documentation only.
- **Target specs:** `cynode_sba.md`, `postgres_schema.md`, `user_api_gateway.md`, `web_console.md`, `cynork_cli.md`, `project_manager_agent.md`, `mcp_tool_catalog.md` (sandbox tools), and new requirements in `sbagnt.md` / `client.md` as needed.

## 1. Purpose

- Extend the job specification so that a job tracks a **persona** that gives the LLM context for how the sandbox agent is supposed to operate (role, background, supporting details).
- Allow users to **CRUD personas** from client tools (Web Console and cynork, with parity).
- Allow the **PMA and PAA** to assign a persona when creating or assigning a job to the SBA.
- Define how **SBA constructs LLM prompts**, including how tools are presented to the LLM and what context is included in each prompt.

## 2. Persona Model

- **Persona:** A named, reusable description of how the sandbox agent should behave for a class of jobs.
- **Persona title** (required): Short human-readable label (e.g. "Backend Developer", "Security Reviewer").
- **Persona description** (required): Prose in the form **"You are a [role] with [background] and [supporting details]."**
  This text is injected into the SBA's LLM system or initial user message so the model operates with that identity and constraints.
- **Scope:** Personas are stored per tenant/organization; visibility may be system-wide, project-scoped, group-scoped, or user-scoped (to be aligned with existing scope model in preferences/projects).
- **Storage and query:** Personas are stored in the database and MUST be **queriable by all agents** (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP (list, get) so that when building a job, the builder can resolve a persona by id and embed it inline in the job spec.
- **CRUD:** Create, Read (get/list), Update, Delete.
  Clients (Web Console and cynork) MUST offer the same operations per [REQ-CLIENT-0004](../requirements/client.md#req-client-0004).

## 3. Job Spec Update: Persona on the Job

- The job spec (e.g. `/job/job.json` consumed by `cynode-sba`) carries persona **inline only**.
  The SBA never receives a persona reference; it always receives the full persona object in the JSON.
- **Inline in JSON:** Top-level optional object `persona` with `title` (string) and `description` (string).
  The description MUST be prose in the form "You are a &lt;role&gt; with &lt;background&gt; and &lt;supporting details&gt;." (or equivalent structure).
- When **persona is present**, the SBA MUST use `persona.description` (and optionally `persona.title` for context) to build the full context for every LLM prompt (see [Section 7](#7-sba-llm-prompt-construction)).
- When **persona is absent**, the SBA uses only baseline context (and other job context) without a persona paragraph.
- **Source of inline persona:** Personas are stored in the database and are **queriable by all agents** (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP (e.g. persona list/get).
  When building a job, the builder (orchestrator or PM agent) looks up the chosen persona by id, then **embeds the full `title` and `description` inline** into the job spec.
  The job payload delivered to the node therefore contains only the inline `persona` object; no `persona_id` is required in the payload to SBA.
  Optionally, `persona_id` may be included in the job JSON for provenance or auditing; the SBA ignores it and uses only `persona.title` and `persona.description`.

Proposed addition to the example job shape in [Job Specification - Context Supplied to SBA](../tech_specs/cynode_sba.md#context-supplied-to-sba-requirements-acceptance-criteria-preferences-skills):

```json
"persona": {
  "title": "Backend Developer",
  "description": "You are a backend developer with experience in Go and APIs and a focus on clarity and testability. You prefer small, reviewable changes and explicit error handling."
}
```

Optional: `"persona_id": "uuid"` may be present for provenance; SBA uses only the inline `persona` object.

## 4. Personas Storage and Schema

- **New table: `personas`**
  - `id` (uuid, pk)
  - `title` (text, required)
  - `description` (text, required) - prose "You are a ..."
  - `scope_type` (text, optional) - e.g. `system`, `project`, `user`; determines visibility and which scope_id applies
  - `scope_id` (uuid, nullable) - e.g. project_id or user_id when scope_type is project or user
  - `created_at` (timestamptz)
  - `updated_at` (timestamptz)
  - `created_by` (uuid, fk to users.id, nullable)
- **Jobs:** The job payload (e.g. `jobs.payload` or the embedded `job_spec_json`) carries the full job spec with **inline** `persona: { title, description }`.
  The builder (orchestrator or PM agent) resolves the chosen persona from the DB and embeds it inline.
  Adding `jobs.persona_id` (fk to personas.id, nullable) is optional for indexing, reporting, and provenance.

## 5. Personas CRUD From Client Tools

- **Web Console:** List personas (with filters by scope), view one, create (title + description, scope), edit (title, description, scope), delete (with confirmation if referenced by jobs/tasks).
  Same capability set as cynork per REQ-CLIENT-0004.
- **cynork:** Commands such as `cynork persona list`, `cynork persona get <id>`, `cynork persona create --title "..." --description "..." [--scope-type project --scope-id <id>]`, `cynork persona update <id> ...`, `cynork persona delete <id>`.
  Output format and error handling aligned with other cynork resources.
- **User API Gateway:** REST endpoints for personas (list, get, create, update, delete) with auth and scope enforcement so that clients never talk to the DB directly.
  **All agents** (PMA, PAA, and any service that builds SBA jobs) MUST be able to query personas (list, get) via the same gateway or MCP so they can resolve a chosen persona and embed it inline into the job spec.

## 6. PMA/PAA Assigning Persona to a Job for SBA

- When the **Project Manager Agent** or **Project Analyst Agent** creates or assigns a job that will run the SBA (e.g. via MCP `sandbox.create` or orchestrator job builder), it MUST be able to set the job's persona.
- **Query then embed:** All agents (PMA, PAA, orchestrator job builder) query the database for personas via the User API Gateway or MCP (e.g. persona list, persona get by id).
  When assigning a persona to a job, the builder fetches the full persona (title, description) and **embeds it inline** in the job spec JSON.
  The job payload sent to the node contains only the inline `persona` object.
- **Mechanisms:**
  - **Orchestrator job builder:** Task create (e.g. with `use_sba: true`) or job-create API accepts an optional `persona_id`; the builder looks up the persona, then embeds `persona: { title, description }` into the job spec JSON (e.g. inside `job_spec_json`).
  - **PMA/PAA via MCP:** When the PM agent invokes sandbox tools that result in an SBA job, it MAY pass `persona_id`; the orchestrator (or PM) resolves it via persona get and embeds the full persona inline in the job spec.
    The MCP contract MUST allow passing a persona (by id or inline) so the final job spec contains the inline persona.
- The SBA runner receives the job spec (e.g. from `/job/job.json`) with persona **always inline** (`persona.title`, `persona.description`).
  The SBA does not resolve persona_id; it uses only the inline object.

## 7. SBA LLM Prompt Construction

The following section should be added to [cynode_sba.md](../tech_specs/cynode_sba.md) (or a dedicated subsection under Job Specification / Context) to define how the SBA builds each LLM request.

### 7.1 Spec: SBA LLM Prompt Construction

- **Spec ID:** `CYNAI.SBAGNT.LlmPromptConstruction` (proposed)

**Purpose:** Unambiguously define the content and order of context sent to the LLM on each request (system message and/or user message), and how tools are presented.

### 7.2 Context Included in Every SBA LLM Request

The SBA MUST build each LLM prompt (system message or equivalent, plus any user/turn content) by including the following **in this order**:

#### 7.2.1 Persona (If Present in Job)

The persona description from the job's inline `persona.description` (always supplied inline; see [Section 3](#3-job-spec-update-persona-on-the-job)) in the form "You are a &lt;role&gt; with &lt;background&gt; and &lt;supporting details&gt;."
MUST be the first context block so the model adopts the role before other instructions.

#### 7.2.2 Baseline Context

Identity, role, responsibilities, and non-goals for the sandbox agent (from `context.baseline_context` or image-baked baseline).
MUST be included in every LLM prompt per existing [Context Supplied to SBA](../tech_specs/cynode_sba.md#context-supplied-to-sba-requirements-acceptance-criteria-preferences-skills).

#### 7.2.3 Project-Level Context

When the job is project-scoped: project identity (id, name, slug), scope, relevant metadata.
MUST be included when present in the job.

#### 7.2.4 Task-Level Context

Task identity (id, name), acceptance criteria summary, status, relevant task metadata.
MUST be included.

#### 7.2.5 Requirements and Acceptance Criteria

Task or project requirements and acceptance criteria from the job context.

#### 7.2.6 Relevant Preferences

Resolved user/task preferences that affect how work is done.

#### 7.2.7 User-Configurable Additional Context

From `agents.sandbox_agent.additional_context` (resolved at job creation).
Merged after the above per existing spec.

#### 7.2.8 Skills (If Supplied in Job or Fetched)

Inline skill content or references; the SBA MAY fetch skills via MCP and append summaries or full text here.

#### 7.2.9 Runtime Context for This Turn

  Remaining time or deadline; current todo list or step progress; and the current user/tool turn (e.g. latest user message or tool result that requires a response).

The implementation MUST concatenate or structure these blocks in a deterministic way (e.g. clear section headers or role/system vs user message split) so that the model receives a consistent, ordered context.

### 7.3 Tools Presented to the LLM

- **Local tools** (run_command, write_file, read_file, apply_unified_diff, list_tree, search_files): The SBA MUST present each tool to the LLM with:
  - **Name:** The canonical tool name (e.g. `run_command`, `read_file`).
  - **Description:** A short, non-ambiguous description of what the tool does (from the spec or implementation).
  - **Arguments schema:** A structured schema (e.g. JSON Schema) for the tool's arguments (required and optional), including types and constraints (e.g. `path` under workspace, `argv` array).
- **MCP tools** (sandbox allowlist): When the SBA wraps MCP tools as langchaingo (or equivalent) tools, each tool MUST be presented to the LLM with:
  - **Name:** The MCP tool name (e.g. `artifact.put`, `skills.get`).
  - **Description:** The tool description from the MCP tool catalog or gateway.
  - **Arguments schema:** The schema required by the MCP tool (e.g. task_id, job_id, and tool-specific parameters); the SBA MUST inject task_id and job_id from the job spec so the LLM is not required to supply them, or the schema MUST document them as required.
- **Order and grouping:** The implementation MAY present local tools first, then MCP tools, or interleave by category; the order MUST be deterministic for a given job so that behavior is reproducible.
- **No credentials or internal URLs:** Tool descriptions and schemas MUST NOT include secrets, bearer tokens, or internal callback URLs; those are injected by the runtime (node/orchestrator) into the SBA environment or request context, not into the tool schema visible to the LLM.

### 7.4 Summary Table: Context Order and Source

- **Order:** 1
  - context block: Persona
  - source (job or runtime): job `persona.description` (inline; personas queriable from DB by all agents at job-build time)
  - required when: When persona present in job
- **Order:** 2
  - context block: Baseline context
  - source (job or runtime): job `context.baseline_context` or image
  - required when: Always
- **Order:** 3
  - context block: Project-level context
  - source (job or runtime): job `context.project_context`
  - required when: When job is project-scoped
- **Order:** 4
  - context block: Task-level context
  - source (job or runtime): job `context.task_context`
  - required when: Always
- **Order:** 5
  - context block: Requirements / AC
  - source (job or runtime): job `context.requirements`, `acceptance_criteria`
  - required when: When supplied
- **Order:** 6
  - context block: Preferences
  - source (job or runtime): job `context.preferences`
  - required when: When supplied
- **Order:** 7
  - context block: Additional context
  - source (job or runtime): job `context.additional_context`
  - required when: When preference resolved
- **Order:** 8
  - context block: Skills
  - source (job or runtime): job `context.skills` / `skill_ids` or MCP
  - required when: When supplied or fetched
- **Order:** 9
  - context block: Runtime (time, todo, turn)
  - source (job or runtime): job/runtime deadline, todo state, messages
  - required when: Every request

## 8. Requirements to Add or Update (Summary)

- **SBAGNT:** The SBA MUST use the job's persona (when present) as the first context block in every LLM prompt.
  The SBA MUST construct LLM prompts according to the defined context order and tool presentation (spec above).
- **CLIENT:** Clients (Web Console and cynork) MUST provide CRUD for personas with capability parity (REQ-CLIENT-0004 already implies this for any new admin resource).
- **Schema:** Add `personas` table; optionally add `jobs.persona_id` for analytics/indexing.

## 9. Traceability

- [REQ-SBAGNT-0107](../requirements/sbagnt.md#req-sbagnt-0107) (context supplied to SBA)
- [REQ-SBAGNT-0111](../requirements/sbagnt.md#req-sbagnt-0111) (baseline and additional context in every LLM prompt)
- [REQ-CLIENT-0004](../requirements/client.md#req-client-0004) (client parity)
- [CYNAI.SBAGNT.JobContext](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext)
- [CYNAI.MCPTOO.SandboxTools](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-sandboxtools)

## 10. Next Steps

- Review this draft and decide whether to adopt persona as reference-only, inline-only, or both (recommended: both).
- Add the `personas` table and optional `jobs.persona_id` to [postgres_schema.md](../tech_specs/postgres_schema.md).
- Insert the job spec persona fields and the full "SBA LLM Prompt Construction" subsection into [cynode_sba.md](../tech_specs/cynode_sba.md).
- Add Personas CRUD to [web_console.md](../tech_specs/web_console.md) and [cynork_cli.md](../tech_specs/cynork_cli.md) (and User API Gateway).
- Update PMA/PAA and sandbox tool specs so that job creation for SBA accepts `persona_id` (resolved from DB and embedded inline) or inline `persona`; ensure all agents can query personas via gateway/MCP.
- Add or update requirements in [sbagnt.md](../requirements/sbagnt.md) and [client.md](../requirements/client.md) as needed.
