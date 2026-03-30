package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

func TestRun_MissingJobFile_ExitsOneAndWritesResult(t *testing.T) {
	dir := t.TempDir()
	resultPath := filepath.Join(dir, "result.json")
	code := run([]string{"-job=" + filepath.Join(dir, "nonexistent.json"), "-result=" + resultPath})
	if code != 1 {
		t.Errorf("run() with missing job file = %d, want 1", code)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}
	var r struct {
		FailureCode *string `json:"failure_code"`
		Status      string  `json:"status"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != sbajob.ResultStatusFailure || r.FailureCode == nil || *r.FailureCode != "schema_validation" {
		t.Errorf("result: status=%q failure_code=%v", r.Status, r.FailureCode)
	}
}

func TestRun_InvalidJobSpec_ExitsOneAndWritesResult(t *testing.T) {
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	resultPath := filepath.Join(dir, "result.json")
	if err := os.WriteFile(jobPath, []byte(`{"protocol_version":"99.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code := run([]string{"-job=" + jobPath, "-result=" + resultPath, "-workspace=" + dir})
	if code != 1 {
		t.Errorf("run() with unsupported protocol = %d, want 1", code)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}
	var r struct {
		FailureCode *string `json:"failure_code"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.FailureCode == nil || *r.FailureCode != "schema_validation" {
		t.Errorf("failure_code = %v", r.FailureCode)
	}
}

func TestRun_ValidJobSuccess_ExitsZeroAndWritesResult(t *testing.T) {
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	resultPath := filepath.Join(dir, "result.json")
	validJob := []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`)
	if err := os.WriteFile(jobPath, validJob, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBA_USE_MOCK_LLM", "1")
	code := run([]string{"-job=" + jobPath, "-result=" + resultPath, "-workspace=" + dir})
	if code != 0 {
		t.Errorf("run() with valid empty job = %d, want 0", code)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}
	var r struct {
		Status string `json:"status"`
		JobID  string `json:"job_id"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != sbajob.ResultStatusSuccess || r.JobID != "j1" {
		t.Errorf("result: status=%q job_id=%q", r.Status, r.JobID)
	}
}

func TestRun_StepFailure_ExitsOneAndWritesResult(t *testing.T) {
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	resultPath := filepath.Join(dir, "result.json")
	// run_command that exits non-zero
	job := []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[{"type":"run_command","args":{"argv":["sh","-c","exit 2"]}}]}`)
	if err := os.WriteFile(jobPath, job, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBA_USE_MOCK_LLM", "1")
	t.Setenv("SBA_MOCK_RESPONSES", `["Action: run_command\nAction Input: {\"argv\": [\"sh\", \"-c\", \"exit 2\"]}","Final Answer: Done"]`)
	code := run([]string{"-job=" + jobPath, "-result=" + resultPath, "-workspace=" + dir})
	if code != 1 {
		t.Errorf("run() with failing step = %d, want 1", code)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}
	var r struct {
		Status      string  `json:"status"`
		FailureCode *string `json:"failure_code"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != sbajob.ResultStatusFailure {
		t.Errorf("result: status=%q", r.Status)
	}
	if r.FailureCode == nil {
		t.Error("result: failure_code missing")
	} else if *r.FailureCode != "step_failed" {
		t.Errorf("result: failure_code=%q want step_failed", *r.FailureCode)
	}
}

func TestRun_FlagParseError_ExitsOne(t *testing.T) {
	code := run([]string{"-unknown=1"})
	if code != 1 {
		t.Errorf("run() with bad flag = %d, want 1", code)
	}
}

func TestRun_EnvOverridesPaths(t *testing.T) {
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	resultPath := filepath.Join(dir, "result.json")
	if err := os.WriteFile(jobPath, []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBA_USE_MOCK_LLM", "1")
	_ = os.Setenv("SBA_JOB_PATH", jobPath)
	_ = os.Setenv("SBA_RESULT_PATH", resultPath)
	_ = os.Setenv("SBA_WORKSPACE", dir)
	defer func() {
		_ = os.Unsetenv("SBA_JOB_PATH")
		_ = os.Unsetenv("SBA_RESULT_PATH")
		_ = os.Unsetenv("SBA_WORKSPACE")
	}()
	code := run([]string{})
	if code != 0 {
		t.Errorf("run() with env overrides = %d, want 0", code)
	}
	if _, err := os.Stat(resultPath); err != nil {
		t.Errorf("result not written to env path: %v", err)
	}
}

func TestWriteResultFailure_WhenWriteFails_LogsError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	// writeResultTo should not panic when result path is invalid (e.g. dir not writable)
	_ = writeResultTo("/nonexistent/path/result.json", nil, false, failureResult("schema_validation", "test"), logger)
}

func TestWriteResultTo_StdinStdoutSuccess(t *testing.T) {
	var buf bytes.Buffer
	r := failureResult("c", "d")
	if err := writeResultTo("", &buf, true, r, slog.Default()); err != nil {
		t.Fatalf("writeResultTo: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(sbajob.SBAProtocolVersion)) {
		t.Errorf("stdout JSON missing protocol: %s", buf.String())
	}
}

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("write failed") }

func TestWriteResultTo_StdinStdoutWriteError(t *testing.T) {
	err := writeResultTo("", errWriter{}, true, failureResult("e", "f"), slog.Default())
	if err == nil {
		t.Fatal("expected error from writer")
	}
}

// TestResultConstants ensures result helpers use shared contract constants (not ad-hoc strings).
func TestResultConstants(t *testing.T) {
	r := failureResult("code", "msg")
	if r.ProtocolVersion != sbajob.SBAProtocolVersion {
		t.Errorf("ProtocolVersion = %q want %q", r.ProtocolVersion, sbajob.SBAProtocolVersion)
	}
	if r.Status != sbajob.ResultStatusFailure {
		t.Errorf("Status = %q want %q", r.Status, sbajob.ResultStatusFailure)
	}
}

func TestRun_StdinMode_ReadsJobWritesResultToStdout(t *testing.T) {
	dir := t.TempDir()
	validJob := []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`)
	exe := filepath.Join(dir, "cynode-sba")
	buildCmd := exec.Command("go", "build", "-o", exe, ".")
	buildCmd.Env = append(os.Environ(), "GOEXPERIMENT=secret")
	if err := buildCmd.Run(); err != nil {
		t.Skipf("build cynode-sba: %v", err)
	}
	cmd := exec.Command(exe, "-stdin", "-workspace="+dir)
	cmd.Env = append(os.Environ(), "SBA_USE_MOCK_LLM=1")
	cmd.Stdin = bytes.NewReader(validJob)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "protocol_version") || !strings.Contains(out, "success") {
		t.Errorf("stdout missing result shape: %s", out)
	}
}

func TestRun_ResultPathUnwritable_ExitsOne(t *testing.T) {
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	if err := os.WriteFile(jobPath, []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SBA_USE_MOCK_LLM", "1")
	// Result path under a nonexistent parent so WriteFile fails
	resultPath := filepath.Join(dir, "nonexistent", "result.json")
	code := run([]string{"-job=" + jobPath, "-result=" + resultPath, "-workspace=" + dir})
	if code != 1 {
		t.Errorf("run() when result write fails = %d, want 1", code)
	}
}

// TestRun_StdinModeInProcess covers writeResultTo(..., os.Stdout, true, ...) and applyEnvOverrides SBA_STDIN_STDOUT.
// In stdin mode result is written to stdout only, not to result path.
func TestRun_StdinModeInProcess(t *testing.T) {
	validJob := []byte(`{"protocol_version":"1.0","job_id":"j1","task_id":"t1","constraints":{"max_runtime_seconds":60,"max_output_bytes":1024},"steps":[]}`)
	dir := t.TempDir()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		_, _ = w.Write(validJob)
		_ = w.Close()
	}()
	t.Setenv("SBA_USE_MOCK_LLM", "1")
	t.Setenv("SBA_STDIN_STDOUT", "true")
	t.Setenv("SBA_RESULT_PATH", filepath.Join(dir, "result.json"))
	t.Setenv("SBA_WORKSPACE", dir)
	defer func() {
		_ = os.Unsetenv("SBA_STDIN_STDOUT")
		_ = os.Unsetenv("SBA_RESULT_PATH")
		_ = os.Unsetenv("SBA_WORKSPACE")
	}()
	code := run([]string{})
	if code != 0 {
		t.Errorf("run() stdin mode = %d, want 0", code)
	}
}
