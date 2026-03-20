# Agent Personas

- [Document Overview](#document-overview)
- [Postgres Schema](#postgres-schema)
  - [Personas Table](#personas-table)

## Document Overview

Agent personas are named, reusable descriptions of how the sandbox agent should behave (role, identity, tone); they are not customer or end-user personas.
They are stored in the deployment and are queriable by agents (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP.
When building a job, the builder resolves the chosen Agent persona by id (or by title with scope precedence) and embeds `title` and `description` inline into the job spec; the SBA receives only the inline object.
Editing (create, update, delete) is subject to RBAC: system-scoped personas require admin (or equivalent) role; user-/project-/group-scoped require appropriate role for that scope; see [data_rest_api.md - Core Resources](data_rest_api.md#spec-cynai-datapi-coreresources).

Related documents

- [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona)
- [Persona MCP Tools](mcp_tools/persona_tools.md)
- [Project Manager Agent - Persona assignment and resolution](project_manager_agent.md#spec-cynai-agents-personaassignment)

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.Personas` <a id="spec-cynai-schema-personas"></a>

### Personas Table

- Spec ID: `CYNAI.SCHEMA.PersonasTable` <a id="spec-cynai-schema-personastable"></a>

- `id` (uuid, pk)
- `title` (text, required)
  - short human-readable label (e.g. "Backend Developer", "Security Reviewer")
- `description` (text, required)
  - short prose in the form "You are a &lt;role&gt; with &lt;background&gt; and &lt;supporting details&gt;."
- `scope_type` (text, optional)
  - e.g. `system`, `project`, `user`; determines visibility and which scope_id applies
- `scope_id` (uuid, nullable)
  - e.g. project_id or user_id when scope_type is project or user
- `default_skill_ids` (jsonb, nullable)
  - optional array of skill stable identifiers; when present, the job builder resolves and includes them in context supplied to the SBA (merged with task recommended_skill_ids; union, task overrides duplicates)
- `recommended_cloud_models` (jsonb, nullable)
  - optional map keyed by provider (e.g. openai, anthropic), value = array of model stable identifiers; orchestrator uses this to select a cloud model when the job uses this persona
- `recommended_local_model_ids` (jsonb, nullable)
  - optional array of model stable identifiers for worker-node inference; orchestrator uses this together with node availability to select a local model
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

Orchestrator-seeded system personas (e.g. PMA, PAA, developer-go, test-engineer) may be updated on release; admin-created system personas and user/project/group-scoped personas are not modified by release updates (implementation distinguishes seeded vs admin-created, e.g. by flag or convention).

#### Personas Table Constraints

- Index: (`scope_type`, `scope_id`)
- Index: (`created_at`)
