@suite_cynork
Feature: cynork TUI streaming behavior

  As a user of the cynork TUI
  I want real token-by-token streaming with correct cancellation and fallback handling
  So that in-flight responses are always coherent and never duplicate content

## Background

  Given a mock gateway is running
  And cynork is built
  And I am logged in with username "alice" and password "secret"

@req_client_0204
@req_usrgwy_0150
@spec_cynai_client_cynorkchat_tuilayout
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: User cancellation stops the active stream and reconciles the in-flight turn
  Given the TUI has sent a message and the gateway is streaming the assistant response token-by-token
  When I send Ctrl+C or otherwise cancel the active stream
  Then the TUI treats the stream as canceled
  And any already-received visible text is retained in the transcript
  And the in-flight turn is reconciled deterministically without duplicating content
  And the session remains active unless I explicitly exit

@req_client_0209
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI degrades to in-flight indicator when backend cannot stream visible-text deltas
  Given the selected backend path does not support true incremental visible-text streaming
  When I send a normal interactive chat turn from the TUI
  Then the TUI shows a degraded in-flight state indicator
  And when the final response arrives the TUI replaces that row with the final assistant turn
  And visible assistant text is not duplicated

@req_client_0215
@req_usrgwy_0151
@spec_cynai_usrgwy_openaichatapi_streamingredactionpipeline
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI replaces streamed text when a post-stream amendment event arrives
  Given the TUI has sent a message and the gateway is streaming the assistant response token-by-token
  And the assistant response contains a detected secret
  When the upstream stream terminates and the gateway emits a cynodeai.amendment event with redacted content
  Then the TUI replaces the accumulated visible text for the in-flight turn with the amended content
  And the final transcript row shows the redacted text without duplicated or stale content
  And the turn is finalized after the terminal DONE event

@req_client_0209
@req_usrgwy_0151
@spec_cynai_usrgwy_openaichatapi_streamingredactionpipeline
Scenario: TUI finalizes cleanly when no amendment event arrives
  Given the TUI has sent a message and the gateway is streaming the assistant response token-by-token
  And the assistant response does not contain any detected secrets
  When the upstream stream terminates without a cynodeai.amendment event
  Then the accumulated visible text is used as the final transcript content
  And the turn is finalized after the terminal DONE event

@req_client_0202
@spec_cynai_client_cynorktui_entrypoint
Scenario: TUI remains coherent for both completions and responses chat surfaces
  Given the gateway supports both POST "/v1/chat/completions" and POST "/v1/responses"
  When I use both completions and responses surfaces in the TUI within the same session
  Then thread state, transcript, and session behavior are coherent for both surfaces
  And the user-visible chat contract does not diverge between the two surfaces

@req_client_0216
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI stores thinking content during streaming and toggles display without re-fetching
  Given the TUI has sent a message and the gateway is streaming the assistant response
  And the stream includes cynodeai.thinking_delta events with reasoning content
  When the assistant turn completes
  Then the TUI has stored the full thinking content for that turn
  And the thinking content is hidden by default
  And when I enable "show thinking" the stored thinking content is displayed instantly without a server request

@req_client_0217
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI stores tool-call content during streaming and toggles display without re-fetching
  Given the TUI has sent a message and the gateway is streaming the assistant response
  And the stream includes cynodeai.tool_call events with tool invocation details
  When the assistant turn completes
  Then the TUI has stored the tool-call content for that turn
  And the tool-call content is hidden by default
  And when I enable "show tool output" the stored tool-call content is displayed instantly without a server request

@req_client_0218
@spec_cynai_client_cynorktui_generationstate
Scenario: Per-iteration overwrite replaces only the targeted iteration visible text
  Given the TUI has sent a message and the gateway is streaming the assistant response
  And the stream includes a cynodeai.iteration_start event for iteration 1
  And visible text "Hello world" has been streamed for iteration 1
  And the stream includes a cynodeai.iteration_start event for iteration 2
  And visible text "Next part" has been streamed for iteration 2
  When the gateway emits a cynodeai.amendment event with scope "iteration" targeting iteration 1 and content "Corrected text"
  Then the TUI replaces only iteration 1 visible text with "Corrected text"
  And iteration 2 visible text "Next part" remains unchanged

@req_client_0219
@spec_cynai_client_cynorktui_generationstate
Scenario: Per-turn overwrite replaces entire accumulated visible text
  Given the TUI has sent a message and the gateway is streaming the assistant response
  And visible text has been streamed across multiple iterations
  When the gateway emits a cynodeai.amendment event with scope "turn" and content "Full replacement text"
  Then the TUI replaces the entire accumulated visible text with "Full replacement text"
  And all previous iteration text is discarded

@req_client_0220
@spec_cynai_usrgwy_openaichatapi_streamingheartbeatfallback
@spec_cynai_client_cynorktui_generationstate
Scenario: TUI renders heartbeat events as progress indicator when streaming is unavailable
  Given the TUI has sent a message and the gateway cannot provide real token streaming
  When the gateway emits periodic cynodeai.heartbeat events with elapsed time
  Then the TUI renders a progress indicator showing the elapsed time
  And when the full response arrives as a single delta the TUI renders it as the final assistant content
  And the heartbeat indicator is removed
