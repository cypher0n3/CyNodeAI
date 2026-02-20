# SKILLS Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SKILLS` domain.
It covers storage, tracking, and exposure of AI skills files so that inference models that support skills can consume them.

## 2 Requirements

- **REQ-SKILLS-0001:** The system MUST store, track, and make available AI skills files to inference models that support skills.
  [CYNAI.SKILLS.Doc.SkillsStorageAndInference](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-doc-skillsstorageandinference)
  <a id="req-skills-0001"></a>
- **REQ-SKILLS-0100:** Skills files MUST be stored in a defined, versioned store with stable paths or identifiers.
  [CYNAI.SKILLS.SkillStore](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillstore)
  <a id="req-skills-0100"></a>
- **REQ-SKILLS-0101:** The system MUST maintain a registry (or equivalent) of skill metadata for discovery and filtering (e.g. identifier, name, scope).
  [CYNAI.SKILLS.SkillRegistry](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillregistry)
  <a id="req-skills-0101"></a>
- **REQ-SKILLS-0102:** Stored skills MUST be retrievable by a stable identifier so that inference callers can request skills by reference.
  [CYNAI.SKILLS.SkillRetrieval](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillretrieval)
  <a id="req-skills-0102"></a>
- **REQ-SKILLS-0103:** The system MUST expose a way for inference models that support skills to receive skill content (e.g. by inclusion in context or by resolved retrieval).
  [CYNAI.SKILLS.InferenceExposure](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-inferenceexposure)
  <a id="req-skills-0103"></a>
- **REQ-SKILLS-0104:** Skills MUST be scoped so that only relevant skills are offered to a given inference request; scope levels include user, group, project, and global.
  [CYNAI.SKILLS.SkillRegistry](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillregistry)
  <a id="req-skills-0104"></a>
- **REQ-SKILLS-0107:** Skills MUST be scoped to the user by default; a newly loaded skill is visible only to that user's inference requests unless the user directs a broader scope.
  [CYNAI.SKILLS.SkillRegistry](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillregistry)
  [CYNAI.SKILLS.SkillLoading](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillloading)
  <a id="req-skills-0107"></a>
- **REQ-SKILLS-0108:** A user MAY set a skill's scope to a broader level (group, project, or global) when they direct (e.g. at load time or on update); the system MUST allow this only if the user has appropriate permissions for that scope (e.g. group membership, project access, or global admin).
  [CYNAI.SKILLS.SkillScopeElevation](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillscopeelevation)
  <a id="req-skills-0108"></a>
- **REQ-SKILLS-0105:** Users MUST be able to load (upload) skills through the web interface; the system MUST store the skill and register it for discovery.
  [CYNAI.SKILLS.SkillLoading](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillloading)
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-skills-0105"></a>
- **REQ-SKILLS-0106:** Users MUST be able to load skills via the CLI by uploading a markdown file (e.g. SKILL.md); the CLI MUST call the gateway and the system MUST store the skill and register it.
  [CYNAI.SKILLS.SkillLoading](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillloading)
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-skills-0106"></a>
- **REQ-SKILLS-0110:** The system MUST scan skill content for malicious or policy-violating patterns before accepting a load (or update) and MUST reject the load when a match is found.
  [CYNAI.SKILLS.SkillAuditing](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillauditing)
  <a id="req-skills-0110"></a>
- **REQ-SKILLS-0111:** Malicious patterns MUST include at least: hidden instructions (e.g. HTML comments or other content intended to instruct the model without visible disclosure), instructions that explicitly tell the model to ignore or override other instructions, and instructions that would prompt the model to expose secrets or bypass security controls.
  [CYNAI.SKILLS.SkillAuditing](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillauditing)
  <a id="req-skills-0111"></a>
- **REQ-SKILLS-0112:** The system SHOULD support periodic or on-demand rescan of stored skills and SHOULD flag or quarantine skills that subsequently match malicious patterns (e.g. after pattern rules are updated).
  [CYNAI.SKILLS.SkillAuditing](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillauditing)
  <a id="req-skills-0112"></a>
- **REQ-SKILLS-0113:** When a skill load or update is rejected because of the security scan, the system MUST return to the caller (the user who submitted the load) the rejection reason, including at least the match category (e.g. hidden instructions, instruction override, secret bypass) and the exact text that triggered the rejection, so the user can fix the content.
  [CYNAI.SKILLS.SkillAuditing](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillauditing)
  <a id="req-skills-0113"></a>
- **REQ-SKILLS-0114:** The system MUST expose MCP tools so that models (agents) can perform full CRUD on skills for the user when directed (create, list, get, update, delete); all operations MUST follow the same controls as web and CLI (auditing on write, default user scope, scope elevation only with permission, gateway enforcement and auditing).
  [CYNAI.SKILLS.SkillToolsMcp](../tech_specs/skills_storage_and_inference.md#skill-tools-via-mcp-crud)
  [CYNAI.MCPGAT.Doc.GatewayEnforcement](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-doc-gatewayenforcement)
  <a id="req-skills-0114"></a>
- **REQ-SKILLS-0115:** The web interface and CLI MUST support full CRUD for skills: create (load), list (with optional scope/owner filter), get (content and metadata by identifier), update (content and/or metadata including scope, subject to same auditing and scope permissions as load), and delete (remove from store and registry).
  All operations MUST go through the User API Gateway; the same controls (auth, scope elevation permission, auditing on write) apply.
  [CYNAI.SKILLS.SkillManagementCrud](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud)
  <a id="req-skills-0115"></a>
- **REQ-SKILLS-0116:** The system MUST provide a built-in default skill that describes how AIs should interact with CyNodeAI (e.g. MCP tools, gateway usage, conventions).
  This skill MUST be loaded by default for inference: when the system exposes skills to an inference request that supports skills, this default skill MUST be included in the set offered (e.g. global scope, always included).
  [CYNAI.SKILLS.DefaultCyNodeAISkill](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-defaultcynodeaiskill)
  <a id="req-skills-0116"></a>
- **REQ-SKILLS-0117:** The content of the default CyNodeAI interaction skill MUST be updated regularly (e.g. with product releases or on a defined schedule) so that it reflects current capabilities, APIs, and conventions.
  [CYNAI.SKILLS.DefaultCyNodeAISkill](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-defaultcynodeaiskill)
  <a id="req-skills-0117"></a>
