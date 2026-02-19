# MVP Feature Files Fleshed Out

- [1 Summary](#1-summary)
- [2 Changes](#2-changes)
- [3 Tag Traceability](#3-tag-traceability)
- [4 Notes](#4-notes)

## 1 Summary

Feature files under `features/` were expanded to cover MVP behavior per
[spec authoring and validation](../docs/docs_standards/spec_authoring_writing_and_validation.md).
Each feature has a user story and each scenario has `@req_*` and `@spec_*` tags.

## 2 Changes

Summary of file-level and scenario-level updates.

### 2.1 Single-Node Happy Path Feature File

- User story left directly under `Feature:`; comment about inference precondition moved below it.
- Added four scenarios identified in `mvp_specs_gaps_closure_status.md` (BDD gap):
  - **Node fetches config after registration** - REQ-ORCHES-0113, REQ-WORKER-0135; config payload.
  - **Node sends config acknowledgement** - REQ-WORKER-0135; ConfigAckV1.
  - **Dispatcher uses per-node worker URL and token** - REQ-WORKER-0100, REQ-ORCHES-0122; WorkerApiAuth, Orchestrator.
  - **Orchestrator fails fast when no inference path** - REQ-BOOTST-0002; BootstrapSource.

### 2.2 New Feature Files

| File | User story | Scenarios |
|------|------------|-----------|
| `initial_auth.feature` | Sign in with local credentials so I can call protected APIs | Login, token refresh, logout, get current user |
| `node_registration_and_config.feature` | Nodes register and receive config so orchestrator can dispatch | Register with PSK, capability reporting, config fetch, config ack |
| `orchestrator_startup.feature` | Orchestrator fails fast when no inference path so I know system is not ready | Fail fast when no inference path |

### 2.3 Features README

- Added a short table listing each feature file and its focus.

## 3 Tag Traceability

- Requirement tags follow `@req_<domain>_<nnnn>` (e.g. `@req_identy_0104`).
- Spec tags follow `@spec_cynai_<domain>_<path>` from Spec IDs (e.g. `@spec_cynai_identy_authenticationmodel`).
- All new and updated scenarios reference at least one REQ and one spec anchor as required.

## 4 Notes

- Default `just lint-md` does not target `.feature` files.
  If markdownlint is run explicitly on `features/*.feature`, MD041 may flag the first line (Gherkin `Feature:`).
  Exclude `.feature` in that case or ignore MD041 for this directory.
- Step definitions (Godog) are not added here; scenarios are written for future implementation.

---

Generated 2026-02-19.
