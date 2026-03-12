// Package sba provides agent tools that wrap step primitives for the langchaingo agent loop.
package sba

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/tmc/langchaingo/tools"
)

const errMsgMaxOutputBytes = "output exceeded max_output_bytes"

// ToolEnv holds workspace and constraints for SBA tools.
type ToolEnv struct {
	Workspace       string
	MaxOutputBytes  int
	ConstraintError *string // when set, tools return constraint_violation
}

// SBATool wraps a step primitive as a langchaingo tool.
type SBATool struct {
	name string
	desc string
	call func(ctx context.Context, raw string, env *ToolEnv) (out string, errMsg string, constraintViolation bool)
}

func (t *SBATool) Name() string        { return t.name }
func (t *SBATool) Description() string { return t.desc }
func (t *SBATool) Call(ctx context.Context, input string) (string, error) {
	env := toolEnvFromContext(ctx)
	if env == nil {
		env = &ToolEnv{Workspace: "/workspace", MaxOutputBytes: 1024 * 1024}
	}
	if env.ConstraintError != nil {
		return "", &constraintViolationError{msg: *env.ConstraintError}
	}
	out, errMsg, constraintViolation := t.call(ctx, strings.TrimSpace(input), env)
	if constraintViolation {
		return "", &constraintViolationError{msg: errMsg}
	}
	if errMsg != "" {
		return "error: " + errMsg, nil
	}
	return out, nil
}

type constraintViolationError struct{ msg string }

func (e *constraintViolationError) Error() string { return e.msg }

// IsConstraintViolation reports whether err is a constraint violation (max_output_bytes etc).
func IsConstraintViolation(err error) bool {
	_, ok := err.(*constraintViolationError)
	return ok
}

type toolEnvKey struct{}

// ContextWithToolEnv attaches ToolEnv to ctx for tool invocations.
func ContextWithToolEnv(ctx context.Context, env *ToolEnv) context.Context {
	return context.WithValue(ctx, toolEnvKey{}, env)
}

func toolEnvFromContext(ctx context.Context) *ToolEnv {
	if v := ctx.Value(toolEnvKey{}); v != nil {
		return v.(*ToolEnv)
	}
	return nil
}

// NewLocalTools returns langchaingo tools for run_command, write_file, read_file, apply_unified_diff, list_tree, search_files.
// Workspace and MaxOutputBytes are taken from context (ContextWithToolEnv) at call time.
func NewLocalTools() []tools.Tool {
	return []tools.Tool{
		runCommandTool(),
		writeFileTool(),
		readFileTool(),
		applyUnifiedDiffTool(),
		listTreeTool(),
		searchFilesTool(),
	}
}

func runCommandTool() *SBATool {
	return &SBATool{
		name: stepTypeRunCommand,
		desc: "Run a shell command. Input JSON: {\"argv\": [\"cmd\", \"arg1\", ...], \"cwd\": \"optional/subdir\"}. cwd is relative to workspace.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			var args runCommandArgs
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				return "", "invalid JSON: " + err.Error(), false
			}
			if len(args.Argv) == 0 {
				return "", "argv must be non-empty", false
			}
			b, _ := json.Marshal(args)
			sr := runCommandStep(ctx, 0, b, te.MaxOutputBytes, te.Workspace)
			if sr.Status == statusFailed || sr.Status == statusTimeout {
				return sr.Output, sr.Error, false
			}
			if strings.HasSuffix(sr.Output, "\n...[truncated]") {
				return "", errMsgMaxOutputBytes, true
			}
			return sr.Output, "", false
		},
	}
}

func writeFileTool() *SBATool {
	return &SBATool{
		name: "write_file",
		desc: "Write content to a file under workspace. Input JSON: {\"path\": \"rel/path.txt\", \"content\": \"text\"}.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			var args writeFileArgs
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				return "", "invalid JSON: " + err.Error(), false
			}
			b, _ := json.Marshal(args)
			sr := writeFileStep(0, b, te.Workspace)
			if sr.Status != statusSuccess {
				return "", sr.Error, false
			}
			return "ok", "", false
		},
	}
}

// runCappedStepTool runs a step that may truncate output and returns constraint violation on truncation.
func runCappedStepTool(raw string, te *ToolEnv, step func(int, json.RawMessage, int, string) sbajob.StepResult) (out, errMsg string, constraintViolation bool) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return "", "invalid JSON: " + err.Error(), false
	}
	b, _ := json.Marshal(m)
	sr := step(0, b, te.MaxOutputBytes, te.Workspace)
	if sr.Status != statusSuccess {
		return "", sr.Error, false
	}
	if strings.HasSuffix(sr.Output, "\n...[truncated]") {
		return "", errMsgMaxOutputBytes, true
	}
	return sr.Output, "", false
}

//nolint:dupl // same pattern as searchFilesTool, different step
func readFileTool() *SBATool {
	return &SBATool{
		name: "read_file",
		desc: "Read a file under workspace. Input JSON: {\"path\": \"rel/path.txt\"}.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			return runCappedStepTool(raw, te, readFileStep)
		},
	}
}

func applyUnifiedDiffTool() *SBATool {
	return &SBATool{
		name: "apply_unified_diff",
		desc: "Apply a unified diff patch under workspace. Input JSON: {\"diff\": \"...\"}. Paths in the diff must be under workspace.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			var args applyUnifiedDiffArgs
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				return "", "invalid JSON: " + err.Error(), false
			}
			b, _ := json.Marshal(args)
			sr := applyUnifiedDiffStep(0, b, te.Workspace)
			if sr.Status != statusSuccess {
				return sr.Output, sr.Error, false
			}
			return sr.Output, "", false
		},
	}
}

func listTreeTool() *SBATool {
	return &SBATool{
		name: "list_tree",
		desc: "List directory tree under workspace. Input JSON: {\"path\": \"optional/subdir\"} or {} for workspace root.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			var args listTreeArgs
			_ = json.Unmarshal([]byte(raw), &args)
			b, _ := json.Marshal(args)
			sr := listTreeStep(0, b, te.MaxOutputBytes, te.Workspace)
			if sr.Status != statusSuccess {
				return "", sr.Error, false
			}
			if strings.HasSuffix(sr.Output, "\n...[truncated]") {
				return "", errMsgMaxOutputBytes, true
			}
			return sr.Output, "", false
		},
	}
}

//nolint:dupl // same pattern as readFileTool, different step
func searchFilesTool() *SBATool {
	return &SBATool{
		name: "search_files",
		desc: "Search for a regex pattern in files under workspace. Input JSON: {\"pattern\": \"regex\", \"path\": \"optional/subdir\", \"include\": \"*.go\"}. Returns path:line_num:content per match; output capped.",
		call: func(ctx context.Context, raw string, te *ToolEnv) (out string, errMsg string, constraintViolation bool) {
			return runCappedStepTool(raw, te, searchFilesStep)
		},
	}
}

// EvalLocalTool runs a single tool by name with JSON input (for tests). It does not use context ToolEnv.
func EvalLocalTool(ctx context.Context, name, input, workspace string, maxOutputBytes int) (output string, err error) {
	if maxOutputBytes <= 0 {
		maxOutputBytes = 1024 * 1024
	}
	env := &ToolEnv{Workspace: workspace, MaxOutputBytes: maxOutputBytes}
	ctx = ContextWithToolEnv(ctx, env)
	for _, t := range NewLocalTools() {
		if t.Name() == name {
			return t.Call(ctx, input)
		}
	}
	return "", nil
}

var _ tools.Tool = (*SBATool)(nil)
