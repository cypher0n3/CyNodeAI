// Package sba provides the SBA job runner: parse job spec, execute steps, produce result contract.
// See docs/tech_specs/cynode_sba.md and docs/requirements/sbagnt.md.
package sba

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

// Step/result status and canonical error strings per CYNAI.SBAGNT.ResultContract and FailureCodes.
const (
	statusSuccess = "success"
	statusFailed  = "failure" // result and step status per spec: success | failure | timeout
	statusTimeout = "timeout"

	errPathEscapesWorkspace = "path escapes workspace"
	stepTypeRunCommand      = "run_command"
)

// RunJob executes the validated job spec via the agent loop (single execution mode) and returns the result contract.
// The context deadline is used for max_runtime_seconds; the runner enforces max_output_bytes on tool output.
func RunJob(ctx context.Context, spec *sbajob.JobSpec, workspace string) *sbajob.Result {
	return RunAgent(ctx, spec, workspace, nil)
}

type runCommandArgs struct {
	Argv []string `json:"argv"`
	Cwd  string   `json:"cwd,omitempty"`
}

func runCommandStep(ctx context.Context, index int, raw json.RawMessage, maxOutputBytes int, workspace string) sbajob.StepResult {
	sr := sbajob.StepResult{Index: index, Type: stepTypeRunCommand, Status: statusSuccess}
	var args runCommandArgs
	if len(raw) == 0 {
		sr.Status = statusFailed
		sr.Error = stepTypeRunCommand + " requires args.argv"
		return sr
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		sr.Status = statusFailed
		sr.Error = "invalid run_command args: " + err.Error()
		return sr
	}
	if len(args.Argv) == 0 {
		sr.Status = statusFailed
		sr.Error = stepTypeRunCommand + " args.argv must be non-empty"
		return sr
	}
	dir := workspace
	if args.Cwd != "" {
		dir = filepath.Join(workspace, filepath.Clean(strings.TrimPrefix(args.Cwd, "/")))
		if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(workspace)) {
			sr.Status = statusFailed
			sr.Error = "cwd must be under workspace"
			return sr
		}
	}
	cmd := exec.CommandContext(ctx, args.Argv[0], args.Argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		sr.Status = statusFailed
		if ctx.Err() != nil {
			sr.Status = statusTimeout
		}
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() > 0 {
			exitCode = exitErr.ExitCode()
		}
		sr.ExitCode = &exitCode
		sr.Output = capString(string(out), maxOutputBytes)
		sr.Error = err.Error()
		return sr
	}
	sr.Output = capString(string(out), maxOutputBytes)
	return sr
}

type writeFileArgs struct {
	Path  string `json:"path"`
	Content string `json:"content"`
}

func writeFileStep(index int, raw json.RawMessage, workspace string) sbajob.StepResult {
	sr := sbajob.StepResult{Index: index, Type: "write_file", Status: statusSuccess}
	var args writeFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		sr.Status = statusFailed
		sr.Error = "invalid write_file args: " + err.Error()
		return sr
	}
	full := resolveWorkspacePath(workspace, args.Path)
	if full == "" {
		sr.Status = statusFailed
		sr.Error = errPathEscapesWorkspace
		return sr
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		return sr
	}
	if err := os.WriteFile(full, []byte(args.Content), 0o644); err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		return sr
	}
	return sr
}

type readFileArgs struct {
	Path string `json:"path"`
}

func readFileStep(index int, raw json.RawMessage, maxOutputBytes int, workspace string) sbajob.StepResult {
	sr := sbajob.StepResult{Index: index, Type: "read_file", Status: statusSuccess}
	var args readFileArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		sr.Status = statusFailed
		sr.Error = "invalid read_file args: " + err.Error()
		return sr
	}
	full := resolveWorkspacePath(workspace, args.Path)
	if full == "" {
		sr.Status = statusFailed
		sr.Error = errPathEscapesWorkspace
		return sr
	}
	data, err := os.ReadFile(full)
	if err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		return sr
	}
	sr.Output = capString(string(data), maxOutputBytes)
	return sr
}

type applyUnifiedDiffArgs struct {
	Diff string `json:"diff"`
}

func applyUnifiedDiffStep(index int, raw json.RawMessage, workspace string) sbajob.StepResult {
	sr := sbajob.StepResult{Index: index, Type: "apply_unified_diff", Status: statusSuccess}
	var args applyUnifiedDiffArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		sr.Status = statusFailed
		sr.Error = "invalid apply_unified_diff args: " + err.Error()
		return sr
	}
	if err := validateDiffPathsWithinWorkspace(args.Diff, workspace); err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		return sr
	}
	cmd := exec.Command("patch", "-p1", "-d", workspace, "--forward")
	cmd.Stdin = strings.NewReader(args.Diff)
	out, err := cmd.CombinedOutput()
	if err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		sr.Output = string(out)
		return sr
	}
	sr.Output = string(out)
	return sr
}

// validateDiffPathsWithinWorkspace parses unified diff for ---/+++ paths and rejects any that escape workspace.
func validateDiffPathsWithinWorkspace(diff, workspace string) error {
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		path, ok := diffLineToPath(line)
		if !ok || path == "" || path == "/dev/null" {
			continue
		}
		if resolveWorkspacePath(workspace, path) == "" {
			return fmt.Errorf("%s: %s", errPathEscapesWorkspace, path)
		}
	}
	return nil
}

// diffLineToPath returns the file path from a unified diff ---/+++ line, or ("", false) if not a path line.
func diffLineToPath(line string) (path string, ok bool) {
	switch {
	case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
		path = strings.TrimSpace(line[4:])
	default:
		return "", false
	}
	if path == "" || path == "/dev/null" {
		return "", true
	}
	if len(path) >= 2 && (path[0] == 'a' || path[0] == 'b') && path[1] == '/' {
		path = path[2:]
	}
	return strings.TrimSpace(path), true
}

type listTreeArgs struct {
	Path string `json:"path,omitempty"`
}

func listTreeStep(index int, raw json.RawMessage, maxOutputBytes int, workspace string) sbajob.StepResult {
	sr := sbajob.StepResult{Index: index, Type: "list_tree", Status: statusSuccess}
	var args listTreeArgs
	_ = json.Unmarshal(raw, &args)
	dir := workspace
	if args.Path != "" {
		dir = resolveWorkspacePath(workspace, args.Path)
		if dir == "" {
			sr.Status = statusFailed
			sr.Error = errPathEscapesWorkspace
			return sr
		}
	}
	var out strings.Builder
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." {
			rel = ""
		}
		prefix := ""
		if info.IsDir() {
			prefix = rel + "/"
		} else {
			prefix = rel
		}
		if prefix != "" {
			out.WriteString(prefix + "\n")
		}
		return nil
	})
	if err != nil {
		sr.Status = statusFailed
		sr.Error = err.Error()
		return sr
	}
	sr.Output = capString(out.String(), maxOutputBytes)
	return sr
}

// resolveWorkspacePath joins workspace with path and ensures result is under workspace (no symlink escape).
func resolveWorkspacePath(workspace, path string) string {
	path = filepath.Clean(strings.TrimPrefix(path, "/"))
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return ""
	}
	full := filepath.Join(workspace, path)
	abs, err := filepath.Abs(full)
	if err != nil {
		return ""
	}
	wsAbs, _ := filepath.Abs(workspace)
	if !strings.HasPrefix(abs, wsAbs) {
		return ""
	}
	return full
}

func capString(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return s
	}
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[truncated]"
}
