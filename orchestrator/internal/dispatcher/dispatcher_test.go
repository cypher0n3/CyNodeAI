package dispatcher

import (
	"errors"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

var (
	parseSpecEmptyStr            = ""
	parseSpecInvalidJSONBrace    = "{"
	parseSpecMissingCommand      = `{"image":"alpine"}`
	parseSpecValidEcho           = `{"command":["echo","hi"],"image":"alpine"}`
	parseSpecSBAJobSpecJSONNoCmd = `{"job_spec_json":"{\"protocol_version\":\"1.0\",\"job_id\":\"j1\",\"task_id\":\"t1\",\"constraints\":{\"max_runtime_seconds\":60,\"max_output_bytes\":1024},\"steps\":[]}"}`
	parseSpecWithTimeout         = `{"command":["a","b"],"image":"img","timeout_seconds":30}`
	parseSpecUseInference        = `{"command":["x"],"use_inference":true}`
)

func TestParseSandboxSpec(t *testing.T) {
	tests := []struct {
		name    string
		payload *string
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", &parseSpecEmptyStr, true},
		{"invalid json", &parseSpecInvalidJSONBrace, true},
		{"missing command", &parseSpecMissingCommand, true},
		{"valid", &parseSpecValidEcho, false},
		{"sba job_spec_json no command", &parseSpecSBAJobSpecJSONNoCmd, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSandboxSpec(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSandboxSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	spec, err := ParseSandboxSpec(&parseSpecWithTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "img" || spec.TimeoutSeconds != 30 || len(spec.Command) != 2 {
		t.Errorf("spec: %+v", spec)
	}

	spec2, err := ParseSandboxSpec(&parseSpecUseInference)
	if err != nil {
		t.Fatal(err)
	}
	if !spec2.UseInference {
		t.Error("use_inference should be true")
	}

	spec3, err := ParseSandboxSpec(&parseSpecSBAJobSpecJSONNoCmd)
	if err != nil {
		t.Fatal(err)
	}
	if spec3.JobSpecJSON == "" || spec3.Image != DefaultSBARunnerImage {
		t.Errorf("SBA spec: JobSpecJSON=%q Image=%q", spec3.JobSpecJSON, spec3.Image)
	}
}

func TestMarshalDispatchError(t *testing.T) {
	s := MarshalDispatchError(errors.New("test err"))
	if s == "" {
		t.Error("MarshalDispatchError should not be empty")
	}
	if len(s) < 10 {
		t.Errorf("expected JSON: %s", s)
	}
}

func TestSummarizeResult(t *testing.T) {
	tests := []struct {
		name string
		resp *workerapi.RunJobResponse
		want string
	}{
		{"nil", nil, ""},
		{"failed", &workerapi.RunJobResponse{Status: workerapi.StatusFailed}, "job failed"},
		{"stdout", &workerapi.RunJobResponse{Status: workerapi.StatusCompleted, Stdout: "hello\n"}, "hello"},
		{"stderr", &workerapi.RunJobResponse{Status: workerapi.StatusCompleted, Stderr: "err"}, "err"},
		{"completed", &workerapi.RunJobResponse{Status: workerapi.StatusCompleted}, "completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SummarizeResult(tt.resp)
			if got != tt.want {
				t.Errorf("SummarizeResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateOneLine(t *testing.T) {
	if TruncateOneLine("a\nb", 10) != "a" {
		t.Error("should stop at newline")
	}
	if TruncateOneLine("abcdefghij", 5) != "abcde" {
		t.Error("should truncate to maxLen")
	}
	if TruncateOneLine("ab", 10) != "ab" {
		t.Error("short line unchanged")
	}
}
