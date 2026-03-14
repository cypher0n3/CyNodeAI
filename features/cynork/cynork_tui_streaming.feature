@suite_cynork
Feature: cynork TUI streaming behavior

  As a user of the cynork TUI
  I want real token-by-token streaming with correct cancellation and fallback handling
  So that in-flight responses are always coherent and never duplicate content

Background:
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
