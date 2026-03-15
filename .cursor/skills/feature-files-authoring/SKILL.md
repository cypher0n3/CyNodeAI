---
name: feature-files-authoring
description: Applies project Gherkin and feature-file standards when writing or editing files in features/. Use when creating or editing .feature files, scenarios, or when the user asks to write or fix BDD/Godog feature files.
---

# Feature Files Authoring

## Overview

Follow the project's spec authoring and feature-file conventions.
Canonical source: [docs/docs_standards/spec_authoring_writing_and_validation.md](../../docs/docs_standards/spec_authoring_writing_and_validation.md), [features/README.md](../../features/README.md), and [markdown_conventions.md](../../docs/docs_standards/markdown_conventions.md).

## Before Writing

- One feature file per major component; file location must match suite tag (e.g. `@suite_cynork` => `features/cynork/`).
  Allowed suite tags: see features/README.md Section 3.1.

## Feature Structure (Mandatory)

- Exactly one **suite tag** on the line immediately above `Feature:` (e.g. `@suite_cynork`).
- **User story** block directly under `Feature:`: `As a ...`, `I want ...`, `So that ...`.
- Each Scenario must link to at least one requirement and at least one spec.

## Traceability Tags

- **Requirement tag**: `@req_<domain>_<nnnn>` from `REQ-<DOMAIN>-<NNNN>` (remove REQ-, replace - with _, lowercase).
  Example: REQ-WORKER-0001 => `@req_worker_0001`.
- **Spec tag**: `@spec_<spec_id>` from Spec ID (dots to underscores, lowercase).
  Example: CYNAI.MCPGAT.Doc.GatewayEnforcement => `@spec_cynai_mcpgat_doc_gatewayenforcement`.
- Each Scenario that validates requirement-defined behavior must include both `@req_*` and `@spec_*` tags.

## Example Skeleton

```gherkin
@suite_cynork
Feature: Example capability

  As a user
  I want something
  So that outcome

  @req_identy_0104 @spec_cynai_identy_authenticationmodel
  Scenario: ...
    Given ...
    When ...
    Then ...
```

## After Editing

- Run `just lint-md` (or `just lint-md 'path/to/file.feature'`) if the project lints feature files; otherwise run `just lint-md` and fix any reported issues.
