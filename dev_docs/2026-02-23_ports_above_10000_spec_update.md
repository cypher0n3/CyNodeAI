# Proposed Spec Update: Default Ports Above 10000

## Summary

**Date:** 2026-02-23
**Target spec:** `docs/tech_specs/ports_and_endpoints.md` (CYNAI.STANDS.PortsAndEndpoints)
**Status:** Draft proposal for review

Move all CyNodeAI HTTP service default ports from the 8080 range (and 9190) into a single block above 10000 (12080-12090) to reduce conflicts with commonly used applications and simplify "ports to keep free" for developers.

## Rationale for Moving off the 8080 Range

Current defaults (8080, 8082, 8083, 8084, 9190) sit in or near the 8xxx band, which is heavily used by other software.

### Why the 8080 Range is Problematic

- **Port 8080** is the IANA-registered "HTTP Alternate" and is extremely common:
  - Default for Apache Tomcat, Spring Boot (embedded Tomcat/Jetty), Jenkins, WildFly/JBoss, GlassFish, Jetty.
  - Default for many web proxies (Squid, Apache Traffic Server, some router/firewall admin UIs).
  - Often used by dev servers (Vite, webpack-dev-server, etc.) and cloud runtimes (e.g. Google Cloud Run expects `PORT=8080`).
  - Frequently scanned for exposed admin interfaces.
- **8082, 8083, 8084** sit in the same 8xxx band used by countless dev tools, admin UIs, and alternate HTTP services.
- Running CyNodeAI alongside any of these (or another project using 8080) forces users to override ports, remember custom values, and keep overrides in sync across compose, env, and CLI config.

### Why Ports Above 10000

- Ports in the **10000-49151** range (IANA "User Ports") are less frequently used as defaults by mainstream dev servers and application servers, which tend to favor 3xxx, 5xxx, 8xxx.
- Using a **single contiguous block** (e.g. 12080-12090) makes it easy to remember "CyNodeAI uses 12xxx" and to firewall or document a small range.
- **Ollama** already uses **11434** (above 10000); keeping all other CyNodeAI HTTP ports in a nearby high range is consistent and avoids mixing 8xxx with 11xxx in docs and scripts.

## Conflicting Applications in Selected Range

The proposed block is 12080-12090.
Known IANA and common assignments in the surrounding range are as follows.

### 12000-12100 (Proposed Block: 12080-12090)

- **12000-12004:** IBM Enterprise Extender (SNA over IP).
  Rare on typical dev/workstation environments.
- **12005-12008:** DBISAM / Accuracer database servers.
  Niche.
- **12010:** ElevateDB Server.
  Niche.
- **12012-12013:** Vipera Messaging Service.
  Niche.

Ports **12080-12090** do not overlap these assignments and are not widely used by common desktop or dev tools.
Conflict risk on a developer machine or small server is low.

### Ports We Leave Unchanged

- **5432** - PostgreSQL.
  Universal standard; changing would break every existing deployment and tooling.
- **11434** - Ollama.
  Upstream default; inference proxy and docs assume 11434; changing would fragment the ecosystem.

## Proposed New Default Port Assignments

- **Component:** PostgreSQL
  - current: 5432
  - proposed: 5432
  - note: No change (standard).
- **Component:** User API Gateway
  - current: 8080
  - proposed: **12080**
  - note: Primary user-facing API.
- **Component:** Control-plane
  - current: 8082
  - proposed: **12082**
  - note: Node registration, dispatch.
- **Component:** MCP Gateway
  - current: 8083
  - proposed: **12083**
  - note: Optional profile.
- **Component:** API Egress
  - current: 8084
  - proposed: **12084**
  - note: Optional profile.
- **Component:** Worker API
  - current: 9190
  - proposed: **12090**
  - note: Single 12080-12090 block.
- **Component:** Ollama
  - current: 11434
  - proposed: 11434
  - note: No change (upstream default).

All CyNodeAI HTTP services then share the **12080-12090** block.
Each worker node hosts its Worker API on **12090**; there is no port incrementing for multiple nodes.

## Why These Specific Numbers

- **12080, 12082, 12083, 12084** preserve the last two digits of the current ports (80, 82, 83, 84), so existing mental model and any internal references (e.g. "gateway on 80") still map.
- **12090** for Worker API keeps it in the same decile as the orchestrator ports and leaves 12085-12089 free for future services or local overrides.
- One contiguous block simplifies conflict-avoidance text (e.g. "ensure 12080-12090 and 11434 are free") and firewall rules.

## Spec Change Summary (For `ports_and_endpoints.md`)

- In **Default Port Assignments**, replace 8080/8082/8083/8084/9190 with 12080/12082/12083/12084/12090; add one sentence that CyNodeAI HTTP defaults use the 12080-12090 block.
- In **Orchestrator Stack**, update listen ports and the example `host.containers.internal` URL to 12080, 12082, 12083, 12084 and Worker API to 12090.
- In **Worker Node**, change default Worker API from 9190 to 12090; each node uses 12090 (remove or adjust any "multiple nodes on one host" incrementing example so the default is 12090 per node).
- In **CLI (Cynork)**, change default gateway URL from `http://localhost:8080` to `http://localhost:12080`.
- In **Conflict Avoidance**, list 5432, 12080, 12082, 12090, 11434 (and optional 12083, 12084) as required/optional.
- In **E2E and BDD**, replace 8080/8082/9190 with 12080/12082/12090.
- In **Environment and Config Overrides**, update all default port values and the Cynork default URL.

Other docs and code that reference the current defaults (e.g. `development_setup.md`, `worker_node.md`, `cynork_cli.md`, `node_bootstrap_example.yaml`, docker-compose, env defaults) would need a follow-up pass to align with the new spec once approved.

## Backward Compatibility and Migration

- This is a **defaults** change.
  All overrides (`USER_GATEWAY_LISTEN_ADDR`, `LISTEN_ADDR`, `CONTROL_PLANE_PORT`, `CYNORK_GATEWAY_URL`, node YAML `worker_api.listen_port`, etc.) remain valid; users who already override ports are unaffected.
- New installs and docs would use the new defaults.
  Existing installs that rely on 8080/8082/9190 without overrides would need to either set env/config to the old values or switch to the new ports and update any bookmarks or scripts.
- Recommendation: document the change in release notes and, if desired, support a short transition period where the old defaults are mentioned as deprecated in the spec.

## Next Steps

1. Review and approve this proposal.
2. Update `docs/tech_specs/ports_and_endpoints.md` per the spec change summary.
3. Update all references in `docs/` (e.g. `development_setup.md`, `mvp_plan.md`, `worker_node.md`, `cynork_cli.md`, examples) and in code (orchestrator, worker_node, cynork defaults, docker-compose, scripts) to use the new defaults.
4. Run `just docs-check` (or `just ci` if code changes) and adjust any E2E/BDD or integration tests that hardcode 8080/8082/9190.
