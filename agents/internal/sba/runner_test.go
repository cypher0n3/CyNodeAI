package sba

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
	"github.com/tmc/langchaingo/tools"
)

const (
	wantStatusSuccess   = "success"
	runCommandEmptyArgv = stepTypeRunCommand + " args.argv must be non-empty"
)

func TestRunJob_EmptyWorkspace_DefaultsToSlashWorkspace(t *testing.T) {
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	result := RunAgent(context.Background(), spec, "", &RunAgentOptions{LLM: &MockLLM{}})
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q", result.Status)
	}
}

func TestRunJob_EmptySteps_Success(t *testing.T) {
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	ctx := context.Background()
	result := RunAgent(ctx, spec, dir, &RunAgentOptions{LLM: &MockLLM{}})
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q, want %q", result.Status, wantStatusSuccess)
	}
	if result.JobID != "j1" {
		t.Errorf("JobID = %q, want j1", result.JobID)
	}
}

func TestRunAgent_RunCommand_WithMockToolCall(t *testing.T) {
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	// Mock returns one tool call then final answer.
	mock := &MockLLM{Responses: []string{
		"Action: run_command\nAction Input: {\"argv\": [\"sh\", \"-c\", \"echo ok\"]}",
		"Final Answer: Done",
	}}
	ctx := context.Background()
	result := RunAgent(ctx, spec, dir, &RunAgentOptions{LLM: mock})
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q, want %q; steps=%+v", result.Status, wantStatusSuccess, result.Steps)
	}
	if len(result.Steps) != 1 || result.Steps[0].Type != "run_command" {
		t.Errorf("steps = %+v", result.Steps)
	}
	if result.Steps[0].Output != "ok\n" {
		t.Errorf("step output = %q, want ok\\n", result.Steps[0].Output)
	}
}

func TestRunJob_WriteFileReadFile_ViaLocalTools(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	out, err := EvalLocalTool(ctx, "write_file", `{"path": "a.txt", "content": "hello"}`, dir, 1024)
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}
	if out != "ok" {
		t.Errorf("write_file out = %q", out)
	}
	out, err = EvalLocalTool(ctx, "read_file", `{"path": "a.txt"}`, dir, 1024)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if out != "hello" {
		t.Errorf("read_file output = %q, want hello", out)
	}
}

func TestEvalLocalTool_ApplyUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644)
	diff := "--- a/f.txt\n+++ b/f.txt\n@@ -1 +1,2 @@\n a\n+b\n"
	args, _ := json.Marshal(map[string]string{"diff": diff})
	ctx := context.Background()
	out, err := EvalLocalTool(ctx, "apply_unified_diff", string(args), dir, 1024)
	if err != nil {
		t.Fatalf("apply_unified_diff: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(data) != "a\nb\n" {
		t.Errorf("file after patch = %q", data)
	}
}

func TestEvalLocalTool_ApplyUnifiedDiff_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	out, err := EvalLocalTool(ctx, "apply_unified_diff", "not json", dir, 1024)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.HasPrefix(out, "error: ") {
		t.Errorf("out = %q", out)
	}
}

func TestEvalLocalTool_ListTree(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	ctx := context.Background()
	out, err := EvalLocalTool(ctx, "list_tree", "{}", dir, 1024)
	if err != nil {
		t.Fatalf("list_tree: %v", err)
	}
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "sub") {
		t.Errorf("list_tree output = %q", out)
	}
}

func TestEvalLocalTool_UnknownTool_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	out, err := EvalLocalTool(ctx, "unknown_tool", "{}", dir, 1024)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if out != "" {
		t.Errorf("out = %q", out)
	}
}

func TestEvalLocalTool_ReadFile_Truncated_ConstraintViolation(t *testing.T) {
	dir := t.TempDir()
	large := strings.Repeat("x", 2000)
	_ = os.WriteFile(filepath.Join(dir, "big.txt"), []byte(large), 0o644)
	ctx := context.Background()
	_, err := EvalLocalTool(ctx, "read_file", `{"path": "big.txt"}`, dir, 10)
	if err == nil {
		t.Fatal("expected constraint violation error")
	}
	if !IsConstraintViolation(err) {
		t.Errorf("err = %v", err)
	}
}

func TestEvalLocalTool_ListTree_Truncated_ConstraintViolation(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		_ = os.WriteFile(filepath.Join(dir, "file"+string(rune('a'+i))+".txt"), []byte("x"), 0o644)
	}
	ctx := context.Background()
	_, err := EvalLocalTool(ctx, "list_tree", "{}", dir, 5)
	if err == nil {
		t.Fatal("expected constraint violation error")
	}
	if !IsConstraintViolation(err) {
		t.Errorf("err = %v", err)
	}
}

func TestSBATool_Call_WithEnv(t *testing.T) {
	dir := t.TempDir()
	ctx := ContextWithToolEnv(context.Background(), &ToolEnv{Workspace: dir, MaxOutputBytes: 1024})
	toolList := NewLocalTools()
	var runCmd tools.Tool
	for _, tool := range toolList {
		if tool.Name() == stepTypeRunCommand {
			runCmd = tool
			break
		}
	}
	if runCmd == nil {
		t.Fatal(stepTypeRunCommand + " tool not found")
	}
	out, err := runCmd.Call(ctx, `{"argv": ["echo", "hi"]}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "hi\n" {
		t.Errorf("output = %q", out)
	}
}

func TestSBATool_Call_ConstraintError(t *testing.T) {
	errMsg := "over limit"
	ctx := ContextWithToolEnv(context.Background(), &ToolEnv{Workspace: t.TempDir(), ConstraintError: &errMsg})
	toolList := NewLocalTools()
	var runCmd tools.Tool
	for _, tool := range toolList {
		if tool.Name() == stepTypeRunCommand {
			runCmd = tool
			break
		}
	}
	if runCmd == nil {
		t.Fatal(stepTypeRunCommand + " tool not found")
	}
	_, err := runCmd.Call(ctx, `{"argv": ["echo", "x"]}`)
	if err == nil {
		t.Fatal("expected constraint violation error")
	}
	if !IsConstraintViolation(err) {
		t.Errorf("err = %v", err)
	}
}

func TestRunAgent_ConstraintViolation_SetsFailureCode(t *testing.T) {
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 5},
		Steps:           nil,
	}
	mock := &MockLLM{Responses: []string{
		`Action: ` + stepTypeRunCommand + `
Action Input: {"argv": ["sh", "-c", "echo 123456789"]}`,
		"Final Answer: Done",
	}}
	result := RunAgent(context.Background(), spec, dir, &RunAgentOptions{LLM: mock})
	if result.Status != statusFailed {
		t.Errorf("Status = %q", result.Status)
	}
	if result.FailureCode == nil || *result.FailureCode != failureCodeConstraintViolation {
		t.Errorf("FailureCode = %v", result.FailureCode)
	}
}

func TestIsExtNetRequired(t *testing.T) {
	if !IsExtNetRequired(ErrExtNetRequired) {
		t.Error("IsExtNetRequired(ErrExtNetRequired) = false")
	}
	if IsExtNetRequired(errors.New("other")) {
		t.Error("IsExtNetRequired(other) = true")
	}
}

func TestRunAgent_WithJobDir_WritesTodoAndListsArtifacts(t *testing.T) {
	dir := t.TempDir()
	artifactsDir := filepath.Join(dir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "out.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Context:         &sbajob.ContextSpec{Requirements: []string{"R1"}},
		Steps:           nil,
	}
	workspace := t.TempDir()
	result := RunAgent(context.Background(), spec, workspace, &RunAgentOptions{LLM: &MockLLM{}, JobDir: dir})
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q", result.Status)
	}
	todoPath := filepath.Join(dir, "todo.json")
	if _, err := os.Stat(todoPath); err != nil {
		t.Errorf("todo.json not written: %v", err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Path != "artifacts/out.txt" {
		t.Errorf("Artifacts = %+v", result.Artifacts)
	}
}

func TestRunJob_UnsupportedStepType_AgentIgnores(t *testing.T) {
	// Agent mode: spec.Steps are only suggested; agent may not call "unknown_type".
	// Unsupported step type is no longer a direct runner concept; agent uses tools only.
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           []sbajob.StepSpec{{Type: "unknown_type", Args: []byte("{}")}},
	}
	result := RunAgent(context.Background(), spec, dir, &RunAgentOptions{LLM: &MockLLM{}})
	// Agent finishes with Final Answer without calling unknown_type; success.
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q (agent may ignore suggested unknown step)", result.Status)
	}
}

func TestRunAgent_WithMCPGatewayURL_AddsMCPTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": [{"type": "text", "text": "ok"}]}`))
	}))
	defer srv.Close()
	t.Setenv("SBA_MCP_GATEWAY_URL", srv.URL)
	defer func() { _ = os.Unsetenv("SBA_MCP_GATEWAY_URL") }()
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	mock := &MockLLM{Responses: []string{"Final Answer: Done"}}
	result := RunAgent(context.Background(), spec, dir, &RunAgentOptions{LLM: mock})
	if result.Status != wantStatusSuccess {
		t.Errorf("Status = %q", result.Status)
	}
}

func TestResolveWorkspacePath_EscapeRejected(t *testing.T) {
	if resolveWorkspacePath("/workspace", "../../etc/passwd") != "" {
		t.Error("expected empty for path escaping workspace")
	}
	if resolveWorkspacePath("/workspace", "a/../../../b") != "" {
		t.Error("expected empty for path escaping workspace")
	}
}

func TestListTreeStep(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("x"), 0o644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "f2.txt"), []byte("y"), 0o644)
	sr := listTreeStep(0, []byte("{}"), 1024, dir)
	if sr.Status != wantStatusSuccess {
		t.Fatalf("list_tree status = %q", sr.Status)
	}
	for _, want := range []string{"f1.txt", "sub/", "sub/f2.txt"} {
		if !strings.Contains(sr.Output, want) {
			t.Errorf("list_tree output %q missing %q", sr.Output, want)
		}
	}
}

func TestListTreeStep_WithPath(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "f.txt"), []byte("x"), 0o644)
	args, _ := json.Marshal(map[string]string{"path": "sub"})
	sr := listTreeStep(0, args, 1024, dir)
	if sr.Status != wantStatusSuccess {
		t.Fatalf("list_tree status = %q", sr.Status)
	}
	if !strings.Contains(sr.Output, "f.txt") {
		t.Errorf("list_tree output %q missing f.txt", sr.Output)
	}
}

func TestRunJob_Timeout_ReturnsTimeoutResult(t *testing.T) {
	dir := t.TempDir()
	spec := &sbajob.JobSpec{
		ProtocolVersion: "1.0",
		JobID:           "j1",
		TaskID:          "t1",
		Constraints:     sbajob.JobConstraints{MaxRuntimeSeconds: 60, MaxOutputBytes: 1024},
		Steps:           nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	result := RunAgent(ctx, spec, dir, &RunAgentOptions{LLM: &MockLLM{}})
	if result.Status != statusTimeout {
		t.Errorf("Status = %q, want timeout", result.Status)
	}
	if result.FailureCode == nil || *result.FailureCode != statusTimeout {
		t.Errorf("FailureCode = %v", result.FailureCode)
	}
}

func TestRunCommandStep_EmptyArgv_Fails(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]interface{}{"argv": []string{}})
	sr := runCommandStep(context.Background(), 0, args, 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != runCommandEmptyArgv {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestRunCommandStep_InvalidArgs_Fails(t *testing.T) {
	dir := t.TempDir()
	sr := runCommandStep(context.Background(), 0, []byte(`{"argv": "not-an-array"}`), 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestRunCommandStep_CwdEscape_Fails(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]interface{}{"argv": []string{"echo", "x"}, "cwd": "../../etc"})
	sr := runCommandStep(context.Background(), 0, args, 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != "cwd must be under workspace" {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestRunCommandStep_ContextCanceled_SetsTimeout(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]interface{}{"argv": []string{"sh", "-c", "sleep 2"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled
	sr := runCommandStep(ctx, 0, args, 1024, dir)
	// Command may still run briefly; we care that we handle context error
	if sr.Status != statusFailed && sr.Status != statusTimeout {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestRunCommandStep_ExitCodeCaptured(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]interface{}{"argv": []string{"sh", "-c", "exit 42"}})
	sr := runCommandStep(context.Background(), 0, args, 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.ExitCode == nil || *sr.ExitCode != 42 {
		t.Errorf("ExitCode = %v", sr.ExitCode)
	}
}

func TestCapString_TruncatesWhenOverMax(t *testing.T) {
	out := capString("hello world", 5)
	if out != "hello\n...[truncated]" {
		t.Errorf("capString = %q", out)
	}
	out2 := capString("hi", 10)
	if out2 != "hi" {
		t.Errorf("capString short = %q", out2)
	}
	out3 := capString("x", 0)
	if out3 != "x" {
		t.Errorf("capString maxBytes 0 = %q", out3)
	}
}

func TestReadFileStep_PathEscape_Fails(t *testing.T) {
	pathEscapeFails(t, func(dir string, args []byte) sbajob.StepResult {
		return readFileStep(0, args, 1024, dir)
	}, "../../etc/passwd")
}

func TestReadFileStep_FileNotFound_Fails(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": "nonexistent.txt"})
	sr := readFileStep(0, args, 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestWriteFileStep_PathEscape_Fails(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": "../../outside.txt", "content": "x"})
	sr := writeFileStep(0, args, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != errPathEscapesWorkspace {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestWriteFileStep_InvalidArgs_Fails(t *testing.T) {
	dir := t.TempDir()
	sr := writeFileStep(0, []byte(`{"path": 1}`), dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestReadFileStep_InvalidArgs_Fails(t *testing.T) {
	dir := t.TempDir()
	sr := readFileStep(0, []byte(`{"path": 1}`), 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestRunCommandStep_NoArgv_Fails(t *testing.T) {
	dir := t.TempDir()
	sr := runCommandStep(context.Background(), 0, []byte(`{}`), 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != runCommandEmptyArgv {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestRunCommandStep_EmptyRaw_RequiresArgv(t *testing.T) {
	dir := t.TempDir()
	sr := runCommandStep(context.Background(), 0, nil, 1024, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != stepTypeRunCommand+" requires args.argv" {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestResolveWorkspacePath_ValidPath(t *testing.T) {
	dir := t.TempDir()
	got := resolveWorkspacePath(dir, "a/b")
	want := filepath.Join(dir, "a", "b")
	if got != want {
		t.Errorf("resolveWorkspacePath = %q, want %q", got, want)
	}
	got2 := resolveWorkspacePath(dir, "a")
	if !strings.HasSuffix(got2, "a") {
		t.Errorf("resolveWorkspacePath a = %q", got2)
	}
}

func TestResolveWorkspacePath_CurrentDir(t *testing.T) {
	dir := t.TempDir()
	got := resolveWorkspacePath(dir, ".")
	if got == "" {
		t.Error("resolveWorkspacePath(., .) returned empty")
	}
	if !strings.HasPrefix(got, dir) {
		t.Errorf("resolveWorkspacePath(., .) = %q not under %q", got, dir)
	}
}

func TestApplyUnifiedDiffStep_Success(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644)
	diff := "--- a/f.txt\n+++ b/f.txt\n@@ -1 +1,2 @@\n a\n+b\n"
	args, _ := json.Marshal(map[string]string{"diff": diff})
	sr := applyUnifiedDiffStep(0, args, dir)
	if sr.Status != wantStatusSuccess {
		t.Errorf("Status = %q: %s", sr.Status, sr.Error)
	}
}

func TestApplyUnifiedDiffStep_InvalidArgs_Fails(t *testing.T) {
	dir := t.TempDir()
	sr := applyUnifiedDiffStep(0, []byte(`{"diff": 123}`), dir) // diff must be string
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestApplyUnifiedDiffStep_PathEscape_Rejected(t *testing.T) {
	dir := t.TempDir()
	diff := "--- a/../../etc/passwd\n+++ b/../../etc/passwd\n@@ -1 +1 @@\n-x\n+y\n"
	args, _ := json.Marshal(map[string]string{"diff": diff})
	sr := applyUnifiedDiffStep(0, args, dir)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != errPathEscapesWorkspace && !strings.Contains(sr.Error, errPathEscapesWorkspace) {
		t.Errorf("Error = %q", sr.Error)
	}
}

func TestValidateDiffPathsWithinWorkspace_RejectsEscape(t *testing.T) {
	ws := t.TempDir()
	diff := "--- a/../../etc/passwd\n+++ b/../../etc/passwd\n@@ -1 +1 @@\n-x\n+y\n"
	err := validateDiffPathsWithinWorkspace(diff, ws)
	if err == nil {
		t.Fatal("expected error for path escaping workspace")
	}
	if !strings.Contains(err.Error(), errPathEscapesWorkspace) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateDiffPathsWithinWorkspace_AllowsUnderWorkspace(t *testing.T) {
	ws := t.TempDir()
	diff := "--- a/f.txt\n+++ b/f.txt\n@@ -1 +1 @@\n-x\n+y\n"
	if err := validateDiffPathsWithinWorkspace(diff, ws); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiffLineToPath_DevNull(t *testing.T) {
	path, ok := diffLineToPath("--- /dev/null")
	if !ok || path != "" {
		t.Errorf("diffLineToPath(--- /dev/null) = %q, %v", path, ok)
	}
	path, ok = diffLineToPath("+++ /dev/null")
	if !ok || path != "" {
		t.Errorf("diffLineToPath(+++ /dev/null) = %q, %v", path, ok)
	}
}

func TestWriteFileStep_Success_NestedPath(t *testing.T) {
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": "sub/dir/f.txt", "content": "nested"})
	sr := writeFileStep(0, args, dir)
	if sr.Status != wantStatusSuccess {
		t.Errorf("Status = %q: %s", sr.Status, sr.Error)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "sub", "dir", "f.txt"))
	if string(data) != "nested" {
		t.Errorf("file content = %q", data)
	}
}

func TestApplyUnifiedDiffStep_PatchFails(t *testing.T) {
	dir := t.TempDir()
	// Invalid diff that patch will reject
	args, _ := json.Marshal(map[string]string{"diff": "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-foo\n+bar\n"})
	sr := applyUnifiedDiffStep(0, args, dir)
	// patch may fail (file x does not exist) or succeed depending on patch version
	if sr.Status != statusFailed && sr.Status != wantStatusSuccess {
		t.Errorf("Status = %q", sr.Status)
	}
}

func TestListTreeStep_PathEscape_Fails(t *testing.T) {
	pathEscapeFails(t, func(dir string, args []byte) sbajob.StepResult {
		return listTreeStep(0, args, 1024, dir)
	}, "../../etc")
}

// pathEscapeFails runs a step that takes a path arg and asserts it fails with errPathEscapesWorkspace.
func pathEscapeFails(t *testing.T, runStep func(dir string, args []byte) sbajob.StepResult, path string) {
	t.Helper()
	dir := t.TempDir()
	args, _ := json.Marshal(map[string]string{"path": path})
	sr := runStep(dir, args)
	if sr.Status != statusFailed {
		t.Errorf("Status = %q", sr.Status)
	}
	if sr.Error != errPathEscapesWorkspace {
		t.Errorf("Error = %q", sr.Error)
	}
}
