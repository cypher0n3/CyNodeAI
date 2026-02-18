package dispatcher

import (
	"errors"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
)

func TestParseSandboxSpec(t *testing.T) {
	tests := []struct {
		name    string
		payload *string
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", strPtr(""), true},
		{"invalid json", strPtr("{"), true},
		{"missing command", strPtr(`{"image":"alpine"}`), true},
		{"valid", strPtr(`{"command":["echo","hi"],"image":"alpine"}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSandboxSpec(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSandboxSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	spec, err := ParseSandboxSpec(strPtr(`{"command":["a","b"],"image":"img","timeout_seconds":30}`))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "img" || spec.TimeoutSeconds != 30 || len(spec.Command) != 2 {
		t.Errorf("spec: %+v", spec)
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

func strPtr(s string) *string {
	return &s
}
