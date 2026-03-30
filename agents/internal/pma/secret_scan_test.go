package pma

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecretScan_RedactsOpenAIKey(t *testing.T) {
	raw := "prefix sk-12345678901234567890123456789012 suffix"
	out, found, kinds := redactKnownSecrets(raw)
	if !found || !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("got %q found=%v kinds=%v", out, found, kinds)
	}
	if len(kinds) == 0 || kinds[0] != "openai_key" {
		t.Fatalf("kinds: %v", kinds)
	}
}

func TestSecretScan_RedactsBearer(t *testing.T) {
	raw := `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`
	out, found, kinds := redactKnownSecrets(raw)
	if !found || !strings.Contains(out, "Bearer [REDACTED]") {
		t.Fatalf("got %q found=%v", out, found)
	}
	if len(kinds) == 0 || kinds[0] != "bearer" {
		t.Fatalf("kinds: %v", kinds)
	}
}

func TestMergeKinds_Dedup(t *testing.T) {
	a := []string{"openai_key"}
	b := []string{"openai_key", "bearer"}
	got := mergeKinds(a, b)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestJoinEmitted_AllKinds(t *testing.T) {
	em := []streamEmitted{
		{Kind: streamEmitDelta, Text: "v"},
		{Kind: streamEmitThinking, Text: "th"},
		{Kind: streamEmitToolCall, Text: "tool"},
	}
	if joinEmittedVisible(em) != "v" {
		t.Fatal(joinEmittedVisible(em))
	}
	if joinEmittedThinking(em) != "th" {
		t.Fatal(joinEmittedThinking(em))
	}
	if joinEmittedToolCalls(em) != "tool" {
		t.Fatal(joinEmittedToolCalls(em))
	}
}

func TestOverwriteNDJSON_TurnScopeOmitsIteration(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	enc.SetEscapeHTML(false)
	if err := encodeOverwriteNDJSON(enc, rec, 9, "all", "turn", "agent_correction", nil); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(rec.Body.String())
	if strings.Contains(line, `"iteration"`) {
		t.Fatalf("turn scope should omit iteration: %s", line)
	}
}

func TestOverwriteNDJSON_EncodeShape(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := json.NewEncoder(rec)
	enc.SetEscapeHTML(false)
	iter := 3
	if err := encodeOverwriteNDJSON(enc, rec, iter, "fixed", "iteration", "secret_redaction", []string{"openai_key"}); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(rec.Body.String())
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &outer); err != nil {
		t.Fatal(err)
	}
	raw, ok := outer["overwrite"]
	if !ok {
		t.Fatalf("missing overwrite: %s", line)
	}
	var inner struct {
		Content   string   `json:"content"`
		Reason    string   `json:"reason"`
		Scope     string   `json:"scope"`
		Iteration *int     `json:"iteration"`
		Kinds     []string `json:"kinds"`
	}
	if err := json.Unmarshal(raw, &inner); err != nil {
		t.Fatal(err)
	}
	if inner.Content != "fixed" || inner.Reason != "secret_redaction" || inner.Scope != "iteration" {
		t.Fatalf("payload: %+v", inner)
	}
	if inner.Iteration == nil || *inner.Iteration != iter {
		t.Fatalf("iteration: %+v", inner.Iteration)
	}
}
