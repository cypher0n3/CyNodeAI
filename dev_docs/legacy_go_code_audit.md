# Legacy Go Code Audit

- [1. Summary](#1-summary)
- [2. Canonical Layout (Reference)](#2-canonical-layout-reference)
- [3. Legacy Go Code (Not Referenced)](#3-legacy-go-code-not-referenced)
  - [3.1 `worker_node/node_manager` (Nested Module)](#31-worker_nodenode_manager-nested-module)
  - [3.2 `worker_node/worker_api` (Nested Module)](#32-worker_nodeworker_api-nested-module)
  - [3.3 `orchestrator/api_egress` (Nested Module)](#33-orchestratorapi_egress-nested-module)
- [4. `go_tools/`](#4-go_tools)
- [5. `go.mod` Locations (For Clarity)](#5-gomod-locations-for-clarity)
- [6. Cleanup Completed](#6-cleanup-completed)
  - [6.1 Legacy Nested Modules Removed](#61-legacy-nested-modules-removed)
  - [6.2 Docs Updated](#62-docs-updated)
- [7. References Checked](#7-references-checked)

## 1. Summary

**Date:** 2026-02-19
**Scope:** All folders and nested directories; identify legacy Go code and confirm it is not referenced.

- **Active Go layout:** `go.work` and the justfile use three modules: `go_shared_libs`, `orchestrator`, `worker_node`.
  Builds and CI use `worker_node/cmd/node-manager`, `worker_node/cmd/worker-api`, and `orchestrator/cmd/*` (including `orchestrator/cmd/api-egress`).
  No top-level `node/` directory exists; docs refer to `worker_node/` for worker implementation.
- **Legacy (safe to remove after your confirmation):** Nested Go modules that are not in `go.work`, are not used by compose/justfile, and either reference a missing `contracts` path or duplicate code under `orchestrator/cmd/` or `worker_node/cmd/`.
- **Optional / clarify:** `go_tools/` is not in `go.work` and is not used by the justfile (which uses `go install ...@latest`).
  Retain only if you want version-pinned tool deps via a tools module.

---

## 2. Canonical Layout (Reference)

| Item                              | Purpose                                                                                           |
| --------------------------------- | ------------------------------------------------------------------------------------------------- |
| `go.work`                         | Lists `./go_shared_libs`, `./orchestrator`, `./worker_node` only.                                 |
| `justfile` `go_modules`           | `go_shared_libs`, `orchestrator`, `worker_node`.                                                  |
| `worker_node/docker-compose.yml`  | Uses `worker_node/cmd/node-manager/Containerfile` and `worker_node/cmd/worker-api/Containerfile`. |
| `orchestrator/docker-compose.yml` | Uses `orchestrator/cmd/api-egress/Containerfile` (and other `orchestrator/cmd/*` Containerfiles). |
| Docs / MVP plan                   | Reference `worker_node/cmd/node-manager`, `worker_node/cmd/worker-api`, `orchestrator/cmd/*`.     |

No `node/` directory exists at repo root; `meta.md` and [`ai_files/ai_coding_instructions.md`](../ai_files/ai_coding_instructions.md) have been updated to reference `worker_node/` and `orchestrator/` as the implementation directories.

---

## 3. Legacy Go Code (Not Referenced)

The following nested modules are not in `go.work` and are not used by compose or the justfile.

### 3.1 `worker_node/node_manager` (Nested Module)

- **What:** Nested module with its own `go.mod` (module `github.com/cypher0n3/cynodeai/worker_node/node_manager`).
- **Why legacy:**
  - Not in `go.work`.
    Justfile and CI only run against the top-level `worker_node` module.
  - `go.mod` has `replace github.com/cypher0n3/cynodeai/contracts => ../../contracts`; there is no `contracts` directory at repo root, so this module does not build.
  - Docker compose uses `worker_node/cmd/node-manager/Containerfile`, which builds `./worker_node/cmd/node-manager`, not this nested module.
- **References:** Only internal to this tree.
  No active code or compose references `worker_node/node_manager/`.
- **Verdict:** **Legacy; safe to remove** (after you confirm).

### 3.2 `worker_node/worker_api` (Nested Module)

- **What:** Nested module with its own `go.mod` (module `github.com/cypher0n3/cynodeai/worker_node/worker_api`).
- **Why legacy:**
  - Same as above: not in `go.work`, replace points to non-existent `../../contracts`.
  - Compose uses `worker_node/cmd/worker-api/Containerfile`, which builds `./worker_node/cmd/worker-api`.
- **References:** Only within this tree (e.g. `worker_node/worker_api/cmd/worker-api/main.go` imports `worker_node/worker_api/executor`).
  No active code or compose references this nested module.
- **Verdict:** **Legacy; safe to remove** (after you confirm).

### 3.3 `orchestrator/api_egress` (Nested Module)

- **What:** Nested module with its own `go.mod` (module `github.com/cypher0n3/cynodeai/orchestrator/api_egress`).
- **Why legacy:**
  - Not in `go.work`.
    Active API egress binary lives under `orchestrator/cmd/api-egress/` (main orchestrator module).
  - `orchestrator/docker-compose.yml` uses `orchestrator/cmd/api-egress/Containerfile`, not `orchestrator/api_egress/`.
- **References:** Docs and code refer to the service "api-egress" and to `orchestrator/cmd/api-egress/`, not to the `orchestrator/api_egress/` directory.
- **Verdict:** **Legacy; safe to remove** (after you confirm).

---

## 4. `go_tools/`

- **What:** Module `github.com/cypher0n3/cynodeai/gotools` with a `tools.go` that blanks-imports `govulncheck` and `staticcheck`.
- **Usage:** Not in `go.work`.
  The justfile's `install-go-tools` runs `go install ...@latest` for golangci-lint, staticcheck, and govulncheck.
  It does not build or run from `go_tools/`.
- **Verdict:** Not referenced by `go.work` or justfile.
  Retain only if you want a dedicated tools module for version pinning; otherwise it can be treated as legacy and removed or repurposed.

---

## 5. `go.mod` Locations (For Clarity)

| Path                              | In go.work? | Used by justfile / compose?                        |
| --------------------------------- | ----------- | -------------------------------------------------- |
| `go_shared_libs/go.mod`           | Yes         | Yes (go_modules)                                   |
| `orchestrator/go.mod`             | Yes         | Yes (go_modules, compose uses orchestrator/cmd/\*) |
| `worker_node/go.mod`              | Yes         | Yes (go_modules, compose uses worker_node/cmd/\*)  |
| `go_tools/go.mod`                 | No          | No                                                 |
| `worker_node/node_manager/go.mod` | No          | No (and replace => ../../contracts is broken)      |
| `worker_node/worker_api/go.mod`   | No          | No (same broken replace)                           |
| `orchestrator/api_egress/go.mod`  | No          | No (compose uses orchestrator/cmd/api-egress)      |

---

## 6. Cleanup Completed

The following was done after confirmation.

### 6.1 Legacy Nested Modules Removed

- `worker_node/node_manager/` (removed)
- `worker_node/worker_api/` (removed)
- `orchestrator/api_egress/` (removed)
- `go_tools/` (removed)

### 6.2 Docs Updated

- [`meta.md`](../meta.md) and [`ai_files/ai_coding_instructions.md`](../ai_files/ai_coding_instructions.md) now reference `worker_node/` and `orchestrator/` as the implementation directories (obsolete `node/` placeholder mention removed).

---

## 7. References Checked

- `go.work`, `justfile`, `worker_node/go.mod`, `worker_node/docker-compose.yml`, `orchestrator/docker-compose.yml`
- All `go.mod` and import paths under `worker_node/`, `orchestrator/`, `go_shared_libs/`, `go_tools/`
- Grep for `contracts`, `worker_node/worker_api`, `worker_node/node_manager`, `orchestrator/api_egress`, and for compose/Containerfile paths
- `meta.md`, `ai_files/ai_coding_instructions.md`, `dev_docs/mvp_phase1_completion_plan.md`, and other docs that reference worker/orchestrator paths
