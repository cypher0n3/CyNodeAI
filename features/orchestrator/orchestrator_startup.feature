@suite_orchestrator
Feature: Orchestrator Startup

  As an operator
  I want the orchestrator to remain running but not ready when no inference path is available
  So that I can fix configuration (settings, credentials, and policy) before accepting work

# "Inference Path" Here Means at Least One Dispatchable Inference-Capable Target
# - a Dispatchable (Registered, Config-Acked) Local Inference Worker (Ollama or Similar), Or
# - an External Provider Path via API Egress When Configured and Allowed
#
# When Local Inference Exists, the Orchestrator is Expected to Select a Default Project Manager Model and Ensure It is Ready
# Using a VRAM-Based Sliding-Scale Policy (8 GB: `Qwen3.5` 9B Default; 16 GB: `Qwen3.5` 9B / `Qwen2.5` 14B; 24 GB: `Qwen3.5` 35B / `Qwen2.5` 32B)
# And Prefer the Orchestrator Host by Default When a Dispatchable Worker Exists There
# Operators Can Seed Selection Policy Values And/or Pin a Local Model Name via Orchestrator Bootstrap and Update Them Later via the CLI
# Or Admin Web Console System Settings UI (`qwen3.5:0.8b` is the Smallest Supported Fallback for Limited Systems)
#
# When No Inference Target is Available, Readyz Returns 503 and a Message Indicating No Inference Path
# BDD Runs This Scenario With a Mock Where No Nodes Are Registered

@req_bootst_0002
@spec_cynai_bootst_bootstrapsource
Scenario: Orchestrator remains not ready when no inference path is available
  Given no local inference (Ollama) is running
  And no external provider key is configured
  When the orchestrator starts
  Then the orchestrator does not enter ready state
  And the orchestrator reports that no inference path is available

@req_orches_0150
@req_models_0008
@spec_cynai_bootst_bootstrapsource
Scenario: Orchestrator becomes ready when inference path is available
  Given a running PostgreSQL database
  And the orchestrator API is running
  When a registered node "ready-node-01" is active with worker_api config and I request the readyz endpoint
  Then the orchestrator enters ready state
