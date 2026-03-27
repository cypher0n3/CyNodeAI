@suite_agents
Feature: PMA chat and context composition

  As an orchestrator
  I want cynode-pma to accept handoff requests and compose LLM context in a defined order
  So that chat completions use baseline, project, task, and user context correctly (REQ-PMAGNT-0108)

@req_pmagnt_0108
@spec_cynai_pmagnt_llmcontextcomposition
Scenario: Internal chat completion accepts handoff with messages only
  Given I have an internal chat completion request with messages only "Hello"
  And I have a mock inference server
  When I send the request to the PMA internal chat completion endpoint
  Then the response status is 200
  And the response content is non-empty

@req_pmagnt_0108
@spec_cynai_pmagnt_llmcontextcomposition
Scenario: Composed context order is baseline then project then task then additional
  Given I have an internal chat completion request with project_id "proj-1" and task_id "task-1" and additional_context "user extra"
  And I have a mock inference server that captures the prompt
  When I send the request to the PMA internal chat completion endpoint
  Then the captured prompt contains "## Project context"
  And the captured prompt contains "## Task context"
  And the captured prompt contains "## User additional context"
  And "## Project context" appears before "## Task context" in the captured prompt
  And "## Task context" appears before "## User additional context" in the captured prompt

@wip
@req_orches_0165
@spec_cynai_pmagnt_conversationhistory
@spec_cynai_pmagnt_chatsurfacemapping
Scenario: Responses-surface continuation preserves prior turns and keeps current user input distinct
  Given I have a normalized PMA handoff from POST "/v1/responses" with retained prior turns and current input "Continue the plan"
  And I have a mock inference server that captures the messages
  When I send the request to the PMA internal chat completion endpoint
  Then the captured messages include the retained prior user and assistant turns in order
  And the last captured user message is "Continue the plan"
  And the last captured user message is not folded into the system message

@wip
@req_pmagnt_0116
@spec_cynai_pmagnt_nodelocalinferenceenv
Scenario: PMA applies node-local backend env values to local inference requests
  Given cynode-pma is configured for node-local inference with orchestrator-directed backend env values derived from node capabilities and policy
  And the managed-service inference config includes backend env key "OLLAMA_NUM_CTX"
  And I have a mock local inference server that captures runner options
  When I send a PMA internal chat completion request
  Then the captured local inference request uses the effective context value from the managed-service backend env

@req_pmagnt_0117
@spec_cynai_pmagnt_thinkingcontentseparation
Scenario: PMA removes think tags from visible assistant text
  Given the PMA inference backend returns visible assistant text mixed with "<think>internal reasoning</think>"
  When I send the request to the PMA internal chat completion endpoint
  Then the visible assistant response does not include "<think>"
  And the visible assistant response does not include "internal reasoning"

@req_pmagnt_0118
@spec_cynai_pmagnt_streamingassistantoutput
Scenario: PMA streams visible assistant text incrementally without leaking hidden thinking
  Given the PMA inference backend supports incremental visible-text output
  And the backend also emits hidden thinking updates
  When I send an interactive PMA chat request on the standard streaming path
  Then PMA emits ordered incremental visible assistant text updates
  And PMA does not emit hidden thinking as visible text deltas
  And PMA finishes with a terminal completion event

@wip
@req_pmagnt_0120
@spec_cynai_pmagnt_streamingllmwrapper
Scenario: PMA streaming wrapper tees tokens to output stream and internal buffer
  Given the PMA is configured with a capable model and MCP gateway
  And the inference backend streams tokens incrementally
  When I send an interactive PMA chat request on the standard streaming path
  Then PMA emits NDJSON delta events in real time as tokens arrive from the backend
  And the langchaingo executor receives the complete buffered response after the LLM call completes
  And the stream includes an iteration_start event before each agent iteration

@req_pmagnt_0121
@spec_cynai_pmagnt_streamingtokenstatemachine
Scenario: PMA state machine classifies think tokens and tool-call tokens separately
  Given the PMA inference backend emits a response containing "<think>internal reasoning</think>" followed by visible text and "<tool_call>" markers
  When I send an interactive PMA chat request on the standard streaming path
  Then PMA emits thinking NDJSON events for the content between think tags
  And PMA emits tool_call NDJSON events for the content between tool-call markers
  And PMA emits delta NDJSON events only for visible text content
  And no think tags or tool-call markers appear in the delta events

@req_pmagnt_0122
@spec_cynai_pmagnt_pmastreamingndjsonformat
Scenario: PMA emits full thinking content as NDJSON thinking events
  Given the PMA inference backend emits a response containing "<think>step-by-step reasoning here</think>"
  When I send an interactive PMA chat request on the standard streaming path
  Then the NDJSON stream includes thinking events containing the full reasoning text
  And the thinking content is not suppressed or summarized

@wip
@req_pmagnt_0120
@spec_cynai_pmagnt_pmastreamingndjsonformat
Scenario: PMA emits iteration_start events between langchaingo iterations
  Given the PMA is configured with a capable model and MCP gateway
  And the langchaingo executor performs multiple iterations with tool calls
  When I send an interactive PMA chat request on the standard streaming path
  Then the NDJSON stream includes an iteration_start event with iteration number 1 before the first LLM call
  And the NDJSON stream includes an iteration_start event with iteration number 2 before the second LLM call
  And tool_progress events appear between the iterations

@req_pmagnt_0124
@spec_cynai_pmagnt_pmastreamingoverwrite
Scenario: PMA emits per-iteration overwrite when think-tag tokens leak to visible text
  Given the PMA inference backend streams tokens that include a partial think tag leaked before detection
  When I send an interactive PMA chat request on the standard streaming path
  Then PMA emits an overwrite NDJSON event with scope "iteration" and reason "think_tag_leaked"
  And the overwrite content does not include the leaked tag characters

@wip
@req_pmagnt_0124
@spec_cynai_pmagnt_pmastreamingoverwrite
Scenario: PMA emits per-turn overwrite on agent output correction
  Given the PMA langchaingo executor returns output that triggers the unexecuted-tool-call fallback
  When PMA falls back to direct inference and obtains corrected output
  Then PMA emits an overwrite NDJSON event with scope "turn" and reason "agent_correction"
  And the overwrite content contains the corrected direct-inference response
