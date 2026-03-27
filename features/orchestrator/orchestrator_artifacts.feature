@suite_orchestrator
Feature: Orchestrator scope-partitioned artifacts API

  As an authenticated user
  I want to create, read, update, delete, and list artifacts in my scope
  So that blob metadata and RBAC match orchestrator_artifacts_storage.md

Background:

  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"

@req_orches_0127 @spec_cynai_orches_artifactsapicrud
Scenario: User-scoped artifact create and read
  Given I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with query "?scope_level=user&path=bdd%2Fhello.txt" and body "hello-artifacts"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And I GET the stored artifact blob
  And the response status is 200
  And the last raw response body equals "hello-artifacts"

@req_orches_0127 @spec_cynai_orches_rbacforartifacts
Scenario: Another user cannot read a user-scoped artifact
  Given a user exists with handle "eve" and password "eve123"
  And I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with query "?scope_level=user&path=bdd%2Fprivate.txt" and body "secret"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And I login as "eve" with password "eve123"
  And I GET the stored artifact blob
  And the response status is 403

@req_orches_0127 @spec_cynai_orches_artifactsapicrud
Scenario: Group-scoped artifact create and read
  Given the BDD group id is "11111111-1111-1111-1111-111111111111"
  And I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with group scope path "bdd/group-note.txt" and body "group-scope"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And I GET the stored artifact blob
  And the response status is 200
  And the last raw response body equals "group-scope"

@req_orches_0127 @spec_cynai_orches_artifactsapicrud
Scenario: Project-scoped artifact create and read
  Given the default project id for handle "admin" is resolved
  And I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with project scope path "bdd/project-note.txt" and body "project-scope"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And I GET the stored artifact blob
  And the response status is 200
  And the last raw response body equals "project-scope"

@req_orches_0127 @spec_cynai_orches_artifactsapicrud
Scenario: Global-scoped artifact create and read
  Given I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with global scope path "bdd/global-note.txt" and body "global-scope"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And I GET the stored artifact blob
  And the response status is 200
  And the last raw response body equals "global-scope"

@req_orches_0127 @spec_cynai_orches_rbacforartifacts
Scenario: Cross-principal read via explicit grant on user-scoped artifact
  Given a user exists with handle "eve" and password "eve123"
  And I am logged in as "admin" with password "admin123"
  When I POST /v1/artifacts with query "?scope_level=user&path=bdd%2Fshared-grant.txt" and body "grant-me"
  Then the response status is 201
  And I store the artifact_id from the last JSON response
  And user "eve" has a read grant for the stored artifact
  And I login as "eve" with password "eve123"
  And I GET the stored artifact blob
  And the response status is 200
  And the last raw response body equals "grant-me"

@req_orches_0167 @spec_cynai_orches_artifactsmcpforpmapaa
Scenario: MCP PMA agent may call artifact.put
  Given I am logged in as "admin" with password "admin123"
  When the MCP agent "pm" calls artifact.put for user "admin" path "bdd/mcp-put.txt" with text "mcp-ok"
  Then the response status is 200

@req_orches_0167 @spec_cynai_mcpgat_doc_gatewayenforcement
Scenario: MCP sandbox agent cannot call artifact.put
  Given I am logged in as "admin" with password "admin123"
  When the MCP agent "sandbox" calls artifact.put for user "admin" path "bdd/mcp-deny.txt" with text "no"
  Then the response status is 403
