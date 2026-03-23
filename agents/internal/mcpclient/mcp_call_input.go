package mcpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DecodeMCPCallInput parses JSON for mcp_call tool input: {"tool_name":"...","arguments":...}.
// Some LLMs emit arguments as a JSON string (e.g. "{\"task_id\":\"x\"}") instead of an object;
// json.Unmarshal into map[string]interface{} fails on that form.
func DecodeMCPCallInput(input string) (toolName string, arguments map[string]interface{}, err error) {
	var outer struct {
		ToolName  string          `json:"tool_name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &outer); err != nil {
		return "", nil, err
	}
	args, err := parseArgumentsRaw(outer.Arguments)
	if err != nil {
		return "", nil, err
	}
	return outer.ToolName, args, nil
}

func parseArgumentsRaw(raw json.RawMessage) (map[string]interface{}, error) {
	b := bytes.TrimSpace(raw)
	if len(b) == 0 {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err == nil {
		return m, nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object or a JSON string containing an object: %w", err)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("arguments string is not a JSON object: %w", err)
	}
	return m, nil
}
