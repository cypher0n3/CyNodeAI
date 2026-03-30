// Package pma — NDJSON overwrite events (REQ-PMAGNT-0124).
package pma

import (
	"encoding/json"
	"net/http"
)

// ndjsonOverwriteWire matches CYNAI.PMAGNT.PMAStreamingNDJSONFormat / cynode_pma.md.
type ndjsonOverwriteWire struct {
	Content   string   `json:"content"`
	Reason    string   `json:"reason"`
	Scope     string   `json:"scope"`
	Iteration *int     `json:"iteration,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
}

func encodeOverwriteNDJSON(enc *json.Encoder, w http.ResponseWriter, iter int, content, scope, reason string, kinds []string) error {
	ow := ndjsonOverwriteWire{
		Content: content,
		Reason:  reason,
		Scope:   scope,
		Kinds:   kinds,
	}
	if scope == "iteration" {
		i := iter
		ow.Iteration = &i
	}
	if err := enc.Encode(map[string]ndjsonOverwriteWire{"overwrite": ow}); err != nil {
		return err
	}
	flushResponseWriter(w)
	return nil
}
