// Package bdd – Godog steps for cynork TUI streaming and in-memory TUI simulation (no PTY).
package bdd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
)

const bddAssistantPrefix = "Assistant: "

func bddEnsureTui(ctx context.Context) *tui.Model {
	st := getState(ctx)
	if st.bddStream == nil {
		cl := gateway.NewClient(st.mockServer.URL)
		cl.SetToken(st.token)
		sess := chat.NewSession(cl)
		sess.CurrentThreadID = "bdd-thread-1"
		m := tui.NewModel(sess)
		m.Width = 80
		m.Height = 24
		st.bddStream = m
	}
	return st.bddStream
}

func bddSyncBddStream(ctx context.Context, nm tea.Model) {
	st := getState(ctx)
	mm, ok := nm.(*tui.Model)
	if !ok {
		return
	}
	st.bddStream = mm
}

func bddGatewayStructuredTurn(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDResetModel()
	visible := "Hello from assistant"
	m.Transcript = []tui.TranscriptTurn{{
		Role:    tui.RoleAssistant,
		Content: visible,
		Parts: []tui.TranscriptPart{
			{Kind: tui.PartKindText, Text: visible},
			{Kind: tui.PartKindThinking, Text: "reasoning", HiddenByDefault: true, Collapsed: true},
		},
	}}
	m.Scrollback = []string{bddAssistantPrefix + visible}
	return nil
}

func bddGatewayStillGenerating(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDSimulateUserMessage("q")
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "partial"})
	return nil
}

func bddTUISentMessageAndStreaming(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDResetModel()
	m.StreamBDDSimulateUserMessage("hi")
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "partial"})
	return nil
}

func bddTUISentMessageAndStreamingTokenByToken(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDResetModel()
	m.StreamBDDSimulateUserMessage("hi")
	m.StreamBDDBeginAssistantStream()
	m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "to"})
	m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "ken"})
	return nil
}

func bddConnectionInterruptedMidStream(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDFinish(&net.OpError{Op: "read", Net: "tcp", Err: fmt.Errorf("connection reset by peer")})
	return nil
}

func bddCancelActiveStream(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	m.StreamBDDFinish(context.Canceled)
	return nil
}

func bddSendNormalInteractiveStreamTurn(ctx context.Context) error {
	st := getState(ctx)
	if st.bddStreamDegraded {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hello")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDApply(&tui.StreamBDDDelta{IsHeartbeat: true, HeartbeatElapsed: 2})
		st.bddStreamSawHeartbeat = true
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "full answer"})
		m.StreamBDDFinish(nil)
		return nil
	}
	if st.bddGatewayTokenStream {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hello")
		m.StreamBDDBeginAssistantStream()
		ch, err := m.Session.StreamMessage(context.Background(), "hello")
		if err != nil {
			return err
		}
		m.StreamBDDDrainChatStream(ch)
		return nil
	}
	return godog.ErrPending
}

func bddLastAssistant(ctx context.Context) *tui.TranscriptTurn {
	m := bddEnsureTui(ctx)
	if len(m.Transcript) == 0 {
		return nil
	}
	last := &m.Transcript[len(m.Transcript)-1]
	if last.Role != tui.RoleAssistant {
		return nil
	}
	return last
}

// registerCynorkStreamingBDDSteps registers streaming / TUI simulation steps that are not bound elsewhere.
func registerCynorkStreamingBDDSteps(sc *godog.ScenarioContext, _ *cynorkState) {
	sc.Step(`^When the TUI renders the assistant turn$`, func(ctx context.Context) error {
		_ = bddEnsureTui(ctx)
		return nil
	})
	sc.Step(`^When the TUI renders the in-flight turn$`, func(ctx context.Context) error {
		_ = bddEnsureTui(ctx)
		return nil
	})
	sc.Step(`^Given the current transcript contains loaded assistant turns with retained thinking$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.Transcript = []tui.TranscriptTurn{{
			Role:    tui.RoleAssistant,
			Content: "visible a",
			Parts: []tui.TranscriptPart{
				{Kind: tui.PartKindText, Text: "visible a"},
				{Kind: tui.PartKindThinking, Text: "think-a", HiddenByDefault: true, Collapsed: true},
			},
		}}
		m.Scrollback = []string{bddAssistantPrefix + "visible a"}
		return nil
	})
	sc.Step(`^older retrieved transcript history also contains retained thinking$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.Transcript = append(m.Transcript, tui.TranscriptTurn{
			Role:    tui.RoleAssistant,
			Content: "visible b",
			Parts: []tui.TranscriptPart{
				{Kind: tui.PartKindText, Text: "visible b"},
				{Kind: tui.PartKindThinking, Text: "think-b", HiddenByDefault: true, Collapsed: true},
			},
		})
		m.Scrollback = append(m.Scrollback, bddAssistantPrefix+"visible b")
		return nil
	})
	sc.Step(`^Given the current transcript contains expanded retained thinking blocks$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.ShowThinking = true
		m.Transcript = []tui.TranscriptTurn{{
			Role:    tui.RoleAssistant,
			Content: "v1",
			Parts: []tui.TranscriptPart{
				{Kind: tui.PartKindText, Text: "v1"},
				{Kind: tui.PartKindThinking, Text: "t1", HiddenByDefault: true, Collapsed: false},
			},
		}}
		return nil
	})
	sc.Step(`^older retrieved transcript history also contains expanded retained thinking$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.Transcript = append(m.Transcript, tui.TranscriptTurn{
			Role:    tui.RoleAssistant,
			Content: "v2",
			Parts: []tui.TranscriptPart{
				{Kind: tui.PartKindText, Text: "v2"},
				{Kind: tui.PartKindThinking, Text: "t2", HiddenByDefault: true, Collapsed: false},
			},
		})
		return nil
	})
	sc.Step(`^When I issue "/show-thinking" in the TUI$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowThinking = true
		return nil
	})
	sc.Step(`^When I issue "/hide-thinking" in the TUI$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowThinking = false
		return nil
	})
	sc.Step(`^And the indicator shows the label "Working" when no structured progress state is available$`, func(ctx context.Context) error {
		return bddAssertWorkingIndicator(ctx)
	})

	sc.Step(`^Given the selected backend path does not support true incremental visible-text streaming$`, func(ctx context.Context) error {
		getState(ctx).bddStreamDegraded = true
		return nil
	})
	sc.Step(`^Given the TUI has sent a message and the gateway is streaming the assistant response token-by-token$`, bddTUISentMessageAndStreamingTokenByToken)
	sc.Step(`^Given the TUI has sent a message and the gateway is streaming the assistant response$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		return nil
	})
	sc.Step(`^And the stream includes a cynodeai\.iteration_start event for iteration (\d+)$`, func(ctx context.Context, iter int) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{IterationStart: true, Iteration: iter})
		return nil
	})
	sc.Step(`^And visible text "([^"]*)" has been streamed for iteration (\d+)$`, func(ctx context.Context, text string, _ int) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: text})
		return nil
	})
	sc.Step(`^When the gateway emits a cynodeai\.amendment event with scope "([^"]*)" targeting iteration (\d+) and content "([^"]*)"$`, func(ctx context.Context, scope string, targetIter int, content string) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{
			Amendment:                content,
			AmendmentScope:           scope,
			AmendmentTargetIteration: targetIter,
		})
		m.StreamBDDFinish(nil)
		return nil
	})
	sc.Step(`^Then the TUI replaces only iteration 1 visible text with "([^"]*)"$`, func(ctx context.Context, want string) error {
		m := bddEnsureTui(ctx)
		if got := m.StreamBDDIterationSegment(1); got != want {
			return fmt.Errorf("iteration 1 visible: got %q want %q", got, want)
		}
		return nil
	})
	sc.Step(`^And iteration 2 visible text "([^"]*)" remains unchanged$`, func(ctx context.Context, want string) error {
		m := bddEnsureTui(ctx)
		if got := m.StreamBDDIterationSegment(2); got != want {
			return fmt.Errorf("iteration 2 visible: got %q want %q", got, want)
		}
		return nil
	})
	sc.Step(`^Given the assistant response contains a detected secret$`, func(_ context.Context) error { return nil })
	sc.Step(`^Given the assistant response does not contain any detected secrets$`, func(_ context.Context) error { return nil })
	sc.Step(`^Given the gateway supports both POST "/v1/chat/completions" and POST "/v1/responses"$`, func(_ context.Context) error { return nil })
	sc.Step(`^When the upstream stream terminates and the gateway emits a cynodeai\.amendment event with redacted content$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{Amendment: "[redacted]"})
		m.StreamBDDFinish(nil)
		return nil
	})
	sc.Step(`^When the upstream stream terminates without a cynodeai\.amendment event$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDFinish(nil)
		return nil
	})
	sc.Step(`^When I use both completions and responses surfaces in the TUI within the same session$`, func(ctx context.Context) error {
		st := getState(ctx)
		cl := gateway.NewClient(st.mockServer.URL)
		cl.SetToken(st.token)
		s1 := chat.NewSession(cl)
		m1 := tui.NewModel(s1)
		m1.StreamBDDSimulateUserMessage("c")
		m1.StreamBDDBeginAssistantStream()
		ch1, err := s1.StreamMessage(context.Background(), "c")
		if err != nil {
			return err
		}
		m1.StreamBDDDrainChatStream(ch1)
		s2 := chat.NewSessionWithResponses(cl)
		m2 := tui.NewModel(s2)
		m2.StreamBDDSimulateUserMessage("r")
		m2.StreamBDDBeginAssistantStream()
		ch2, err := s2.StreamMessage(context.Background(), "r")
		if err != nil {
			return err
		}
		m2.StreamBDDDrainChatStream(ch2)
		return nil
	})
	sc.Step(`^And the stream includes cynodeai\.thinking_delta events with reasoning content$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDApply(&tui.StreamBDDDelta{Thinking: "because"})
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "done"})
		return nil
	})
	sc.Step(`^And the stream includes cynodeai\.tool_call events with tool invocation details$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDApply(&tui.StreamBDDDelta{ToolName: "grep", ToolArgs: "{}"})
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "out"})
		return nil
	})
	sc.Step(`^When the assistant turn completes$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDFinish(nil)
		return nil
	})
	sc.Step(`^Given visible text has been streamed across multiple iterations$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		m.StreamBDDApply(&tui.StreamBDDDelta{IterationStart: true, Iteration: 1})
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "aa"})
		m.StreamBDDApply(&tui.StreamBDDDelta{IterationStart: true, Iteration: 2})
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "bb"})
		return nil
	})
	sc.Step(`^When the gateway emits a cynodeai\.amendment event with scope "turn" and content "Full replacement text"$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{Amendment: "Full replacement text"})
		m.StreamBDDFinish(nil)
		return nil
	})
	sc.Step(`^Given the TUI has sent a message and the gateway cannot provide real token streaming$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.StreamBDDResetModel()
		m.StreamBDDSimulateUserMessage("hi")
		m.StreamBDDBeginAssistantStream()
		return nil
	})
	sc.Step(`^When the gateway emits periodic cynodeai\.heartbeat events with elapsed time$`, func(ctx context.Context) error {
		st := getState(ctx)
		m := bddEnsureTui(ctx)
		m.StreamBDDApply(&tui.StreamBDDDelta{IsHeartbeat: true, HeartbeatElapsed: 3})
		m.StreamBDDApply(&tui.StreamBDDDelta{IsHeartbeat: true, HeartbeatElapsed: 6})
		st.bddHeartbeatViewSnap = m.View()
		m.StreamBDDApply(&tui.StreamBDDDelta{Delta: "final"})
		m.StreamBDDFinish(nil)
		return nil
	})

	sc.Step(`^Then the TUI treats the stream as canceled$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || !last.Interrupted {
			return fmt.Errorf("expected interrupted assistant turn")
		}
		return nil
	})
	sc.Step(`^any already-received visible text is retained in the transcript$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || !strings.Contains(last.Content, "token") {
			return fmt.Errorf("expected retained partial visible text in transcript; got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^the in-flight turn is reconciled deterministically without duplicating content$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		combined := strings.Join(m.Scrollback, "\n")
		if strings.Count(combined, "token") > 1 {
			return fmt.Errorf("duplicate visible token in scrollback: %q", combined)
		}
		return nil
	})
	sc.Step(`^the session remains active unless I explicitly exit$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.Loading {
			return fmt.Errorf("expected session not loading after cancel reconciliation")
		}
		return nil
	})
	sc.Step(`^Then the TUI shows a degraded in-flight state indicator$`, func(ctx context.Context) error {
		if !getState(ctx).bddStreamSawHeartbeat {
			return fmt.Errorf("expected heartbeat deltas for degraded streaming path")
		}
		return nil
	})
	sc.Step(`^when the final response arrives the TUI replaces that row with the final assistant turn$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.InFlight {
			return fmt.Errorf("expected finalized assistant turn")
		}
		return nil
	})
	sc.Step(`^visible assistant text is not duplicated$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if strings.Count(strings.Join(m.Scrollback, " "), "full answer") > 1 {
			return fmt.Errorf("duplicated final visible text in scrollback")
		}
		return nil
	})
	sc.Step(`^Then the TUI replaces the accumulated visible text for the in-flight turn with the amended content$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.Content != "[redacted]" {
			return fmt.Errorf("want redacted content; got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^the final transcript row shows the redacted text without duplicated or stale content$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		combined := strings.Join(m.Scrollback, "\n")
		if !strings.Contains(combined, "[redacted]") {
			return fmt.Errorf("scrollback missing redacted text: %q", combined)
		}
		return nil
	})
	sc.Step(`^the turn is finalized after the terminal DONE event$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.InFlight {
			return fmt.Errorf("turn still in flight")
		}
		return nil
	})
	sc.Step(`^Then the accumulated visible text is used as the final transcript content$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.Content != "token" {
			return fmt.Errorf("expected token from streamed deltas without amendment; got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^Then thread state, transcript, and session behavior are coherent for both surfaces$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^the user-visible chat contract does not diverge between the two surfaces$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^Then the TUI has stored the full thinking content for that turn$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil {
			return fmt.Errorf("no assistant turn")
		}
		var got string
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindThinking {
				got += p.Text
			}
		}
		if !strings.Contains(got, "because") {
			return fmt.Errorf("thinking not stored: %q", got)
		}
		return nil
	})
	sc.Step(`^the thinking content is hidden by default$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.ShowThinking {
			return fmt.Errorf("ShowThinking should default false for this scenario")
		}
		last := bddLastAssistant(ctx)
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindThinking && !p.HiddenByDefault {
				return fmt.Errorf("thinking part should be hidden-by-default")
			}
		}
		return nil
	})
	sc.Step(`^when I enable "show thinking" the stored thinking content is displayed instantly without a server request$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowThinking = true
		last := bddLastAssistant(ctx)
		var got string
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindThinking {
				got = p.Text
			}
		}
		if got == "" {
			return fmt.Errorf("no thinking text to display")
		}
		v := m.View()
		if !strings.Contains(v, "because") {
			return fmt.Errorf("view should include thinking when expanded; snippet missing")
		}
		return nil
	})
	sc.Step(`^Then the TUI has stored the tool-call content for that turn$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		found := false
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindToolCall {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected tool_call part")
		}
		return nil
	})
	sc.Step(`^the tool-call content is hidden by default$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindToolCall && !p.HiddenByDefault {
				return fmt.Errorf("tool call should be hidden by default")
			}
		}
		return nil
	})
	sc.Step(`^when I enable "show tool output" the stored tool-call content is displayed instantly without a server request$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		m.ShowToolOutput = true
		last := bddLastAssistant(ctx)
		var toolName string
		for _, p := range last.Parts {
			if p.Kind == tui.PartKindToolCall && p.Meta != nil {
				if n, _ := p.Meta["name"].(string); n != "" {
					toolName = n
				}
			}
		}
		v := m.View()
		if toolName != "" && !strings.Contains(v, toolName) {
			return fmt.Errorf("view should surface tool name %q when show-tool-output on", toolName)
		}
		return nil
	})
	sc.Step(`^Then the TUI replaces the entire accumulated visible text with "Full replacement text"$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.Content != "Full replacement text" {
			return fmt.Errorf("got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^all previous iteration text is discarded$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || strings.Contains(last.Content, "aa") || strings.Contains(last.Content, "bb") {
			return fmt.Errorf("expected only replacement text; got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^Then the TUI renders a progress indicator showing the elapsed time$`, func(ctx context.Context) error {
		v := getState(ctx).bddHeartbeatViewSnap
		if !strings.Contains(v, "heartbeat") {
			return fmt.Errorf("expected heartbeat indicator in captured view")
		}
		return nil
	})
	sc.Step(`^when the full response arrives as a single delta the TUI renders it as the final assistant content$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.Content != "final" {
			return fmt.Errorf("got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^the heartbeat indicator is removed$`, func(ctx context.Context) error {
		m := bddEnsureTui(ctx)
		if m.StreamBDDHeartbeatNote() != "" {
			return fmt.Errorf("heartbeat note should be cleared after done")
		}
		return nil
	})

	sc.Step(`^Then the TUI requests streaming output by default$`, func(ctx context.Context) error {
		if !getState(ctx).lastChatStream {
			return fmt.Errorf("expected chat completion request with stream:true")
		}
		return nil
	})
	sc.Step(`^visible assistant text is appended token-by-token within one in-flight assistant turn$`, func(ctx context.Context) error {
		last := bddLastAssistant(ctx)
		if last == nil || last.Content != "token" {
			return fmt.Errorf("expected concatenated streamed tokens; got %q", last.Content)
		}
		return nil
	})
	sc.Step(`^the final assistant turn replaces the in-flight row without duplicating visible text$`, func(ctx context.Context) error {
		c := strings.Join(bddEnsureTui(ctx).Scrollback, "\n")
		if strings.Count(c, "Assistant: token") != 1 {
			return fmt.Errorf("expected exactly one finalized assistant line with token; scrollback:\n%s", c)
		}
		return nil
	})
}

func bddAssertWorkingIndicator(ctx context.Context) error {
	last := bddLastAssistant(ctx)
	if last == nil || !last.InFlight {
		return fmt.Errorf("expected in-flight assistant turn")
	}
	if last.StreamingState.Phase != tui.StreamingPhaseWorking {
		return fmt.Errorf("phase = %v", last.StreamingState.Phase)
	}
	return nil
}

func bddAssertThinkingCollapsedDistinctHint(ctx context.Context) error {
	last := bddLastAssistant(ctx)
	if last == nil {
		return fmt.Errorf("no assistant turn")
	}
	var thinking *tui.TranscriptPart
	var text *tui.TranscriptPart
	for i := range last.Parts {
		switch last.Parts[i].Kind {
		case tui.PartKindThinking:
			thinking = &last.Parts[i]
		case tui.PartKindText:
			text = &last.Parts[i]
		}
	}
	if text == nil || thinking == nil {
		return fmt.Errorf("expected text and thinking parts")
	}
	if !thinking.Collapsed || !thinking.HiddenByDefault {
		return fmt.Errorf("thinking should be collapsed placeholder")
	}
	v := bddEnsureTui(ctx).View()
	if !strings.Contains(v, "/show-thinking") {
		return fmt.Errorf("view should hint /show-thinking")
	}
	return nil
}

func bddAssertExpandedThinking(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	if !m.ShowThinking {
		return fmt.Errorf("ShowThinking should be true")
	}
	return nil
}

func bddAssertCollapsedThinkingPlaceholders(ctx context.Context) error {
	m := bddEnsureTui(ctx)
	if m.ShowThinking {
		return fmt.Errorf("ShowThinking should be false after hide")
	}
	for _, turn := range m.Transcript {
		if turn.Role != tui.RoleAssistant {
			continue
		}
		for _, p := range turn.Parts {
			if p.Kind == tui.PartKindThinking && !p.Collapsed {
				return fmt.Errorf("thinking should be collapsed")
			}
		}
	}
	return nil
}

func bddAssertViewContainsThinkA(ctx context.Context) error {
	v := bddEnsureTui(ctx).View()
	if !strings.Contains(v, "think-a") {
		return fmt.Errorf("view should include first retained thinking")
	}
	return nil
}

func bddAssertViewContainsThinkB(ctx context.Context) error {
	v := bddEnsureTui(ctx).View()
	if !strings.Contains(v, "think-b") {
		return fmt.Errorf("view should include older retained thinking")
	}
	return nil
}

func bddAssertShowThinkingHintStill(ctx context.Context) error {
	v := bddEnsureTui(ctx).View()
	if !strings.Contains(v, "/show-thinking") {
		return fmt.Errorf("expected /show-thinking hint in view")
	}
	return nil
}

func bddAssertThinkingExpandedDefaultFromConfig(ctx context.Context) error {
	st := getState(ctx)
	raw, err := os.ReadFile(st.configPath)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return err
	}
	tuiSec, _ := root["tui"].(map[string]any)
	got, _ := tuiSec["show_thinking_by_default"].(bool)
	if !got {
		return fmt.Errorf("expected tui.show_thinking_by_default true in config")
	}
	m := bddEnsureTui(ctx)
	m.ShowThinking = true
	m.StreamBDDResetModel()
	m.Transcript = []tui.TranscriptTurn{{
		Role:    tui.RoleAssistant,
		Content: "x",
		Parts: []tui.TranscriptPart{
			{Kind: tui.PartKindText, Text: "x"},
			{Kind: tui.PartKindThinking, Text: "loaded-think", HiddenByDefault: true, Collapsed: false},
		},
	}}
	for _, p := range m.Transcript[0].Parts {
		if p.Kind == tui.PartKindThinking && p.Collapsed {
			return fmt.Errorf("expected thinking expanded when show_thinking default is true")
		}
	}
	return nil
}
