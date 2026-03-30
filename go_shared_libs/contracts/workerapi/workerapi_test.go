package workerapi

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDefaultSandboxSpec(t *testing.T) {
	s := DefaultSandboxSpec()
	if s.Image != DefaultImage || s.Command != nil || s.Env != nil {
		t.Fatalf("DefaultSandboxSpec = %+v", s)
	}
}

func TestExitCodePtr(t *testing.T) {
	p := ExitCodePtr(42)
	if p == nil || *p != 42 {
		t.Fatalf("ExitCodePtr(42) = %v", p)
	}
}

func TestValidateRequest(t *testing.T) {
	if err := ValidateRequest(nil); err == nil {
		t.Fatal("nil request: want error")
	} else {
		var rv *RequestValidationError
		if !errors.As(err, &rv) || rv.Reason == "" {
			t.Fatalf("want RequestValidationError, got %v", err)
		}
	}
	req := &RunJobRequest{Version: 1, Sandbox: SandboxSpec{Image: "x"}}
	if err := ValidateRequest(req); err == nil {
		t.Fatal("empty command: want error")
	}
	req.Sandbox.Command = []string{"sh", "-c", "true"}
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("valid: %v", err)
	}
	req2 := &RunJobRequest{Version: 1, Sandbox: SandboxSpec{Image: "img", JobSpecJSON: "{}"}}
	if err := ValidateRequest(req2); err != nil {
		t.Fatalf("SBA without command: %v", err)
	}
}

func TestRequestValidationError_Error(t *testing.T) {
	e := &RequestValidationError{Reason: "bad"}
	if e.Error() != "bad" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestRunJobResponse_roundTripDiagnostics(t *testing.T) {
	resp := RunJobResponse{
		Version: 1, TaskID: "t", JobID: "j", Status: StatusFailed,
		ExitCode: ExitCodePtr(1),
		RunDiagnostics: &RunDiagnostics{
			Runtime: "podman", RuntimeArgv: []string{"podman", "run"},
			JobDir: "/tmp/job", Image: "img", ContainerStarted: true,
		},
		Truncated: TruncatedInfo{Stdout: true},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var back RunJobResponse
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.RunDiagnostics == nil || back.RunDiagnostics.Runtime != "podman" {
		t.Fatalf("diagnostics: %+v", back.RunDiagnostics)
	}
	if !back.Truncated.Stdout || back.Truncated.Stderr {
		t.Fatalf("truncated: %+v", back.Truncated)
	}
}

func TestExitCodeZero(t *testing.T) {
	resp := RunJobResponse{
		Version:   1,
		TaskID:    "t1",
		JobID:     "j1",
		Status:    StatusCompleted,
		ExitCode:  ExitCodePtr(0),
		Stdout:    "out",
		Stderr:    "",
		StartedAt: "2020-01-01T00:00:00Z",
		EndedAt:   "2020-01-01T00:00:01Z",
		Truncated: TruncatedInfo{},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["exit_code"]; !ok {
		t.Fatalf("exit_code missing from JSON: %s", data)
	}
	var back RunJobResponse
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.ExitCode == nil || *back.ExitCode != 0 {
		t.Fatalf("unmarshal ExitCode = %v", back.ExitCode)
	}
}
