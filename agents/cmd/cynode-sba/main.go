// Package main provides the cynode-sba sandbox agent runner binary.
// See docs/tech_specs/cynode_sba.md and docs/requirements/sbagnt.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/agents/internal/sba"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

const (
	defaultJobPath   = "/job/job.json"
	defaultResultPath = "/job/result.json"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("cynode-sba", flag.ContinueOnError)
	stdinStdout := fs.Bool("stdin", false, "Read job from stdin, write result to stdout (or set SBA_STDIN_STDOUT=true)")
	jobPath := fs.String("job", defaultJobPath, "Path to job.json (or set SBA_JOB_PATH)")
	resultPath := fs.String("result", defaultResultPath, "Path to write result.json (or set SBA_RESULT_PATH)")
	workspace := fs.String("workspace", "/workspace", "Workspace root (or set SBA_WORKSPACE)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	applyEnvOverrides(stdinStdout, jobPath, resultPath, workspace)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	var data []byte
	var err error
	if *stdinStdout {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*jobPath)
	}
	if err != nil {
		_ = writeResultTo(*resultPath, os.Stdout, *stdinStdout, failureResult("schema_validation", "failed to read job: "+err.Error()), logger)
		return 1
	}

	spec, err := sbajob.ParseAndValidateJobSpec(data)
	if err != nil {
		msg := err.Error()
		if ve := new(sbajob.ValidationError); errors.As(err, &ve) {
			msg = ve.Error()
		}
		_ = writeResultTo(*resultPath, os.Stdout, *stdinStdout, failureResult("schema_validation", msg), logger)
		return 1
	}

	lifecycle := sba.NewLifecycleClient()
	lifecycle.NotifyInProgress(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(spec.Constraints.MaxRuntimeSeconds)*time.Second)
	defer cancel()

	jobDir := filepath.Dir(*resultPath)
	opts := &sba.RunAgentOptions{JobDir: jobDir}
	if os.Getenv("SBA_USE_MOCK_LLM") == "1" {
		mock := &sba.MockLLM{}
		if resp := os.Getenv("SBA_MOCK_RESPONSES"); resp != "" {
			_ = json.Unmarshal([]byte(resp), &mock.Responses)
		}
		opts.LLM = mock
	}
	result := sba.RunAgent(ctx, spec, *workspace, opts)

	lifecycle.NotifyCompletion(context.Background(), result)

	out := os.Stdout
	if !*stdinStdout {
		out = nil
	}
	if err := writeResultTo(*resultPath, out, *stdinStdout, result, logger); err != nil {
		logger.Error("failed to write result", "error", err)
		return 1
	}

	if result.Status != "success" {
		return 1
	}
	return 0
}

func applyEnvOverrides(stdinStdout *bool, jobPath, resultPath, workspace *string) {
	if v := os.Getenv("SBA_STDIN_STDOUT"); v != "" && strings.EqualFold(v, "true") {
		*stdinStdout = true
	}
	if v := os.Getenv("SBA_JOB_PATH"); v != "" {
		*jobPath = v
	}
	if v := os.Getenv("SBA_RESULT_PATH"); v != "" {
		*resultPath = v
	}
	if v := os.Getenv("SBA_WORKSPACE"); v != "" {
		*workspace = v
	}
}

func failureResult(code, msg string) *sbajob.Result {
	return &sbajob.Result{
		ProtocolVersion: "1.0",
		JobID:           "",
		Status:          "failure",
		FailureCode:     &code,
		FailureMessage:  &msg,
	}
}

// writeResultTo writes result to path (when outPath is not empty) and/or to w (when stdinStdout).
func writeResultTo(resultPath string, w io.Writer, stdinStdout bool, r *sbajob.Result, logger *slog.Logger) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if stdinStdout && w != nil {
		_, err = w.Write(data)
		if err != nil {
			return err
		}
	}
	if !stdinStdout && resultPath != "" {
		return os.WriteFile(resultPath, data, 0o644)
	}
	return nil
}

func writeResultFailure(resultPath, failureCode, failureMessage string, logger *slog.Logger) {
	r := failureResult(failureCode, failureMessage)
	if err := writeResultTo(resultPath, nil, false, r, logger); err != nil {
		logger.Error("failed to write result", "error", err)
	}
}
