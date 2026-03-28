package gateway

import (
	"encoding/json"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

func TestProcessChatSSEDataLine_DoneMarker(t *testing.T) {
	done, err := processChatSSEDataLine("[DONE]", "", nil, nil, nil, nil)
	if !done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
}

func TestProcessChatSSEDataLine_ThinkingToolHeartbeatIteration(t *testing.T) {
	var thinking, toolN, toolA string
	var hbE int
	var hbS string
	var iter int
	extra := &StreamExtra{
		OnThinking:       func(s string) { thinking = s },
		OnToolCall:       func(n, a string) { toolN, toolA = n, a },
		OnHeartbeat:      func(e int, s string) { hbE, hbS = e, s },
		OnIterationStart: func(i int) { iter = i },
	}
	raw, _ := json.Marshal(map[string]string{"content": "reason"})
	done, err := processChatSSEDataLine(string(raw), userapi.SSEEventThinkingDelta, nil, nil, nil, extra)
	if done || err != nil || thinking != "reason" {
		t.Fatalf("thinking: done=%v err=%v thinking=%q", done, err, thinking)
	}
	raw2, _ := json.Marshal(map[string]string{"name": "grep", "arguments": "{}"})
	done, err = processChatSSEDataLine(string(raw2), userapi.SSEEventToolCall, nil, nil, nil, extra)
	if done || err != nil || toolN != "grep" || toolA != "{}" {
		t.Fatalf("tool: %q %q err=%v", toolN, toolA, err)
	}
	raw3, _ := json.Marshal(map[string]any{"elapsed_s": 3, "status": "wait"})
	done, err = processChatSSEDataLine(string(raw3), userapi.SSEEventHeartbeat, nil, nil, nil, extra)
	if done || err != nil || hbE != 3 || hbS != "wait" {
		t.Fatalf("heartbeat: %d %q err=%v", hbE, hbS, err)
	}
	raw4, _ := json.Marshal(map[string]int{"iteration": 2})
	done, err = processChatSSEDataLine(string(raw4), userapi.SSEEventIterationStart, nil, nil, nil, extra)
	if done || err != nil || iter != 2 {
		t.Fatalf("iteration: %d err=%v", iter, err)
	}
}

func TestProcessChatSSEDataLine_StructuredWithoutExtra(t *testing.T) {
	raw, _ := json.Marshal(map[string]string{"content": "x"})
	done, err := processChatSSEDataLine(string(raw), userapi.SSEEventThinkingDelta, nil, nil, nil, nil)
	if done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
}
