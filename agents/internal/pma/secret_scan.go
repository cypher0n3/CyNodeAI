// Package pma — opportunistic secret scan (REQ-PMAGNT-0125).
package pma

import (
	"regexp"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"
)

var (
	reOpenAIKey = regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)
	reBearer    = regexp.MustCompile(`(?i)Bearer\s+\S+`)
)

// redactKnownSecrets replaces obvious secret patterns in s. It returns the redacted string,
// whether any replacement occurred, and stable kind labels for overwrite metadata.
func redactKnownSecrets(s string) (out string, found bool, kinds []string) {
	secretutil.RunWithSecret(func() {
		out, found, kinds = redactKnownSecretsImpl(s)
	})
	return out, found, kinds
}

func redactKnownSecretsImpl(s string) (out string, found bool, kinds []string) {
	out = s
	seen := map[string]struct{}{}
	if loc := reOpenAIKey.FindStringIndex(out); loc != nil {
		out = reOpenAIKey.ReplaceAllString(out, "[REDACTED]")
		seen["openai_key"] = struct{}{}
		found = true
	}
	if loc := reBearer.FindStringIndex(out); loc != nil {
		out = reBearer.ReplaceAllString(out, "Bearer [REDACTED]")
		seen["bearer"] = struct{}{}
		found = true
	}
	for k := range seen {
		kinds = append(kinds, k)
	}
	return out, found, kinds
}

// detectSecrets reports whether s still contains secret-like material after a no-op redact
// (used when comparing raw vs redacted accumulators).
func detectSecrets(s string) bool {
	_, found, _ := redactKnownSecrets(s)
	return found
}

func mergeKinds(dst, add []string) []string {
	seen := map[string]struct{}{}
	for _, k := range dst {
		seen[k] = struct{}{}
	}
	for _, k := range add {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			dst = append(dst, k)
		}
	}
	return dst
}

func kindsFromBuffer(s string) []string {
	_, _, k := redactKnownSecrets(s)
	return k
}

// secretKindsFromBuffers merges kind labels from visible, thinking, and tool-call buffers.
func secretKindsFromBuffers(vis, think, tool string) []string {
	k := mergeKinds(kindsFromBuffer(vis), kindsFromBuffer(think))
	return mergeKinds(k, kindsFromBuffer(tool))
}

func redactStreamEmitted(em streamEmitted) streamEmitted {
	t := em.Text
	switch em.Kind {
	case streamEmitDelta, streamEmitThinking, streamEmitToolCall:
		r, _, _ := redactKnownSecrets(t)
		em.Text = r
	default:
	}
	return em
}

// joinEmittedVisible reconstructs iteration-visible text from classified emissions (delta only).
func joinEmittedVisible(emissions []streamEmitted) string {
	var b strings.Builder
	for _, em := range emissions {
		if em.Kind == streamEmitDelta {
			b.WriteString(em.Text)
		}
	}
	return b.String()
}

// joinEmittedThinking aggregates thinking segments for an iteration.
func joinEmittedThinking(emissions []streamEmitted) string {
	var b strings.Builder
	for _, em := range emissions {
		if em.Kind == streamEmitThinking {
			b.WriteString(em.Text)
		}
	}
	return b.String()
}

// joinEmittedToolCalls aggregates tool-call argument text for an iteration.
func joinEmittedToolCalls(emissions []streamEmitted) string {
	var b strings.Builder
	for _, em := range emissions {
		if em.Kind == streamEmitToolCall {
			b.WriteString(em.Text)
		}
	}
	return b.String()
}
