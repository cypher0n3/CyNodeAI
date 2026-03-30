package tui

import (
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

// StreamBDDDelta is a test/BDD-friendly view of one streaming event applied directly
// to the model (no live tea.Cmd loop, no stream channel scheduling).
type StreamBDDDelta struct {
	Delta                    string
	Amendment                string
	AmendmentScope           string // "turn" (default) or "iteration"
	AmendmentTargetIteration int
	Thinking                 string
	ToolName                 string
	ToolArgs                 string
	IsHeartbeat              bool
	HeartbeatElapsed         int
	HeartbeatStatus          string
	IterationStart           bool
	Iteration                int
}

// StreamBDDResetModel clears scrollback, transcript, and streaming scratch state for an isolated BDD scenario.
func (m *Model) StreamBDDResetModel() {
	m.StreamBDDResetStreamingState()
	m.Transcript = nil
	m.Scrollback = nil
	m.Err = ""
}

// StreamBDDResetStreamingState clears in-flight streaming scratch fields without
// touching transcript or scrollback.
func (m *Model) StreamBDDResetStreamingState() {
	m.streamCh = nil
	m.streamCancel = nil
	m.streamBuf.Reset()
	m.clearStreamIterationState()
	m.streamHeartbeatNote = ""
	m.Loading = false
	m.clearConnectionRecovery()
}

// StreamBDDSimulateUserMessage mirrors handleEnterKey's user line for BDD (scrollback + transcript).
func (m *Model) StreamBDDSimulateUserMessage(line string) {
	m.appendTranscriptUser(line)
	m.Scrollback = append(m.Scrollback, "You: "+line)
}

// StreamBDDBeginAssistantStream seeds an in-flight assistant turn like streamCmd (no gateway I/O).
func (m *Model) StreamBDDBeginAssistantStream() {
	m.Loading = true
	m.streamBuf.Reset()
	m.clearStreamIterationState()
	m.streamHeartbeatNote = ""
	m.streamCh = nil
	m.Scrollback = append(m.Scrollback, assistantPrefix)
	m.seedTranscriptAssistantInFlight()
}

// StreamBDDApply applies one streaming delta without scheduling further reads from streamCh.
func (m *Model) StreamBDDApply(d *StreamBDDDelta) {
	if d == nil {
		return
	}
	saved := m.streamCh
	m.streamCh = nil
	defer func() { m.streamCh = saved }()
	msg := streamDeltaMsg{
		delta:                    d.Delta,
		amendment:                d.Amendment,
		amendmentScope:           d.AmendmentScope,
		amendmentTargetIteration: d.AmendmentTargetIteration,
		thinking:                 d.Thinking,
		toolName:                 d.ToolName,
		toolArgs:                 d.ToolArgs,
		isHeartbeat:              d.IsHeartbeat,
		hbElapsed:                d.HeartbeatElapsed,
		hbStatus:                 d.HeartbeatStatus,
		iterationStart:           d.IterationStart,
		iteration:                d.Iteration,
	}
	m.applyStreamDelta(&msg)
}

// StreamBDDFinish ends the active streaming turn (terminal DONE or error).
func (m *Model) StreamBDDFinish(err error) {
	done := streamDoneMsg{err: err}
	m.applyStreamDone(done)
	_ = m.maybeScheduleStreamRecovery(done)
}

// StreamBDDDrainChatStream drains a chat.StreamMessage channel into this model using the
// same delta routing as the live TUI (readNextDelta + applyStreamDelta / applyStreamDone).
// StreamBDDConnectionRecoveryState exposes connection recovery state for BDD assertions.
func (m *Model) StreamBDDConnectionRecoveryState() ConnectionState {
	return m.connectionRecoveryState
}

// StreamBDDStreamRecoveryAttempt exposes the current stream recovery attempt counter for BDD.
func (m *Model) StreamBDDStreamRecoveryAttempt() int {
	return m.streamRecoveryAttempt
}

// StreamBDDHeartbeatNote returns the ephemeral heartbeat status line (empty after stream completes).
func (m *Model) StreamBDDHeartbeatNote() string {
	return m.streamHeartbeatNote
}

// StreamBDDIterationSegment returns visible text accumulated for one iteration index during streaming (tests/BDD).
func (m *Model) StreamBDDIterationSegment(iter int) string {
	if m.streamIterSegs == nil {
		return ""
	}
	return m.streamIterSegs[iter]
}

func (m *Model) StreamBDDDrainChatStream(ch <-chan chat.ChatStreamDelta) {
	for {
		switch msg := readNextDelta(ch).(type) {
		case streamDoneMsg:
			m.applyStreamDone(msg)
			_ = m.maybeScheduleStreamRecovery(msg)
			return
		case streamDeltaMsg:
			m.streamCh = nil
			m.applyStreamDelta(&msg)
		case streamPollMsg:
			continue
		default:
			return
		}
	}
}
