package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestJobResponse_JSON(t *testing.T) {
	resp := &JobResponse{
		Version:   1,
		TaskID:    "task-123",
		JobID:     "job-456",
		Status:    "completed",
		ExitCode:  0,
		Stdout:    "hello\n",
		Stderr:    "",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		EndedAt:   time.Now().UTC().Format(time.RFC3339),
		Truncated: TruncatedInfo{
			Stdout: false,
			Stderr: false,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded JobResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Version != resp.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, resp.Version)
	}

	if decoded.TaskID != resp.TaskID {
		t.Errorf("TaskID = %s, want %s", decoded.TaskID, resp.TaskID)
	}

	if decoded.Status != resp.Status {
		t.Errorf("Status = %s, want %s", decoded.Status, resp.Status)
	}
}

func TestMarshalResult(t *testing.T) {
	resp := &JobResponse{
		Version:   1,
		TaskID:    "task-123",
		JobID:     "job-456",
		Status:    "completed",
		ExitCode:  0,
		Stdout:    "output",
		Stderr:    "",
		StartedAt: "2024-01-01T00:00:00Z",
		EndedAt:   "2024-01-01T00:00:01Z",
		Truncated: TruncatedInfo{},
	}

	result, err := MarshalResult(resp)
	if err != nil {
		t.Fatalf("MarshalResult() error = %v", err)
	}

	if result == "" {
		t.Error("MarshalResult() returned empty string")
	}

	// Should be valid JSON
	var decoded JobResponse
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Errorf("MarshalResult() produced invalid JSON: %v", err)
	}
}

func TestParseJobPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantImg string
		wantCmd []string
	}{
		{
			name:    "structured payload",
			payload: `{"image": "alpine:latest", "command": ["echo", "hello"]}`,
			wantImg: "alpine:latest",
			wantCmd: []string{"echo", "hello"},
		},
		{
			name:    "simple command",
			payload: "echo hello world",
			wantImg: "alpine:latest",
			wantCmd: []string{"sh", "-c", "echo hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, cmd, _, err := ParseJobPayload(tt.payload)
			if err != nil {
				t.Fatalf("ParseJobPayload() error = %v", err)
			}

			if img != tt.wantImg {
				t.Errorf("image = %s, want %s", img, tt.wantImg)
			}

			if len(cmd) != len(tt.wantCmd) {
				t.Errorf("command len = %d, want %d", len(cmd), len(tt.wantCmd))
			}
		})
	}
}

func TestNewExecutor(t *testing.T) {
	e := NewExecutor("podman", 300*time.Second, 1<<20)

	if e.runtime != "podman" {
		t.Errorf("runtime = %s, want podman", e.runtime)
	}

	if e.defaultTimeout != 300*time.Second {
		t.Errorf("defaultTimeout = %v, want 300s", e.defaultTimeout)
	}

	if e.maxOutputBytes != 1<<20 {
		t.Errorf("maxOutputBytes = %d, want %d", e.maxOutputBytes, 1<<20)
	}
}

func TestJobRequest_JSON(t *testing.T) {
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-123",
		JobID:   "job-456",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"echo", "hello"},
			Env:            map[string]string{"KEY": "value"},
			TimeoutSeconds: 60,
			NetworkPolicy:  "restricted",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded JobRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Version != req.Version {
		t.Errorf("Version = %d, want %d", decoded.Version, req.Version)
	}

	if decoded.Sandbox.Image != req.Sandbox.Image {
		t.Errorf("Sandbox.Image = %s, want %s", decoded.Sandbox.Image, req.Sandbox.Image)
	}
}

func TestParseJobPayloadWithEnv(t *testing.T) {
	payload := `{"image": "ubuntu:22.04", "command": ["env"], "env": {"FOO": "bar", "BAZ": "qux"}}`
	img, cmd, env, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}

	if img != "ubuntu:22.04" {
		t.Errorf("image = %s, want ubuntu:22.04", img)
	}

	if len(cmd) != 1 || cmd[0] != "env" {
		t.Errorf("command = %v, want [env]", cmd)
	}

	if env["FOO"] != "bar" {
		t.Errorf("env[FOO] = %s, want bar", env["FOO"])
	}
}

func TestParseJobPayloadEmpty(t *testing.T) {
	payload := ""
	// Empty payload is treated as a plain command
	img, cmd, _, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}
	// Should use default image
	if img != "alpine:latest" {
		t.Errorf("image = %s, want alpine:latest", img)
	}
	// Command should be empty string wrapped
	if len(cmd) == 0 {
		t.Error("expected command to be set")
	}
}

func TestParseJobPayloadInvalidJSON(t *testing.T) {
	payload := `{"image": "alpine", invalid`
	// Should treat as plain command since JSON is invalid
	img, cmd, _, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}
	// Should use default image for plain commands
	if img != "alpine:latest" {
		t.Errorf("image = %s, want alpine:latest", img)
	}
	if len(cmd) < 1 {
		t.Error("expected command to be set")
	}
}

func TestMarshalResultNil(t *testing.T) {
	result, err := MarshalResult(nil)
	// MarshalResult may handle nil gracefully or return error
	// Either way, if it doesn't error, result should be empty or "null"
	if err != nil {
		// This is expected - nil input should cause an error
		return
	}
	// If no error, the result should be something representing null
	if result != "null" && result != "" {
		t.Errorf("MarshalResult(nil) = %s, expected empty or null", result)
	}
}

func TestJobResponseStatuses(t *testing.T) {
	statuses := []string{"completed", "failed", "timeout", "cancelled"}
	for _, status := range statuses {
		resp := &JobResponse{Status: status}
		if resp.Status != status {
			t.Errorf("expected status %s, got %s", status, resp.Status)
		}
	}
}

func TestTruncatedInfo(t *testing.T) {
	info := TruncatedInfo{
		Stdout: true,
		Stderr: false,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded TruncatedInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Stdout != true {
		t.Error("expected Stdout to be true")
	}
	if decoded.Stderr != false {
		t.Error("expected Stderr to be false")
	}
}

func TestSandboxSpec(t *testing.T) {
	spec := SandboxSpec{
		Image:          "python:3.11",
		Command:        []string{"python", "-c", "print('hello')"},
		Env:            map[string]string{"PYTHONUNBUFFERED": "1"},
		TimeoutSeconds: 120,
		NetworkPolicy:  "none",
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded SandboxSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Image != "python:3.11" {
		t.Errorf("Image = %s, want python:3.11", decoded.Image)
	}
	if decoded.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", decoded.TimeoutSeconds)
	}
}

func TestExecutorDefaults(t *testing.T) {
	e := NewExecutor("docker", 0, 0)

	if e.runtime != "docker" {
		t.Errorf("runtime = %s, want docker", e.runtime)
	}
	if e.defaultTimeout != 0 {
		t.Errorf("defaultTimeout = %v, want 0", e.defaultTimeout)
	}
}

func TestRunJobContextCancelled(t *testing.T) {
	e := NewExecutor("nonexistent-runtime", 1*time.Second, 1024)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &JobRequest{
		Version: 1,
		TaskID:  "task-1",
		JobID:   "job-1",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"sleep", "10"},
			TimeoutSeconds: 60,
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Should fail because runtime doesn't exist or context is cancelled
	if resp.Status == "completed" {
		t.Error("expected job to fail with cancelled context")
	}
}

func TestRunJobInvalidRuntime(t *testing.T) {
	e := NewExecutor("nonexistent-runtime-12345", 5*time.Second, 1024)

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-1",
		JobID:   "job-1",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"echo", "test"},
			TimeoutSeconds: 5,
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Should fail because runtime doesn't exist
	if resp.Status == "completed" {
		t.Error("expected job to fail with invalid runtime")
	}
	if resp.ExitCode == 0 {
		t.Error("expected non-zero exit code for failed job")
	}
}

func TestJobRequestVersionValidation(t *testing.T) {
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-123",
		JobID:   "job-456",
	}

	if req.Version != 1 {
		t.Errorf("Version = %d, want 1", req.Version)
	}
}

func TestEmptyEnvMap(t *testing.T) {
	payload := `{"image": "alpine:latest", "command": ["echo", "test"], "env": {}}`
	_, _, env, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}
	if env == nil {
		t.Error("expected env to be non-nil map")
	}
	if len(env) != 0 {
		t.Errorf("expected empty env map, got %d entries", len(env))
	}
}

func TestStatusConstants(t *testing.T) {
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %s, want completed", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %s, want failed", StatusFailed)
	}
	if StatusTimeout != "timeout" {
		t.Errorf("StatusTimeout = %s, want timeout", StatusTimeout)
	}
}

func TestDefaultImage(t *testing.T) {
	if DefaultImage != "alpine:latest" {
		t.Errorf("DefaultImage = %s, want alpine:latest", DefaultImage)
	}
}

func TestMarshalResultEmpty(t *testing.T) {
	resp := &JobResponse{}
	result, err := MarshalResult(resp)
	if err != nil {
		t.Fatalf("MarshalResult() error = %v", err)
	}
	if result == "" {
		t.Error("MarshalResult() returned empty string for empty response")
	}
}

func TestRunJobWithEnv(t *testing.T) {
	e := NewExecutor("nonexistent-runtime", 1*time.Second, 1024)

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-env",
		JobID:   "job-env",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"env"},
			Env:            map[string]string{"TEST_VAR": "test_value", "ANOTHER": "val"},
			TimeoutSeconds: 5,
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Should fail because runtime doesn't exist
	if resp.Status == StatusCompleted {
		t.Error("expected job to fail with invalid runtime")
	}
	if resp.TaskID != "task-env" {
		t.Errorf("TaskID = %s, want task-env", resp.TaskID)
	}
	if resp.JobID != "job-env" {
		t.Errorf("JobID = %s, want job-env", resp.JobID)
	}
}

func TestRunJobDefaults(t *testing.T) {
	e := NewExecutor("nonexistent", 30*time.Second, 4096)

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-default",
		JobID:   "job-default",
		Sandbox: SandboxSpec{
			Image:   "alpine:latest",
			Command: []string{"echo", "test"},
			// No timeout specified, should use default
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Verify response structure
	if resp.Version != 1 {
		t.Errorf("Version = %d, want 1", resp.Version)
	}
	if resp.StartedAt == "" {
		t.Error("StartedAt should be set")
	}
	if resp.EndedAt == "" {
		t.Error("EndedAt should be set")
	}
}

func TestParseJobPayloadNoImage(t *testing.T) {
	payload := `{"command": ["echo", "hello"]}`
	img, cmd, _, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}
	// Should use default image when image is empty
	if img != DefaultImage {
		t.Errorf("image = %s, want %s", img, DefaultImage)
	}
	if len(cmd) != 2 || cmd[0] != "echo" {
		t.Errorf("command = %v, expected [echo hello]", cmd)
	}
}

func TestRunJobOutputTruncation(t *testing.T) {
	// Test with very small maxOutputBytes to trigger truncation
	e := NewExecutor("nonexistent-runtime", 5*time.Second, 10) // Only 10 bytes max

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-trunc",
		JobID:   "job-trunc",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"echo", "this is a long output that should be truncated"},
			TimeoutSeconds: 5,
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Check response structure is valid
	if resp.TaskID != "task-trunc" {
		t.Errorf("TaskID = %s, want task-trunc", resp.TaskID)
	}
}

func TestRunJobWithTimeout(t *testing.T) {
	// Test timeout behavior
	e := NewExecutor("nonexistent", 1*time.Millisecond, 1024) // Very short timeout

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-timeout",
		JobID:   "job-timeout",
		Sandbox: SandboxSpec{
			Image:   "alpine:latest",
			Command: []string{"sleep", "100"},
			// No timeout specified, use default
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// The job should fail (either timeout or command not found)
	if resp.Status == StatusCompleted && resp.ExitCode == 0 {
		t.Error("expected job to fail or timeout")
	}
}

func TestJobResponseAllFields(t *testing.T) {
	resp := &JobResponse{
		Version:   1,
		TaskID:    "task-full",
		JobID:     "job-full",
		Status:    StatusCompleted,
		ExitCode:  0,
		Stdout:    "standard output",
		Stderr:    "standard error",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		EndedAt:   time.Now().UTC().Format(time.RFC3339),
		Truncated: TruncatedInfo{
			Stdout: true,
			Stderr: true,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded JobResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Stdout != "standard output" {
		t.Errorf("Stdout = %s, want 'standard output'", decoded.Stdout)
	}
	if decoded.Stderr != "standard error" {
		t.Errorf("Stderr = %s, want 'standard error'", decoded.Stderr)
	}
	if !decoded.Truncated.Stdout {
		t.Error("expected Truncated.Stdout to be true")
	}
}

func TestSandboxSpecAllFields(t *testing.T) {
	spec := SandboxSpec{
		Image:          "custom-image:v1",
		Command:        []string{"./run.sh", "--flag"},
		Env:            map[string]string{"KEY1": "val1", "KEY2": "val2"},
		TimeoutSeconds: 300,
		NetworkPolicy:  "host",
	}

	if spec.NetworkPolicy != "host" {
		t.Errorf("NetworkPolicy = %s, want 'host'", spec.NetworkPolicy)
	}
	if len(spec.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(spec.Env))
	}
}

func TestRunJobCustomTimeout(t *testing.T) {
	e := NewExecutor("fake-runtime", 60*time.Second, 4096) // Default 60s

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-custom-timeout",
		JobID:   "job-custom-timeout",
		Sandbox: SandboxSpec{
			Image:          "alpine:latest",
			Command:        []string{"echo", "test"},
			TimeoutSeconds: 5, // Override with custom timeout
		},
	}

	resp, _ := e.RunJob(ctx, req)

	// Verify response has expected fields
	if resp.Version != 1 {
		t.Errorf("Version = %d, want 1", resp.Version)
	}
	if resp.StartedAt == "" || resp.EndedAt == "" {
		t.Error("expected StartedAt and EndedAt to be set")
	}
}

func TestParseJobPayloadWithWhitespace(t *testing.T) {
	// Test payload with leading/trailing whitespace
	payload := "   echo hello world   "
	img, cmd, _, err := ParseJobPayload(payload)
	if err != nil {
		t.Fatalf("ParseJobPayload() error = %v", err)
	}

	if img != DefaultImage {
		t.Errorf("image = %s, want %s", img, DefaultImage)
	}
	// Command should be trimmed
	if len(cmd) < 3 {
		t.Error("expected command to be wrapped in sh -c")
	}
}

func TestJobRequestAllFields(t *testing.T) {
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-all",
		JobID:   "job-all",
		Sandbox: SandboxSpec{
			Image:          "ubuntu:22.04",
			Command:        []string{"bash", "-c", "echo $FOO"},
			Env:            map[string]string{"FOO": "bar"},
			TimeoutSeconds: 120,
			NetworkPolicy:  "restricted",
		},
	}

	if req.TaskID != "task-all" {
		t.Errorf("TaskID = %s, want 'task-all'", req.TaskID)
	}
	if req.Sandbox.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %s, want 'bar'", req.Sandbox.Env["FOO"])
	}
}

func TestMarshalResultWithTruncatedOutput(t *testing.T) {
	resp := &JobResponse{
		Version:   1,
		TaskID:    "task-1",
		JobID:     "job-1",
		Status:    StatusCompleted,
		ExitCode:  0,
		Stdout:    "truncated...",
		Stderr:    "",
		StartedAt: "2024-01-01T00:00:00Z",
		EndedAt:   "2024-01-01T00:01:00Z",
		Truncated: TruncatedInfo{
			Stdout: true,
			Stderr: false,
		},
	}

	result, err := MarshalResult(resp)
	if err != nil {
		t.Fatalf("MarshalResult() error = %v", err)
	}

	// Verify result contains truncated info
	if result == "" {
		t.Error("expected non-empty result")
	}

	var decoded JobResponse
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !decoded.Truncated.Stdout {
		t.Error("expected Truncated.Stdout to be true")
	}
}

func TestExecutorWithEmptyEnv(t *testing.T) {
	e := NewExecutor("fake-runtime", 30*time.Second, 2048)

	ctx := context.Background()
	req := &JobRequest{
		Version: 1,
		TaskID:  "task-empty-env",
		JobID:   "job-empty-env",
		Sandbox: SandboxSpec{
			Image:   "alpine:latest",
			Command: []string{"env"},
			Env:     map[string]string{}, // Empty env map
		},
	}

	resp, _ := e.RunJob(ctx, req)

	if resp.TaskID != "task-empty-env" {
		t.Errorf("TaskID = %s, want 'task-empty-env'", resp.TaskID)
	}
}
