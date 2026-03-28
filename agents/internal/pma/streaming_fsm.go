package pma

import (
	"strings"
)

// Stream tag delimiters for tool-call blocks (CYNAI.PMAGNT.StreamingTokenStateMachine).
// Think tags reuse xmlThinkOpen / xmlThinkClose from langchain.go (same package).
const (
	toolCallOpen  = "\u003ctool_call\u003e"
	toolCallClose = "\u003c/tool_call\u003e"
)

type streamEmitKind string

const (
	streamEmitDelta    streamEmitKind = "delta"
	streamEmitThinking streamEmitKind = "thinking"
	streamEmitToolCall streamEmitKind = "tool_call"
)

type streamEmitted struct {
	Kind streamEmitKind
	Text string
}

// streamingClassifier performs incremental classification of streamed model text into
// visible deltas, thinking segments, and tool-call segments per cynode_pma.md.
type streamingClassifier struct {
	pending string
	inThink bool
	inTool  bool
}

func newStreamingClassifier() *streamingClassifier {
	return &streamingClassifier{}
}

// newStreamingTokenFSM is an alias for tests that refer to the state machine by the older name.
func newStreamingTokenFSM() *streamingClassifier { return newStreamingClassifier() }

// Feed consumes one upstream chunk (may be empty). Order of returned segments matches stream order.
func (c *streamingClassifier) Feed(chunk string) []streamEmitted {
	if chunk != "" {
		c.pending += chunk
	}
	return c.drain(false)
}

// Flush emits any remaining buffered text at EOF.
func (c *streamingClassifier) Flush() []streamEmitted {
	return c.drain(true)
}

func (c *streamingClassifier) drain(eof bool) []streamEmitted {
	var out []streamEmitted
	for {
		switch {
		case c.inThink:
			em, stop := c.drainThinkBranch(eof)
			out = append(out, em...)
			if stop {
				return out
			}
		case c.inTool:
			em, stop := c.drainToolBranch(eof)
			out = append(out, em...)
			if stop {
				return out
			}
		default:
			em, stop := c.drainNeutralBranch(eof)
			out = append(out, em...)
			if stop {
				return out
			}
		}
	}
}

// drainThinkBranch handles c.inThink. stop is true when more input is needed before emitting.
func (c *streamingClassifier) drainThinkBranch(eof bool) (em []streamEmitted, stop bool) {
	idx := strings.Index(c.pending, xmlThinkClose)
	if idx == -1 {
		if !eof {
			return nil, true
		}
		if c.pending != "" {
			em = append(em, streamEmitted{Kind: streamEmitThinking, Text: c.pending})
			c.pending = ""
		}
		c.inThink = false
		return em, false
	}
	inner := c.pending[:idx]
	if inner != "" {
		em = append(em, streamEmitted{Kind: streamEmitThinking, Text: inner})
	}
	c.pending = c.pending[idx+len(xmlThinkClose):]
	c.inThink = false
	return em, false
}

// drainToolBranch handles c.inTool. stop is true when more input is needed before emitting.
func (c *streamingClassifier) drainToolBranch(eof bool) (em []streamEmitted, stop bool) {
	idx := strings.Index(c.pending, toolCallClose)
	if idx == -1 {
		if !eof {
			return nil, true
		}
		if c.pending != "" {
			em = append(em, streamEmitted{Kind: streamEmitToolCall, Text: c.pending})
			c.pending = ""
		}
		c.inTool = false
		return em, false
	}
	inner := c.pending[:idx]
	em = append(em, streamEmitted{Kind: streamEmitToolCall, Text: inner})
	c.pending = c.pending[idx+len(toolCallClose):]
	c.inTool = false
	return em, false
}

// drainNeutralBranch handles visible/tool/think tag boundaries outside think/tool regions.
func (c *streamingClassifier) drainNeutralBranch(eof bool) (em []streamEmitted, stop bool) {
	// Stray closes (e.g. chunk boundary) must not become visible deltas.
	if strings.HasPrefix(c.pending, xmlThinkClose) {
		c.pending = c.pending[len(xmlThinkClose):]
		return nil, false
	}
	if strings.HasPrefix(c.pending, toolCallClose) {
		c.pending = c.pending[len(toolCallClose):]
		return nil, false
	}
	iThink := strings.Index(c.pending, xmlThinkOpen)
	iTool := strings.Index(c.pending, toolCallOpen)
	next := -1
	which := 0 // 1=think 2=tool
	if iThink >= 0 && (next < 0 || iThink < next) {
		next = iThink
		which = 1
	}
	if iTool >= 0 && (next < 0 || iTool < next) {
		next = iTool
		which = 2
	}
	if next < 0 {
		return c.flushPendingAsVisibleDelta(eof)
	}
	if next > 0 {
		em = append(em, streamEmitted{Kind: streamEmitDelta, Text: c.pending[:next]})
		c.pending = c.pending[next:]
	}
	switch which {
	case 1:
		c.pending = c.pending[len(xmlThinkOpen):]
		c.inThink = true
	case 2:
		c.pending = c.pending[len(toolCallOpen):]
		c.inTool = true
	}
	return em, false
}

func (c *streamingClassifier) flushPendingAsVisibleDelta(eof bool) (em []streamEmitted, stop bool) {
	keep := trailingIncompleteTagPrefix(c.pending)
	if keep > 0 && !eof {
		if emitLen := len(c.pending) - keep; emitLen > 0 {
			em = append(em, streamEmitted{Kind: streamEmitDelta, Text: c.pending[:emitLen]})
			c.pending = c.pending[emitLen:]
		}
		return em, true
	}
	if c.pending != "" {
		em = append(em, streamEmitted{Kind: streamEmitDelta, Text: c.pending})
		c.pending = ""
	}
	return em, true
}

func trailingIncompleteTagPrefix(s string) int {
	if s == "" {
		return 0
	}
	best := 0
	for _, tag := range []string{xmlThinkOpen, toolCallOpen} {
		maxN := min(len(s), len(tag)-1)
		for n := maxN; n >= 1; n-- {
			suf := s[len(s)-n:]
			if strings.HasPrefix(tag, suf) && n < len(tag) {
				if n > best {
					best = n
				}
				break
			}
		}
	}
	return best
}

// iterationOverwriteReplace returns full with [start:end) replaced by replacement.
func iterationOverwriteReplace(full string, start, end int, replacement string) string {
	if start < 0 || end < start || end > len(full) {
		return full
	}
	return full[:start] + replacement + full[end:]
}

// turnOverwriteReplace returns the corrected visible stream for turn scope.
func turnOverwriteReplace(visible, correction string) string {
	if strings.TrimSpace(correction) != "" {
		return correction
	}
	return visible
}
