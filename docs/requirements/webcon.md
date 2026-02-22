# WEBCON Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the **Web Console** (`WEBCON`) domain.
Web Console-specific behavior is defined in the [Web Console](../tech_specs/web_console.md) tech spec with spec IDs `CYNAI.WEBCON.*`.
Requirements that apply to both the Web Console and the CLI (capability parity, credentials, preferences, nodes, system settings, projects) remain in [client.md](client.md).

## 2 Requirements

- **REQ-WEBCON-0001:** Web Console: no direct DB; gateway only; secrets write-only in UI; least privilege; no embedded credentials.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0001"></a>
- **REQ-WEBCON-0100:** The web console MUST NOT connect directly to PostgreSQL.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0100"></a>
- **REQ-WEBCON-0101:** The web console MUST call the User API Gateway for all operations.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0101"></a>
- **REQ-WEBCON-0102:** Secrets MUST be write-only in the UI.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0102"></a>
- **REQ-WEBCON-0103:** The UI MUST never display plaintext secret values after creation.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0103"></a>
- **REQ-WEBCON-0104:** The UI MUST support least privilege and MUST not expose admin features to non-admin users.
  [CYNAI.WEBCON.Security](../tech_specs/web_console.md#spec-cynai-webcon-security)
  <a id="req-webcon-0104"></a>
- **REQ-WEBCON-0105:** The console MUST not embed privileged service credentials.
  [CYNAI.WEBCON.Implementation](../tech_specs/web_console.md#spec-cynai-webcon-implementation)
  <a id="req-webcon-0105"></a>
- **REQ-WEBCON-0106:** The console MUST not bypass gateway authorization and auditing.
  [CYNAI.WEBCON.Implementation](../tech_specs/web_console.md#spec-cynai-webcon-implementation)
  <a id="req-webcon-0106"></a>
- **REQ-WEBCON-0107:** The console MUST treat gateway responses as the source of truth.
  [CYNAI.WEBCON.Implementation](../tech_specs/web_console.md#spec-cynai-webcon-implementation)
  <a id="req-webcon-0107"></a>
- **REQ-WEBCON-0108:** The console MUST avoid storing bearer tokens in localStorage.
  [CYNAI.WEBCON.AuthModel](../tech_specs/web_console.md#spec-cynai-webcon-authmodel)
  <a id="req-webcon-0108"></a>
- **REQ-WEBCON-0109:** The console MUST support logout and token invalidation.
  [CYNAI.WEBCON.AuthModel](../tech_specs/web_console.md#spec-cynai-webcon-authmodel)
  <a id="req-webcon-0109"></a>
- **REQ-WEBCON-0110:** The console MUST enforce HTTPS in production deployments.
  [CYNAI.WEBCON.Deployment](../tech_specs/web_console.md#spec-cynai-webcon-deployment)
  <a id="req-webcon-0110"></a>
- **REQ-WEBCON-0111:** CORS SHOULD be avoided by preferring same-origin hosting behind the gateway.
  [CYNAI.WEBCON.Deployment](../tech_specs/web_console.md#spec-cynai-webcon-deployment)
  <a id="req-webcon-0111"></a>
- **REQ-WEBCON-0112:** The web console MUST support full CRUD for skills (create, list, view, update, delete) via the User API Gateway, with the same controls as defined in the skills spec (scope visibility, scope elevation permission, auditing on write).
  [CYNAI.WEBCON.SkillsManagement](../tech_specs/web_console.md#spec-cynai-webcon-skillsmanagement)
  [CYNAI.SKILLS.SkillManagementCrud](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud)
  <a id="req-webcon-0112"></a>
- **REQ-WEBCON-0113:** The web console MUST provide Swagger UI (or equivalent API documentation UI) for the User API Gateway so that authenticated admins can discover and try API endpoints.
  [CYNAI.WEBCON.SwaggerUi](../tech_specs/web_console.md#spec-cynai-webcon-swaggerui)
  <a id="req-webcon-0113"></a>
