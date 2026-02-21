@suite_orchestrator
Feature: Orchestrator Startup

  As an operator
  I want the orchestrator to remain running but not ready when no inference path is available
  So that I can fix configuration (settings, credentials, and policy) before accepting work

# "Inference Path" Here Means at Least One Dispatchable Inference-Capable target
# - a Dispatchable (Registered, Config-Acked) Local Inference Worker (Ollama or Similar), Or
# - an External Provider Path via API Egress When Configured and allowed
#
# When Local Inference Exists, the Orchestrator is Expected to Select a Default Project Manager Model and Ensure It is Ready
# Using a Resource-Aware Sliding-Scale Policy (And Prefer the Orchestrator Host by Default When a Dispatchable Worker Exists there)
# Operators Can Seed Selection Policy Values And/or/or Pin a Local Model Name via Orchestrator Bootstrap and Update Them Later via the CLI
# Or Admin Web Console System Settings UI (`tinyllama` is the Smallest Supported Fallback Model for Limited systems)
#
# When No Inference Target is Available, Readyz Returns 503 and a Message Indicating No Inference path
# BDD Runs This Scenario With a Mock Where No Nodes Are registered
`
  @req_bootst_0002
  @spec_cynai_bootst_bootstrapsource
  Scenario: Orchestrator remains not ready when no inference path is available
    Given no local inference (Ollama) is running
    And no external provider key is configured
    When the orchestrator starts
    Then the orchestrator does not enter ready state
    And the orchestrator reports that no inference path is available
