@suite_e2e
Feature: Access control on the user gateway

  As a security reviewer
  I want unauthenticated requests to user-scoped APIs to be rejected
  So that policy is enforced at the gateway edge (default deny for protected routes)

@req_access_0001 @spec_cynai_access_doc_accesscontrol
Scenario: Unauthenticated list-tasks request is rejected
  Given the orchestrator API is running
  When I call GET "/v1/tasks" without an Authorization header
  Then the response status is 401
