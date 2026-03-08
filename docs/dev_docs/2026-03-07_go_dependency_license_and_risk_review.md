# Go Dependency License and Risk Review

- [1 Summary](#1-summary)
- [2 Methodology](#2-methodology)
- [3 License Compatibility](#3-license-compatibility)
  - [3.1 Typical Licenses in the Tree](#31-typical-licenses-in-the-tree)
  - [3.2 Notable Packages Checked](#32-notable-packages-checked)
  - [3.3 Absence of GPL and AGPL](#33-absence-of-gpl-and-agpl)
- [4 Versioning and Maintenance Risks](#4-versioning-and-maintenance-risks)
  - [4.1 +Incompatible Modules](#41-incompatible-modules)
  - [4.2 Legacy JWT Fork (Not Present)](#42-legacy-jwt-fork-not-present)
  - [4.3 Obscure or Niche Dependencies (Mostly Agents)](#43-obscure-or-niche-dependencies-mostly-agents)
  - [4.4 Very Old Pinned Versions](#44-very-old-pinned-versions)
- [5 Per-Module Overview](#5-per-module-overview)
- [6 Direct Dependencies: Obscure or Replaceable](#6-direct-dependencies-obscure-or-replaceable)
- [7 Recommendations](#7-recommendations)
- [8 References](#8-references)

## 1 Summary

**Date:** 2026-03-07.
**Scope:** All Go workspace modules: `agents`, `cynork`, `e2e`, `go_shared_libs`, `orchestrator`, `worker_node`.
**Purpose:** Assess license compatibility and dependency risks (obscure, unmaintained, or versioning concerns) for Go packages used across the CyNodeAI workspace.

- **License compatibility:** Dependencies appear to use licenses compatible with commercial and open-source distribution (Apache-2.0, MIT, BSD-3-Clause).
  No GPL/AGPL dependencies were identified in the aggregated dependency list.
- **Risks:** A small set of versioning and maintenance concerns exist: `+incompatible` modules, one legacy JWT fork not present in this repo but common in the ecosystem, and a large transitive tree from the `agents` module (langchaingo) that includes niche or less widely used packages.
- **Recommendation:** Continue using `just vulncheck-go` and consider adding periodic license reporting (e.g. `go-licenses report`) once runnable in your environment; optionally reduce surface by auditing whether all langchaingo-backed features are required.

---

## 2 Methodology

- Aggregated all unique modules via `go list -m all` across the six workspace modules.
- Cross-referenced `go.mod` and `go.sum` for direct vs indirect and `+incompatible` tags.
- Used pkg.go.dev and public documentation for license and maintenance status of notable packages.
- No automated license scanner was run (e.g. `go-licenses` failed in this environment due to sumdb cache permissions).

---

## 3 License Compatibility

Dependencies use permissive licenses compatible with commercial and open-source distribution.

### 3.1 Typical Licenses in the Tree

- **Apache-2.0:** Used by Google (golang.org/x, google.*, cloud.google.com), Docker/Moby, Kubernetes-related, OpenTelemetry, many gRPC/API clients.
- **MIT:** Used by Charm Bracelet (glamour, lipgloss), stretchr/testify, many small utilities, langchaingo (tmc/langchaingo).
- **BSD-3-Clause:** Used by modernc.org/sqlite, many Go standard-style packages.

These are generally compatible with each other and with commercial use, subject to retaining copyright and license notices.

### 3.2 Notable Packages Checked

- `github.com/docker/docker` (v28.5.1+incompatible): Apache-2.0 (and MIT for some components); used indirectly by testcontainers in orchestrator.
- `modernc.org/sqlite`: BSD-3-Clause (worker_node); no BSL in the Go wrapper.
- `github.com/tmc/langchaingo`: MIT (agents).
- `golang.org/x/*`, `google.golang.org/*`, `cloud.google.com/*`: Typically Apache-2.0 or BSD-3-Clause.

### 3.3 Absence of GPL and AGPL

No GPL or AGPL-licensed dependencies were identified in the current dependency set.

---

## 4 Versioning and Maintenance Risks

Items below are worth monitoring for versioning or maintenance reasons.

### 4.1 +Incompatible Modules

These modules use a major version in the path that does not match Go module semantics, so they are tagged `+incompatible`:

- **github.com/airbrake/gobrake** (agents, transitive): Error-reporting; license permissive; maintenance status not verified.
- **github.com/cenkalti/backoff** v2.2.1 (agents, transitive): Backoff; v4 exists and is used elsewhere (e.g. orchestrator uses cenkalti/backoff/v4).
- **github.com/docker/docker** v28.5.1 (orchestrator, via testcontainers): Actively maintained (Moby); Apache-2.0.
- **github.com/gofrs/uuid** v4.4.0 (orchestrator, worker_node, cynork, e2e): Maintained; MIT.
  The project also uses `github.com/google/uuid` directly; gofrs/uuid is pulled in transitively (e.g. by godog/cucumber).
- **github.com/google/flatbuffers** (agents, transitive): Used by some ML/vector libs; license permissive.
- **github.com/uber/jaeger-client-go** (agents, transitive): Tracing; Apache-2.0; Jaeger project is maintained.

Risk: `+incompatible` does not imply incompatible licenses; it means the module may not follow Go semver for major versions, which can complicate upgrades.

### 4.2 Legacy JWT Fork (Not Present)

The ecosystem has moved from `dgrijalva/jwt-go` and `form3tech-oss/jwt-go` to `golang-jwt/jwt`.

This repository does **not** depend on form3tech-oss/jwt-go or dgrijalva/jwt-go; the orchestrator uses `github.com/golang-jwt/jwt/v5` directly.

No action required; included for context.

### 4.3 Obscure or Niche Dependencies (Mostly Agents)

The `agents` module depends on `github.com/tmc/langchaingo`, which pulls a large transitive set including:

- LLM and embedding SDKs (e.g. AssemblyAI, AWS Bedrock, Cohere, Mistral, IBM watsonx, Metaphor, Zep, Pinecone, Weaviate, Milvus, OpenSearch, Chroma).
- Testcontainers modules for Chroma, Milvus, MongoDB, MySQL, OpenSearch, Postgres, Redis, Weaviate.

Some of these are less widely used or maintained by smaller teams (e.g. metaphorsystems/metaphor-go, getzep/zep-go, amikos-tech/chroma-go).

Risk: Higher chance of abandonment or slow security fixes; consider whether all integrations are needed or can be trimmed.

### 4.4 Very Old Pinned Versions

- **honnef.co/go/tools** v0.0.0-20190523083050-ea95bdfd59fc (agents, transitive): Very old staticcheck/analysis version.
  Current staticcheck is typically installed via `go install honnef.co/go/tools/cmd/staticcheck@latest`; this pinned version is from a dependency, not from project tooling.

---

## 5 Per-Module Overview

- **go_shared_libs:** No external dependencies; no license or risk concerns.
- **orchestrator:** Direct deps are well-known (godog, jwt v5, testcontainers, gorm, etc.); indirect docker/docker and gofrs/uuid are +incompatible but licensed permissively and maintained.
- **worker_node:** Small tree; modernc.org/sqlite (BSD-3-Clause); gofrs/uuid indirect.
- **cynork:** Small tree; Charm Bracelet (MIT), Cobra, godog; gofrs/uuid indirect.
- **e2e:** Minimal (godog); gofrs/uuid indirect.
- **agents:** Single direct external dep (langchaingo) but very large transitive set; contains the niche SDKs and the old honnef.co/go/tools pin.

---

## 6 Direct Dependencies: Obscure or Replaceable

This section lists direct (non-workspace) dependencies that are relatively obscure or worth evaluating for replacement by another package or by our own code.

### 6.1 github.com/oklog/ulid/v2 (Orchestrator)

- **Where used:** [orchestrator/internal/handlers/nodes.go](../../orchestrator/internal/handlers/nodes.go): generates a lexicographically sortable config version ID (`ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()`).
- **Obscurity:** Not obscure; ~4.3k stars, Apache-2.0, maintained.
- **Replace with alternative:** Not necessary for adoption reasons.
- **Replace with own code:** ULID is a small spec (timestamp 48b + randomness 80b, Crockford base32).
  A minimal implementation is on the order of 30-50 lines using `crypto/rand` and `time`.
  Replacing removes one direct dep if you want to minimize surface; low priority.

### 6.2 Github.com/peterh/liner.com/peterh/liner (Cynork)

- **Where used:** [cynork/cmd/chat.go](../../cynork/cmd/chat.go): interactive line input with history and editing (e.g. for chat prompts).
- **Obscurity:** Moderate; ~1k stars, MIT, single maintainer; pure Go, readline-style.
- **Replace with alternative:** `github.com/chzyer/readline` is another pure-Go option with similar features and adoption.
  Switching would be a small refactor (different API) with similar risk profile.
- **Replace with own code:** Only if you drop interactive editing (e.g. plain `bufio.Scanner` or `fmt.Scanln`).
  That would simplify the CLI but remove history and line editing; only worth it if cynork chat is used in non-interactive or scripted flows only.

**Recommendation:** Keep unless you explicitly want to reduce deps or drop interactive editing; if replacing, prefer chzyer/readline over reimplementing.

### 6.3 Github.com/glebarez/sqlite.com/glebarez/sqlite (Worker_node_node)

- **Where used:** [worker_node/internal/telemetry/store.go](../../worker_node/internal/telemetry/store.go): GORM driver for SQLite telemetry store (pure-Go, no CGO).
- **Obscurity:** Less widely known than `mattn/go-sqlite3`; builds on `modernc.org/sqlite` (and glebarez/go-sqlite).
  Chosen to avoid CGO for portability and simpler builds.
- **Replace with alternative:** `mattn/go-sqlite3` is the most used Go SQLite driver but requires CGO.
  Switching would increase build complexity and break pure-Go / cross-compile workflows.
- **Replace with own code:** Not realistic; SQLite is a full DB engine.

**Recommendation:** Keep; the pure-Go choice is intentional.
Monitor glebarez and modernc.org/sqlite for security and maintenance; both are actively maintained.

### 6.4 Github.com/tmc/langchaingo.com/tmc/langchaingo (Agents)

- **Where used:** PMA and SBA agent loops ([agents/internal/pma](../../agents/internal/pma), [agents/internal/sba](../../agents/internal/sba)): LLM calls (Ollama), tool schema, MCP tool wrappers, agent execution.
- **Obscurity:** Not obscure; it is the main LangChain-style framework for Go (MIT).
  The concern is the large transitive tree (many LLM/vector SDKs and testcontainers) and some niche transitive deps.
- **Replace with alternative:** No drop-in replacement that significantly shrinks the tree while keeping the same feature set.
  Other options (e.g. direct Ollama HTTP client + minimal tool marshalling) would be more code and less off-the-shelf tooling.
- **Replace with own code:** Possible in principle: thin client to Ollama API + your own tool-call loop and MCP wiring.
  That would remove the langchaingo tree and many transitive deps but would require implementing and maintaining agent loop, tool schema handling, and any future LLM integrations yourself.

**Recommendation:** Keep for now unless product direction is to own the full agent stack.
  To reduce risk without replacing: trim unused langchaingo providers/vector backends (e.g. exclude optional integrations you do not use) so that fewer transitive deps are pulled in.

### 6.5 Other Direct Dependencies (Not Obscure)

- **github.com/cucumber/godog:** BDD; widely used.
- **github.com/charmbracelet/glamour** (cynork): Markdown in terminal; Charm Bracelet is popular.
- **github.com/creack/pty** (cynork): PTY; widely used (e.g. by Kubernetes, Docker).
- **github.com/spf13/cobra** (cynork): CLI; standard in the Go ecosystem.
- **golang.org/x/*, google/uuid, gorm, testcontainers:** Well-known and maintained.

No replacement recommended for these.

---

## 7 Recommendations

1. **Licenses:** Treat current dependency set as license-compatible for normal commercial and open-source use; keep retaining notices as required by each license.
2. **Vulnerabilities:** Keep running `just vulncheck-go` (govulncheck) regularly.
3. **License reporting:** When feasible (e.g. sumdb/cache permissions or CI environment), run `go-licenses report ./...` from each module (or a single module that pulls the full tree) and archive the report for compliance.
4. **agents surface:** Periodically review whether all langchaingo-backed providers and vector DBs are required; reducing unused integrations will shrink the transitive tree and maintenance risk.
5. **gofrs/uuid:** Optional cleanup: if possible, prefer consolidating on `google/uuid` and avoiding indirect pull of gofrs/uuid (e.g. by upstream godog/cucumber adopting google/uuid); low priority.
6. **Obscure direct deps:** Consider only (a) replacing **oklog/ulid** with a small in-house ULID if minimizing deps is a goal; (b) replacing **peterh/liner** with chzyer/readline or simple stdin read if you prefer a different maintenance profile or non-interactive chat; keep **glebarez/sqlite** and **langchaingo** unless you have a clear reason to take on CGO or own the agent stack.

---

## 8 References

- [go-licenses](https://github.com/google/go-licenses) for license reporting.
- [pkg.go.dev license tabs](https://pkg.go.dev) (e.g. `?tab=licenses`) for per-module license.
- [Go vulnerability database](https://vuln.go.dev) and `govulncheck` (used by `just vulncheck-go`).
