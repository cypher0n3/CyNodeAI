# Draft: Orchestrator Specifications Table and Related Objects

- [Document Overview](#document-overview)
- [Goals and Scope](#goals-and-scope)
- [Specifications Table](#specifications-table)
  - [`specifications` Table Columns](#specifications-table-columns)
  - [GORM Model Conventions](#gorm-model-conventions)
  - [Specification Row and Object (Columns + Meta)](#specification-row-and-object-columns--meta)
  - [`specifications` Table Constraints](#specifications-table-constraints)
  - [`specifications` Table Behavior](#specifications-table-behavior)
- [Plan and Task Reference Tables](#plan-and-task-reference-tables)
  - [`plan_specifications` Join Table](#plan_specifications-join-table)
  - [`task_specifications` Join Table](#task_specifications-join-table)
- [Specification Object Type (API and MCP)](#specification-object-type-api-and-mcp)
  - [`SpecificationObject` Inputs (Create/update Row)](#specificationobject-inputs-createupdate-row)
  - [`SpecificationObject` Outputs (Get/list)](#specificationobject-outputs-getlist)
  - [`SpecificationObject` Validation](#specificationobject-validation)
- [Processing: Resolve Specifications for Plan or Task](#processing-resolve-specifications-for-plan-or-task)
  - [`ResolveSpecificationsForPlanOrTask` Algorithm](#resolvespecificationsforplanortask-algorithm)
  - [`ResolveSpecificationsForPlanOrTask` Behavior](#resolvespecificationsforplanortask-behavior)
- [MCP Tools and Schema Guidance](#mcp-tools-and-schema-guidance)
- [Related Skills (`default_skills`)](#related-skills-default_skills)
- [References](#references)

## Document Overview

**Incorporated into canonical specs as of 2026-03-18.**
Single source of truth: [postgres_schema.md](../tech_specs/postgres_schema.md), [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md), [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md), [requirements/](../requirements/).
See [_draft_specs_incorporation_and_conflicts_report.md](../dev_docs/_draft_specs_incorporation_and_conflicts_report.md) Section 4.3 and 8.

This draft specifies a **`specifications`** table in the orchestrator PostgreSQL database for storing technical specification references scoped to **projects**.
Plans and tasks **reference** specifications via join tables (they do not own them).
It defines the table schema (scalar columns plus a jsonb `meta` column per Go/GORM conventions), join tables, the specification row and object contract, processing for resolving specifications for a plan or task, MCP help tools, and the related PMA skill in `default_skills`.

When promoted, the table definition and object structure belong in [postgres_schema.md](../tech_specs/postgres_schema.md); MCP tool additions in [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md); skills are already in [default_skills](../../default_skills/).

## Goals and Scope

- Store specification references as first-class rows scoped to a **project** so they can be queried, audited, and revised independently.
- **Plans** and **tasks** reference specifications (many-to-many via join tables); specifications do not reference plans or tasks.
- Store identity, metadata, location, and ordering as **scalar columns** (indexed, queried via GORM); store only variable or nested data (e.g. traces_to, see_also, contract_subsections) in a **jsonb `meta`** column so Go structs map cleanly to the table while flexible content remains in one place.
- Requirements state **what** is required; specification references point to **which** technical specs define or constrain implementation or verification.

## Specifications Table

- Spec ID: `CYNAI.SCHEMA.Draft.SpecificationsTable` <a id="spec-cynai-schema-draft-specificationstable"></a>
- Status: draft

One row per specification reference.
Each row is tied to a **project**; plans and tasks reference specifications via join tables (they do not own them).
Schema details below are draft; when adopted, they MUST be moved into [postgres_schema.md](../tech_specs/postgres_schema.md) as the single source of truth.
Implementations use **Go and GORM** per [Go SQL database standards](../tech_specs/go_sql_database_standards.md): GORM models (structs with tags), snake_case column names, and jsonb only where variable or nested structure is needed.

### `specifications` Table Columns

Scalar columns for identity, metadata, location, and ordering (indexed and queried in Go via GORM).
At least one of `spec_id`, `ref`, or `description` MUST be non-null and non-empty (application or check constraint).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, NOT NULL)
- `spec_id` (text, nullable) - stable identifier; format host-defined
- `ref` (text, nullable) - alternative identifier (e.g. doc path, external id)
- `description` (text, nullable) - prose summary or spec description; Markdown when host uses Markdown
- `symbol` (text, nullable) - short name or symbol for the spec item
- `kind` (text, nullable) - category of the spec item (e.g. Type, Operation, Rule); host-defined
- `heading` (text, nullable) - display heading or title
- `status` (text, nullable) - lifecycle or maturity (e.g. draft, stable, deprecated); host-defined
- `since` (text, nullable) - version or date introduced; format host-defined
- `document_path` (text, nullable) - path to the containing document (repo-relative or URI)
- `anchor` (text, nullable) - fragment or anchor id for direct linking
- `source` (text, nullable) - provenance, URL, or path to the spec content
- `section` (text, nullable) - section or anchor within the document
- `spec_type` (text, nullable) - categorization (e.g. tech_spec, api_spec); host-defined (column name `spec_type` avoids Go reserved word `type`)
- `sort_order` (integer, nullable) - explicit order for display within the project
- `meta` (jsonb, nullable) - variable or nested data: traces_to, see_also, contract_subsections, and any additional host-defined keys (see [Specification meta (jsonb)](#specification-meta-jsonb))
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `created_by` (uuid, fk to `users.id`, nullable)

### GORM Model Conventions

- Table name: `specifications` (snake_case, plural per [postgres_schema naming](../tech_specs/postgres_schema.md)).
- Column names: snake_case; use struct tag `gorm:"column:spec_id"` etc. so Go field names (e.g. `SpecID`, `SpecType`) map to DB columns.
- Nullable scalar columns: use pointer types (e.g. `*string`, `*int`) or `sql.Null*`; GORM serializes nulls correctly.
- `meta`: use a type that serializes to/from jsonb (e.g. `datatypes.JSON` or a struct with `json` tags); document the recommended shape in [Specification meta (jsonb)](#specification-meta-jsonb).
- Identity constraint: validate in application code (or use a check constraint) that at least one of `spec_id`, `ref`, `description` is non-empty before create/update.

### Specification Row and Object (Columns + Meta)

- Spec ID: `CYNAI.SCHEMA.Draft.SpecificationObject` <a id="spec-cynai-schema-draft-specificationobject"></a>

A specification is stored as a **table row**: scalar columns for identity, metadata, location, and ordering (see [specifications table columns](#specifications-table-columns)), plus a **meta** (jsonb) column for variable or nested data.
This layout supports Go and GORM: struct fields map to columns; only the parts that are arrays or nested objects live in jsonb.
The shape is **generic**; hosts and users define semantics and allowed values for identity, kind, status, etc. (This project's [spec authoring](../docs_standards/spec_authoring_writing_and_validation.md) is one example of good practice.)

#### Scalar Fields (Table Columns)

Identity: `spec_id`, `ref`, `description` (at least one non-empty).
Metadata: `symbol`, `kind`, `heading`, `status`, `since`.
Location: `document_path`, `anchor`, `source`, `section`, `spec_type`.
Display: `sort_order`.
All are optional (nullable) except the identity constraint; semantics are host-defined.

#### Specification Meta (Jsonb)

The `meta` column holds a single JSON object for data that is variable, repeated, or nested and does not need its own column.

Recommended keys (all optional; host-defined semantics):

- `traces_to` (array of strings): references to requirements, other specs, or external items.
- `see_also` (array of strings): related doc or spec links.
- `contract_subsections` (array of objects): subsection-level detail (e.g. Inputs, Behavior, Algorithm).
  Element shape host-defined; common pattern: `heading` (string), `anchor` (string, optional), `kind` (string, optional).

Additional keys are allowed; the host or project defines semantics.
Implementations use a Go struct with `json` tags (or `datatypes.JSON`) for `meta`; unmarshal into the struct on read, marshal on write.

#### Constraints and Validation

- At least one of `spec_id`, `ref`, or `description` MUST be non-empty (application or check constraint).
- All other constraints (allowed values for `kind`, `status`, format of `spec_id`, etc.) are host-defined.
- Before create/update, validate identity in Go; the host MAY add validation for format or enums and return a structured error.

### `specifications` Table Constraints

- Check or application: at least one of `spec_id`, `ref`, or `description` is non-null and non-empty.
- Index: (`project_id`) for list all specifications for this project.
- Unique: (`project_id`, `spec_id`) WHERE `spec_id` IS NOT NULL AND `spec_id` != '' (optional; one row per spec_id per project).
  Alternatively unique on (`project_id`, `ref`) when ref is used as the logical key; host may allow multiple rows with same ref for different sections.
- Index: (`project_id`, `source`) for find by source when source is a path or doc id (optional).
- Index: (`project_id`, `sort_order`) for ordered listing (optional).
- GIN index on `meta` (optional) for containment or key existence queries on meta.

### `specifications` Table Behavior

- The orchestrator or gateway MUST enforce that only authorized callers can create, update, or delete specification rows; scope to the project the context is authorized to access.
- When a project is deleted, specification rows for that project_id SHOULD be deleted (cascade) or retained for audit per host policy.
- Plans and tasks reference specifications via join tables; deleting a specification row MAY remove or retain references in those join tables per host policy (e.g. cascade delete the join row or forbid delete when referenced).

## Plan and Task Reference Tables

Plans and tasks **reference** specifications; they do not own them.
Reference is many-to-many: a plan or task can reference multiple specifications, and a specification can be referenced by multiple plans or tasks.

### `plan_specifications` Join Table

- Spec ID: `CYNAI.SCHEMA.Draft.PlanSpecificationsTable` <a id="spec-cynai-schema-draft-planspecificationstable"></a>
- Status: draft

#### `plan_specifications` Table Columns

- `plan_id` (uuid, fk to `project_plans.id`, NOT NULL)
- `specification_id` (uuid, fk to `specifications.id`, NOT NULL)

#### `plan_specifications` Constraints

- Unique: (`plan_id`, `specification_id`).
- Index: (`plan_id`), (`specification_id`).

#### `plan_specifications` Table Behavior

- When a plan is deleted, join rows for that plan_id MAY be cascade-deleted; when a specification is deleted, join rows MAY be cascade-deleted per host policy.

### `task_specifications` Join Table

- Spec ID: `CYNAI.SCHEMA.Draft.TaskSpecificationsTable` <a id="spec-cynai-schema-draft-taskspecificationstable"></a>
- Status: draft

#### `task_specifications` Columns

- `task_id` (uuid, fk to `tasks.id`, NOT NULL)
- `specification_id` (uuid, fk to `specifications.id`, NOT NULL)

#### `task_specifications` Table Constraints

- Unique: (`task_id`, `specification_id`).
- Index: (`task_id`), (`specification_id`).

#### `task_specifications` Table Behavior

- Application MUST ensure the task's project (via task.project_id or task.plan_id -> project_plans.project_id) matches the specification's project_id when adding a reference.
- When a task is deleted, join rows for that task_id MAY be cascade-deleted; when a specification is deleted, join rows MAY be cascade-deleted per host policy.

## Specification Object Type (API and MCP)

- Spec ID: `CYNAI.SCHEMA.Draft.SpecificationObjectContract` <a id="spec-cynai-schema-draft-specificationobjectcontract"></a>
- Status: draft

Contract for the specification object when used in API or MCP payloads (create/update a specification row, or attach a specification reference to a plan or task by `specification_id`).

### `SpecificationObject` Inputs (Create/update Row)

- **Required:** `project_id` (uuid); at least one of `spec_id` (string), `ref` (string), or `description` (string, Markdown).
- **Optional (identity and metadata):** `symbol`, `kind`, `heading`, `status`, `since` (semantics and allowed values host-defined).
- **Optional (location and provenance):** `document_path`, `anchor`, `source`, `section`, `type` (categorization).
- **Optional (content and traceability):** `traces_to` (array of strings), `see_also` (array of strings), `contract_subsections` (array of objects; shape host-defined, e.g. heading, anchor, kind).
- **Optional (display):** `sort_order` (number).
  Full field semantics: [Specification row and object (columns + meta)](#specification-row-and-object-columns--meta).

### `SpecificationObject` Outputs (Get/list)

- When the host exposes specifications on a task or plan (e.g. in get/list responses), each element is a projection of the referenced specification row: same keys as the object structure, plus `id`, `project_id`, `created_at`, `updated_at` when included.

### `SpecificationObject` Validation

- Before insert or update, the implementation MUST validate identity: at least one of `spec_id`, `ref`, or `description` is non-empty.
- The host MAY enforce additional validation (e.g. format or allowed values for `kind`, `status`, `spec_id`) and reject invalid payloads with a structured error.
- When attaching a spec to a plan or task, the client sends `specification_id` (or a list of specification_ids); the join table is updated; the specification row is not modified.

When the host does not yet have a specifications table, specification references MAY be expressed in task content (e.g. in `description` or `acceptance_criteria`) or as requirement objects with `type: spec`; the PMA skill `pma-specification-object` can still guide building those payloads.

## Processing: Resolve Specifications for Plan or Task

- Spec ID: `CYNAI.SCHEMA.Draft.ResolveSpecificationsForPlanOrTask` <a id="spec-cynai-schema-draft-resolvespecificationsforplanortask"></a>
- Status: draft

Pipeline for resolving the set of specifications referenced by a plan or a task (for display, MCP responses, or downstream logic).

### `ResolveSpecificationsForPlanOrTask` Algorithm

<a id="algo-cynai-schema-draft-resolvespecificationsforplanortask"></a>

1. Given `plan_id` or `task_id`. <a id="algo-cynai-schema-draft-resolvespecificationsforplanortask-step-1"></a>
2. If plan_id: select `specification_id` from `plan_specifications` where `plan_id` = ?; if task_id: select `specification_id` from `task_specifications` where `task_id` = ?. <a id="algo-cynai-schema-draft-resolvespecificationsforplanortask-step-2"></a>
3. Load specification rows from `specifications` for those ids (join or IN clause); ensure only rows for the same project as the plan/task are returned (authorization). <a id="algo-cynai-schema-draft-resolvespecificationsforplanortask-step-3"></a>
4. Return ordered list: sort by `sort_order` (nulls last), then by `ref`, then by `created_at`. <a id="algo-cynai-schema-draft-resolvespecificationsforplanortask-step-4"></a>

### `ResolveSpecificationsForPlanOrTask` Behavior

Required procedure is defined by the [ResolveSpecificationsForPlanOrTask Algorithm](#algo-cynai-schema-draft-resolvespecificationsforplanortask) above.

## MCP Tools and Schema Guidance

- **`specification.help`** (read-only): Returns the specification object structure (required and optional fields), allowed or suggested `type` values, and examples.
  Allowlist: project_manager (PMA); when exposed to PAA or SBA, read-only.
  Implementation MUST derive the response from the actual schema (orchestrator or API) so agents build valid payloads.

- **Specification CRUD tools:** When the host adds a `specifications` table, MCP tools for create/update/delete of specification rows (e.g. `db.specification.create` with `project_id` and scalar fields plus optional `meta` object, `db.specification.list` by `project_id`, `db.specification.get`, `db.specification.update`, `db.specification.delete`) SHOULD be added and gated by the same allowlist and scope rules as task write tools (PMA may write; SBA read-only; PAA per catalog).
  Tools to attach or detach specifications to/from plans and tasks (e.g. `db.plan.specifications.set` or `db.task.specifications.set` with specification_id list) SHOULD be added so PMA can reference project-scoped specifications from plans and tasks.
  Tool names and argument schemas MUST be documented in [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md) when adopted.

## Related Skills (`default_skills`)

Skills that reference specification objects or the specifications table live under **default_skills** (see [default_skills/README.md](../../default_skills/README.md)).

- **pma-specification-object** ([pma_specification_object_skill.md](../../default_skills/pma_specification_object_skill.md)): Guides the PMA to build specification objects that conform to the host schema when creating project-scoped specification rows or when attaching specification references to plans and tasks.
  Use when creating specifications (project_id and scalar fields plus optional meta) or when referencing them from a plan or task; call `specification.help` before building create/update payloads.

- **pma-task-creation** ([pma_task_creation_skill.md](../../default_skills/pma_task_creation_skill.md)): When the task payload includes specification references (e.g. specification_ids or a list of specification objects for the project), use the specification object shape per `pma-specification-object` and attach via task_specifications.

- **pma-plan-creation** ([pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md)): Plan body and per-task sections reference "Requirements and Specifications"; when persisting spec refs via MCP, create or resolve project-scoped specifications and attach them to the plan or task via the join tables; ingest `pma-specification-object` as needed.

Skills MUST NOT link to `docs/draft_specs` or `dev_docs`; they link only to canonical docs and to other skills by registry name (see default_skills README).

## References

- Schema (canonical): [postgres_schema.md](../tech_specs/postgres_schema.md).
- Go SQL (GORM): [go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md).
- Default skills: [default_skills/README.md](../../default_skills/README.md), [pma_specification_object_skill.md](../../default_skills/pma_specification_object_skill.md).
- MCP catalog: [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md).
- Spec authoring (example of good practice in this project): [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).
- Requirement object (parallel concept): [Requirement object structure](../tech_specs/postgres_schema.md#spec-cynai-schema-requirementobject).
