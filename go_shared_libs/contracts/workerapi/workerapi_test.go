package workerapi

import (
	"encoding/json"
	"testing"
)

func TestDefaultSandboxSpec(t *testing.T) {
	spec := DefaultSandboxSpec()
	if spec.Image != DefaultImage {
		t.Errorf("DefaultSandboxSpec().Image = %q, want %q", spec.Image, DefaultImage)
	}
	if spec.Command != nil {
		t.Errorf("DefaultSandboxSpec().Command should be nil, got %v", spec.Command)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *RunJobRequest
		wantErr bool
	}{
		{"nil request", nil, true},
		{"empty command", &RunJobRequest{Sandbox: SandboxSpec{Command: []string{}}}, true},
		{"nil command", &RunJobRequest{Sandbox: SandboxSpec{Command: nil}}, true},
		{"valid", &RunJobRequest{Sandbox: SandboxSpec{Command: []string{"echo", "hi"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequestValidationError_Error(t *testing.T) {
	e := &RequestValidationError{Reason: "test reason"}
	if e.Error() != "test reason" {
		t.Errorf("Error() = %q, want %q", e.Error(), "test reason")
	}
}

func TestRunJobRequestResponseJSON(t *testing.T) {
	req := RunJobRequest{
		Version: 1,
		TaskID:  "task-1",
		JobID:   "job-1",
		Sandbox: SandboxSpec{
			Image:   DefaultImage,
			Command: []string{"echo", "hello"},
			Env:     map[string]string{"K": "V"},
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded RunJobRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Version != req.Version || decoded.TaskID != req.TaskID || decoded.Sandbox.Image != req.Sandbox.Image {
		t.Errorf("round-trip mismatch: got %+v", decoded)
	}
}
