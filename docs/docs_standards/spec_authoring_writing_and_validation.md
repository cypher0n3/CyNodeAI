# Spec Authoring, Writing, and Validation Standards

- [Markdown Conventions](#markdown-conventions)
- [Tech Spec Conventions](#tech-spec-conventions)
- [Documentation Layers](#documentation-layers)
  - [Requirements (`docs/requirements/`)](#requirements-docsrequirements)
  - [Technical Specifications (`docs/tech_specs/`)](#technical-specifications-docstech_specs)
  - [Feature Files (`features/`)](#feature-files-features)
- [Spec Items](#spec-items)
  - [Core Definitions](#core-definitions)
  - [Mandatory Structure for a Spec Item](#mandatory-structure-for-a-spec-item)
  - [Algorithm and Processing](#algorithm-and-processing)
  - [Language Reference Blocks](#language-reference-blocks)
  - [Backticked Symbols and Kind Tokens](#backticked-symbols-and-kind-tokens)
  - [Allowed Inline HTML](#allowed-inline-html)
- [Cross-References and Traceability](#cross-references-and-traceability)
  - [Canonical Link Target](#canonical-link-target)
  - [Requirements to Spec References](#requirements-to-spec-references)
  - [Spec to Requirement Traceability](#spec-to-requirement-traceability)
  - [Feature to Requirement and Spec References (Gherkin)](#feature-to-requirement-and-spec-references-gherkin)
  - [Requirement IDs](#requirement-ids)
- [Documentation Validation Pipeline](#documentation-validation-pipeline)
  - [Order of Checks](#order-of-checks)
  - [When PATHS is Set](#when-paths-is-set)
  - [After Creating or Changing Markdown](#after-creating-or-changing-markdown)
  - [Justfile / Doc Check Quick Reference](#justfile--doc-check-quick-reference)
- [Validator Requirements Summary](#validator-requirements-summary)
- [Authoring Checklist](#authoring-checklist)

## Markdown Conventions

These standards apply to this document.

See:

- [`markdown_conventions.md`](./markdown_conventions.md)

## Tech Spec Conventions

- Tech specs must be **highly prescriptive** and **leave no room for interpretation**.
  Implementation choices, behavior, contracts, and algorithms must be stated unambiguously so that implementers and reviewers can verify compliance without inferring intent.
- Use prose to describe technical implementation and behavior.
- Keep code in tech specs minimal.
  Prefer function signatures, constants, generic type definitions, and short usage snippets.
- There must be a single source of truth for each specification.
  Related specs should link to it rather than restate it.
- Tech specs should avoid introducing new normative RFC-2119 obligations ("MUST", "SHOULD", etc.).
  Normative obligations belong in `docs/requirements/` and should be referenced from tech specs.
- Use the latest structured errors approach for error handling.
  Remove references to legacy sentinel errors.

## Documentation Layers

This repository uses three documentation layers that serve different purposes.
Keeping each layer focused reduces duplication and improves testability and traceability.

### Requirements (`docs/requirements/`)

Requirements docs define **what is required**.
They are the canonical home for normative RFC-2119 obligations ("MUST", "SHOULD", etc.).
They are implementation-agnostic and should not describe design choices or internal architecture.
Requirements exist to state outcomes, constraints, and acceptance criteria at the "what" level.

Rules:

- Requirements should be atomic and testable.
  Prefer one obligation per `REQ-<DOMAIN>-<NNNN>` entry.
- Requirements should link to the relevant implementation spec sections via `spec-*` anchors.
- Requirements must be stable, uniquely identified, and easy to reference from specs and tests.

See:

- [`docs/requirements/`](../requirements/)

### Technical Specifications (`docs/tech_specs/`)

Tech specs define **how we build it**.
They are **highly prescriptive**: they must describe architecture, design, flows, and implementation details in unambiguous terms so that there is no room for interpretation.
Implementations must be verifiable against the spec without inferring author intent.
Tech specs should define Spec Items with Spec IDs and stable anchors.

Rules:

- Tech specs should cross-link back to applicable `REQ-*` items.
  Do not duplicate long blocks of requirement text in specs.

See:

- [`docs/tech_specs/`](../tech_specs/)

### Feature Files (`features/`)

Feature files define **user stories and executable acceptance tests** in Gherkin.
They exist to make requirements and specs testable end-to-end using Godog (Cucumber) scenarios.

Rules:

- Feature files must live under [`features/`](../../features/).
- Each feature file MUST be confined to a single major component so BDD can run per component without cross-component dependencies.
- A feature that intentionally spans multiple major components MUST be tagged and treated as end-to-end only.
- Each feature file MUST live under the suite directory implied by its suite tag.
  Example: `@suite_orchestrator` feature files live under `features/orchestrator/`.
- Each Feature must include a user story narrative block directly under the `Feature:` line:
  `As a ...`, `I want ...`, `So that ...`.
- Each Feature MUST include exactly one suite tag on a line immediately above the `Feature:` line.
  Allowed suite tags are defined in [`features/README.md`](../../features/README.md) Section 3.1.
- Each Feature or Scenario must link to both:
  - at least one requirement ID (`REQ-<DOMAIN>-<NNNN>`), and
  - at least one tech spec anchor (`spec-*`) that defines the relevant implementation design.

## Spec Items

This section defines the required structure for Spec Items in `docs/tech_specs/`.

### Core Definitions

- **Spec Item**: A uniquely identifiable contract element (interface, type, operation, rule, or constant).
- **Spec ID**: A stable, language-neutral identifier for a Spec Item.
  It is the canonical key for cross-links, requirements, and validation tooling.
  Spec ID domains must be kept in sync with the canonical requirements domains.
  See [`requirements_domains.md`](./requirements_domains.md).

Rules:

- Spec IDs must not include the token `Normative` (or any `.Normative` suffix).
  Normative obligations belong in `docs/requirements/` as `REQ-*` entries.

### Mandatory Structure for a Spec Item

Every Spec Item section must follow this structure in this order.

#### Canonical Heading

- A Spec Item must have a numbered Markdown heading.
- Heading level must match the surrounding document hierarchy.
- Numbering must remain strictly increasing within the document (existing behavior).
- Numbering segment count must match heading level as: \(segments = heading\_level - 1\).
  For example H2 has 1 segment, H3 has 2 segments, and so on.
- Headings must be unique within the document.
  Validators must treat headings as duplicates if their text matches after stripping any leading numbering prefix.

#### Spec ID Anchor Line

Append the Spec Item anchor to the end of the `Spec ID:` bullet line (not the heading line).
This avoids extra blank lines between the heading, anchor, and list blocks while keeping the anchor adjacent to the Spec ID it is derived from.

Rules:

- The anchor must appear at the end of the `Spec ID:` bullet line.
- Exactly one Spec ID anchor per Spec Item heading.
- Inline HTML is disallowed except for the narrowly-scoped anchor forms listed in [Allowed Inline HTML](#allowed-inline-html).

#### Spec ID Line

Immediately after the Spec Item heading line, add a dash-bulleted list whose first bullet is:

```markdown
- Spec ID: `CYNAI.DOMAIN.PATH` <a id="spec-cynai-domain-path"></a>
```

Rules:

- The Spec ID line must be the first non-blank line after the Spec Item heading line.
- Exactly one Spec ID line per Spec Item heading.
- Spec IDs must be globally unique across `docs/`.
- Validators must enforce that the anchor value matches the Spec ID normalization rule:
  lowercase, dots replaced with dashes, prefixed with `spec-`.

#### Optional Metadata Lines

Optional metadata may follow the `Spec ID:` bullet as additional bullets in the same list.

Examples:

- `- Status: draft|stable|deprecated`
- `- Since: vX.Y`
- `- See also: <links>`

Rules:

- Metadata lines must be single-line key/value pairs using `<Key>: <Value>`.
- Any sub-bullets must be dash bullets and must be indented consistently under their parent bullet.

#### Contract Content

The Spec Item body must include contract content unless the item is purely informational.

Minimum contract requirements by kind:

- **Interface**: required operations, invariants, error semantics, concurrency expectations.
- **Type**: layout rules, constraints, canonical encoding, validation rules.
- **Operation**: inputs, outputs, behavior, error conditions, side effects, ordering, cancellation semantics, optional algorithm.
- **Rule**: scope, preconditions, outcomes, error semantics, observability, optional algorithm.
- **Constant**: meaning, allowed ranges, encoding rules.

#### Contract Section Layout

- Contract subsections must be one heading level deeper than the Spec Item.
- Heading depth must not exceed H5 (`#####`).
  If nesting would require H6+, restructure the document.
- Contract subsection numbering must extend the Spec Item numbering by appending one numeric segment.
- Contract subsection headings must be unique within the file even after stripping numbering.
  Include the Spec Item symbol in each contract subsection heading text to keep headings unique across a file.

Cross-language cancellation and context guidance:

- If an Operation can block, perform I/O, or otherwise take non-trivial time, it should define cancellation semantics.
- Specify cancellation in both:
  - Inputs (declare the presence of a cancellation context or token as an input concept).
  - Concurrency (define concurrency expectations and observable behavior on cancellation).
- Implementations must follow the target language's best practices for context and cancellation propagation.
  For example, Go commonly passes `ctx context.Context` as the first parameter by convention.

Recommended contract subsection headings:

- Operation:
  - `<Backticked Symbol> Inputs`
  - `<Backticked Symbol> Outputs`
  - `<Backticked Symbol> Behavior`
  - `<Backticked Symbol> Error conditions`
  - `<Backticked Symbol> Concurrency` (include cancellation semantics here when applicable)
  - `<Backticked Symbol> Ordering and determinism` (optional)
  - `<Backticked Symbol> Algorithm` (optional)
  - `<Backticked Symbol> Processing` (optional)
- Rule:
  - `<Backticked Symbol> Scope`
  - `<Backticked Symbol> Preconditions`
  - `<Backticked Symbol> Outcomes`
  - `<Backticked Symbol> Error conditions`
  - `<Backticked Symbol> Observability`
  - `<Backticked Symbol> Algorithm` (optional)
  - `<Backticked Symbol> Processing` (optional)

### Algorithm and Processing

- Use `Algorithm` for a normative sequence of decisions and required execution order.
- Use `Processing` for pipeline or staged transformation models.
- `Algorithm` may be a long, numbered procedure (business procedure spec) for complex operations.
  In these cases, keep `Behavior` brief and use `Algorithm` to describe the full required procedure.
- If an `Algorithm` subsection exists, the corresponding `Behavior` subsection must link to the Algorithm anchor.

Algorithm anchors and step anchors:

- Every `Algorithm` subsection must define a stable Algorithm anchor on its own line at the start of the section (immediately after the Algorithm heading).
  The anchor line must be separated from the procedure list by a blank line.

  Example (showing both the Algorithm anchor and a step anchor):

  ````markdown
  ### `Package.AddFile` Algorithm

  <a id="algo-cynai-core-package-addfile"></a>

  1. Validate filesystem path exists. <a id="algo-cynai-core-package-addfile-step-1"></a>
  ````

- Algorithm anchors must be derived from the Spec ID normalization rule:
  - lowercase, dots replaced with dashes, prefixed with `algo-`
  - for example `Spec ID: CYNAI.CORE.Package.AddFile` => `algo-cynai-core-package-addfile`
- If an Algorithm subsection contains a numbered procedure, it may define step anchors to link requirements and feature files to a specific step.
  Step anchors must be placed at the end of the corresponding ordered or unordered list item line.
  Step anchor format examples:
  - `1. Validate filesystem path exists. <a id="algo-cynai-core-package-addfile-step-1"></a>`
  - `- Compare checksums. <a id="algo-cynai-core-package-addfile-step-2-3"></a>` (step 2, substep 3)

### Language Reference Blocks

Language references are optional but strongly recommended.

Rules:

- Reference headings must not exceed H5 (`#####`).
- A Spec Item may include multiple references (one per language).
- A reference section should contain only the minimal signature or definition snippet.
  Long examples belong in external examples or per-language docs and should be linked.

#### Reference Heading Format

Use this canonical format:

- `##### <N.N.N.X> <Backticked Symbol> Reference - <Language>` (H5)
- `#### <N.N.X> <Backticked Symbol> Reference - <Language>` (H4, when the Spec Item is H3)

#### Reference Anchors

Each Reference section must define a stable reference anchor on its own line directly above the referenced fenced code block.
The reference anchor line must be separated from the code block by a blank line (so there is always an empty line above the code fence).
Example:

````markdown
<a id="ref-go-cynai-core-package-readfile"></a>

```go
func (p *Package) ReadFile(path string) ([]byte, error)
```
````

Rules:

- The language token must be lowercase (for example `go`, `rust`, `zig`).
- The remainder must be derived from the Spec ID using the same normalization as Spec ID anchors.

### Backticked Symbols and Kind Tokens

Spec Item headings use this canonical form:

- `<Numbering> <Backticked Symbol> <Kind>`

Only these kind tokens are allowed:

- `Interface`
- `Type`
- `Operation`
- `Rule`
- `Field`
- `Constant`

Rules:

- Do not use language-specific kind tokens like "Method", "Function", or "Struct" in canonical spec headings.
- The backticked symbol should be stable across languages and avoid language-specific naming conventions when possible.

### Allowed Inline HTML

Inline HTML is disallowed except for these anchor forms:

- Spec Item anchors: `<a id="spec-..."></a>`
- Reference anchors: `<a id="ref-<lang>-..."></a>`
- Algorithm anchors: `<a id="algo-..."></a>`
- Algorithm step anchors: `<a id="algo-...-step-..."></a>`
- Requirement entry anchors: `<a id="req-..."></a>` (requirements docs only; on a continuation line under the requirement list item, after any spec reference link lines).
- Normative anchors: `<a id="norm-..."></a>` (deprecated; do not introduce in new tech specs).

Rules:

- Anchor tags must contain only an `id` attribute.
  Only this exact tag form is allowed: `<a id="..."></a>`.
- Spec Item anchors must be appended to the end of the `Spec ID:` bullet line.
- Algorithm anchors must be on their own line at the start of the `Algorithm` section (immediately after the Algorithm heading, separated by a blank line).
- Algorithm step anchors must be appended to the end of the corresponding ordered or unordered list item line within an `Algorithm` section.
- Reference anchors must be on their own line directly above a fenced code block, separated from the code block by a blank line.
- Requirement entry anchors must appear on a continuation line under the requirement list item (after the `- REQ-<DOMAIN>-<NNNN>: ...` line and after any spec reference link lines).
- All other inline HTML must be rejected by tooling.
  This is enforced by markdownlint with a custom rule and may also be enforced by the Python validation scripts.

## Cross-References and Traceability

This section defines how to link specs, requirements, and feature files and how to maintain traceability between them.

### Canonical Link Target

All cross-references into tech specs should prefer Spec ID anchors over heading-derived anchors.

Example:

- `../tech_specs/api_core.md#spec-cynai-core-package-readfile`

### Requirements to Spec References

In requirements docs, use a list-style requirements format.
Each requirement is a list item with continuation lines for spec references and the requirement anchor.

Example:

- REQ-MCPGAT-0001: The MCP gateway MUST enforce gateway policy for all MCP tool calls.
  [CYNAI.MCPGAT.Doc.GatewayEnforcement](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-doc-gatewayenforcement)
  <a id="req-mcpgat-0001"></a>

Rules:

- Each requirement entry is a list item: `- REQ-<DOMAIN>-<NNNN>: <short label>.`
- Continuation lines: one or more spec reference links (Spec ID as link text, href to `spec-*` anchor), then the `req-*` anchor on its own line.
- Requirements are the canonical normative "what".
  Tech specs should link back to requirements rather than re-stating requirements as normative spec content.

### Spec to Requirement Traceability

Tech specs must link to the applicable requirements they implement.

Format:

- `Traces To: [REQ-MCPGAT-0001](../requirements/mcpgat.md#req-mcpgat-0001), [REQ-MCPGAT-0002](../requirements/mcpgat.md#req-mcpgat-0002)`

Rules:

- Requirement identifiers must be stable.
- Do not embed long requirement text inside specs.
  Link instead.

### Feature to Requirement and Spec References (Gherkin)

Feature files should reference both requirement IDs and Spec IDs for scenarios that validate requirement-defined behavior.
This enables BDD traceability between:
requirements ("what"), specs ("how"), and tests (Gherkin scenarios executed by Godog).

Use tags derived from requirement IDs and Spec IDs used in this repository.

- `@spec_cynai_mcpgat_doc_gatewayenforcement`

Requirement tag format:

- `@req_worker_0001`

Rules:

- Requirement tags must be derived from the requirement ID by removing `REQ-`, replacing `-` with `_`, and lowercasing.
  Example: `REQ-WORKER-0001` => `@req_worker_0001`.
- Each Scenario must include both `@req_*` and `@spec_*` tags.

Rules:

- `@spec_*` tags must be derived from Spec ID by replacing `.` with `_` and lowercasing.
  Example: `CYNAI.MCPGAT.Doc.GatewayEnforcement` => `@spec_cynai_mcpgat_doc_gatewayenforcement`.
- If a feature declares Spec references, all referenced Spec IDs must exist.
- If a feature declares requirement references, all referenced requirement IDs must exist.
- If a Scenario references requirements, it must also reference at least one spec anchor that defines the scenario's behavior.

### Requirement IDs

Requirement ID format:

- `REQ-<DOMAIN>-<NNNN>` (4 digits, e.g. 0001-9999)

Rules:

- IDs must be unique across the requirements set.
- IDs must not encode file paths or heading numbers.

## Documentation Validation Pipeline

The documentation pipeline runs via **`just lint-md`** (and in CI via `just ci`, which includes `just lint-md`).

Use the project `justfile`; do not invoke validation scripts directly unless instructed.

### Order of Checks

Currently this project uses `just lint-md` for Markdown linting; additional checks may be added.

### When PATHS is Set

When path-aware doc checks are available, running with a path (e.g. `just lint-md 'path/to/file.md'`) runs only path-aware checks on the given files.

Checks that require a full-repo view (e.g. `validate-go-defs-index`, `validate-req-references`, `audit-coverage`) are skipped when only specific paths are checked.

### After Creating or Changing Markdown

Run `just lint-md` (or `just lint-md 'path/to/file.md'`) for the file you changed.
Fix any reported errors and re-run until the check passes.

### Justfile / Doc Check Quick Reference

- `just lint-md` - Lint all Markdown (default target).
- `just lint-md 'path/to/file.md'` - Lint specific file(s).
- `just ci` - Full CI (includes `just lint-md`).

## Validator Requirements Summary

Tooling should enforce at minimum:

- Blank line after every header line.
- Headings unique within a file (including after stripping numbering).
- Numbering segment count consistent with heading level for numbered headings.
- Inline HTML restricted to the allowed anchor forms only.
- Spec Items detected by canonical heading format.
- Spec IDs and anchors present exactly once per Spec Item and consistent with normalization rules.
- Example and reference code fences declare their language.
- Indexes link to Spec ID anchors and do not restate spec content.

## Authoring Checklist

When adding a new Spec Item:

- Add a heading in canonical format.
- Add the Spec ID anchor and the `Spec ID:` bullet (and optional metadata).
- Write the contract in language-neutral terms.
- Add reference blocks and examples with per-language headings when needed.
- Link to other specs using Spec ID anchors.
- Run `just lint-md` and fix issues until it passes.
