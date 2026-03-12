# CONNEC Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `CONNEC` domain.
It covers connector framework behavior and external connector integration patterns.

## 2 Requirements

- **REQ-CONNEC-0001:** Connector catalog and instances; credentials in PostgreSQL (envelope encryption); default-deny policy and audit; User API Gateway surface.
  [CYNAI.CONNEC.ConnectorModel](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel)
  [CYNAI.CONNEC.ConnectorUserApiGateway](../tech_specs/connector_framework.md#spec-cynai-connec-connusergwy)
  <a id="req-connec-0001"></a>
- **REQ-CONNEC-0100:** The orchestrator MUST maintain a connector catalog of supported connector types and their operations.
  [CYNAI.CONNEC.ConnectorModel](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel)
  <a id="req-connec-0100"></a>
- **REQ-CONNEC-0101:** Users MUST be able to install a connector instance, enable it, disable it, and uninstall it.
  [CYNAI.CONNEC.ConnectorModel](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel)
  <a id="req-connec-0101"></a>
- **REQ-CONNEC-0102:** A connector instance MUST be scoped to an owner identity (user or group) and MAY be scoped to a project.
  [CYNAI.CONNEC.ConnectorModel](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel)
  <a id="req-connec-0102"></a>
- **REQ-CONNEC-0103:** Connector instances MUST have stable identifiers for policy rules and auditing.
  [CYNAI.CONNEC.ConnectorModel](../tech_specs/connector_framework.md#spec-cynai-connec-connectormodel)
  <a id="req-connec-0103"></a>
- **REQ-CONNEC-0104:** Connector credentials MUST be stored in PostgreSQL as ciphertext with a key identifier for envelope encryption.
  [CYNAI.CONNEC.ConnectorCredentialStorage](../tech_specs/connector_framework.md#spec-cynai-connec-conncredstorage)
  <a id="req-connec-0104"></a>
- **REQ-CONNEC-0105:** Only the service responsible for executing connector operations MUST be able to decrypt connector credentials.
  [CYNAI.CONNEC.ConnectorCredentialStorage](../tech_specs/connector_framework.md#spec-cynai-connec-conncredstorage)
  <a id="req-connec-0105"></a>
- **REQ-CONNEC-0106:** Credential rotation MUST be supported without changing connector instance identifiers.
  [CYNAI.CONNEC.ConnectorCredentialStorage](../tech_specs/connector_framework.md#spec-cynai-connec-conncredstorage)
  <a id="req-connec-0106"></a>
- **REQ-CONNEC-0107:** Credential revocation MUST support immediate deactivation.
  [CYNAI.CONNEC.ConnectorCredentialStorage](../tech_specs/connector_framework.md#spec-cynai-connec-conncredstorage)
  <a id="req-connec-0107"></a>
- **REQ-CONNEC-0108:** The orchestrator MUST enforce a default-deny policy for connector operations.
  [CYNAI.CONNEC.ConnectorPolicyAuditing](../tech_specs/connector_framework.md#spec-cynai-connec-connpolicy)
  <a id="req-connec-0108"></a>
- **REQ-CONNEC-0109:** Policy MUST be evaluated per operation with subject, action, resource, and task context.
  [CYNAI.CONNEC.ConnectorPolicyAuditing](../tech_specs/connector_framework.md#spec-cynai-connec-connpolicy)
  <a id="req-connec-0109"></a>
- **REQ-CONNEC-0110:** Connector operations MUST emit audit logs including subject identity, connector instance, operation, and decision.
  [CYNAI.CONNEC.ConnectorPolicyAuditing](../tech_specs/connector_framework.md#spec-cynai-connec-connpolicy)
  <a id="req-connec-0110"></a>
- **REQ-CONNEC-0111:** Connector management tools (install, enable, disable, rotate, revoke) MUST be restricted to authorized subjects.
  [CYNAI.CONNEC.ConnectorToolSurfaceMcp](../tech_specs/connector_framework.md#spec-cynai-connec-conntoolmcp)
  <a id="req-connec-0111"></a>
- **REQ-CONNEC-0112:** Connector invocation tools (read, send) MUST be policy-gated per operation and per connector instance.
  [CYNAI.CONNEC.ConnectorToolSurfaceMcp](../tech_specs/connector_framework.md#spec-cynai-connec-conntoolmcp)
  <a id="req-connec-0112"></a>
- **REQ-CONNEC-0113:** Tool calls MUST be audited with task context when invoked during task execution.
  [CYNAI.CONNEC.ConnectorToolSurfaceMcp](../tech_specs/connector_framework.md#spec-cynai-connec-conntoolmcp)
  <a id="req-connec-0113"></a>
- **REQ-CONNEC-0114:** The User API Gateway MUST expose endpoints to install, enable, disable, and uninstall connector instances.
  [CYNAI.CONNEC.ConnectorUserApiGateway](../tech_specs/connector_framework.md#spec-cynai-connec-connusergwy)
  <a id="req-connec-0114"></a>
- **REQ-CONNEC-0115:** The User API Gateway MUST expose endpoints to manage connector credentials (create, rotate, revoke, disable).
  [CYNAI.CONNEC.ConnectorUserApiGateway](../tech_specs/connector_framework.md#spec-cynai-connec-connusergwy)
  <a id="req-connec-0115"></a>
- **REQ-CONNEC-0116:** The User API Gateway SHOULD expose connector run history and audit views for visibility and debugging.
  [CYNAI.CONNEC.ConnectorUserApiGateway](../tech_specs/connector_framework.md#spec-cynai-connec-connusergwy)
  <a id="req-connec-0116"></a>
- **REQ-CONNEC-0117:** OpenClaw connectors MUST run behind an orchestrator-controlled boundary.
  [CYNAI.CONNEC.OpenClawCompatibility](../tech_specs/connector_framework.md#spec-cynai-connec-openclaw)
  <a id="req-connec-0117"></a>
- **REQ-CONNEC-0118:** The boundary MUST enforce policy, auditing, allowlists, and response validation for every connector operation.
  [CYNAI.CONNEC.OpenClawCompatibility](../tech_specs/connector_framework.md#spec-cynai-connec-openclaw)
  <a id="req-connec-0118"></a>
- **REQ-CONNEC-0119:** Connector credentials MUST NOT be returned to agents or worker sandboxes.
  [CYNAI.CONNEC.OpenClawCompatibility](../tech_specs/connector_framework.md#spec-cynai-connec-openclaw)
  <a id="req-connec-0119"></a>
- **REQ-CONNEC-0120:** OpenClaw connectors MUST NOT be executed inside worker sandboxes that run arbitrary agent code.
  [CYNAI.CONNEC.OpenClawCompatibility](../tech_specs/connector_framework.md#spec-cynai-connec-openclaw)
  <a id="req-connec-0120"></a>
- **REQ-CONNEC-0121:** Connector invocation MUST be scoped to a connector instance id and operation name with subject identity and task context.
  [CYNAI.CONNEC.OpenClawCompatibility](../tech_specs/connector_framework.md#spec-cynai-connec-openclaw)
  <a id="req-connec-0121"></a>
