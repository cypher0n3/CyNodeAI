package tui

import (
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
)

func (m *Model) appendTranscriptUser(content string) {
	m.Transcript = append(m.Transcript, TranscriptTurn{
		Role:      RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	})
}

func (m *Model) seedTranscriptAssistantInFlight() {
	m.Transcript = append(m.Transcript, TranscriptTurn{
		Role:           RoleAssistant,
		InFlight:       true,
		CreatedAt:      time.Now(),
		StreamingState: StreamingState{Phase: StreamingPhaseWorking},
	})
}

// appendTranscriptThinking appends to the in-flight assistant turn; thinking bytes are handled via secure buffer rules.
func (m *Model) appendTranscriptThinking(chunk string) {
	if chunk == "" || len(m.Transcript) == 0 {
		return
	}
	t := &m.Transcript[len(m.Transcript)-1]
	if t.Role != RoleAssistant || !t.InFlight {
		return
	}
	secretutil.RunWithSecret(func() {
		mergeThinkingPart(t, chunk)
	})
}

func mergeThinkingPart(t *TranscriptTurn, chunk string) {
	for i := range t.Parts {
		if t.Parts[i].Kind == PartKindThinking {
			t.Parts[i].Text += chunk
			return
		}
	}
	t.Parts = append(t.Parts, TranscriptPart{
		Kind:            PartKindThinking,
		Text:            chunk,
		HiddenByDefault: true,
		Collapsed:       true,
	})
}

func (m *Model) appendTranscriptToolCall(name, args string) {
	if len(m.Transcript) == 0 {
		return
	}
	t := &m.Transcript[len(m.Transcript)-1]
	if t.Role != RoleAssistant || !t.InFlight {
		return
	}
	secretutil.RunWithSecret(func() {
		meta := map[string]any{"name": name}
		t.Parts = append(t.Parts, TranscriptPart{
			Kind:            PartKindToolCall,
			Text:            args,
			Meta:            meta,
			HiddenByDefault: true,
			Collapsed:       true,
		})
	})
}

func (m *Model) syncInFlightTranscriptVisible() {
	if len(m.Transcript) == 0 {
		return
	}
	t := &m.Transcript[len(m.Transcript)-1]
	if t.Role != RoleAssistant || !t.InFlight {
		return
	}
	t.Content = m.streamBuf.String()
}
