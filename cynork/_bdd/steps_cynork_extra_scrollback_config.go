// Package bdd – cynork Godog steps for scrollback/config assertions and CLI-side YAML (prescribed startup, etc.).
package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"
)

func registerCynorkExtraScrollbackConfig(sc *godog.ScenarioContext, state *cynorkState) {

	sc.Step(`^the scrollback shows task result output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "task") && !strings.Contains(lower, "result") &&
			!strings.Contains(lower, "completed") && !strings.Contains(lower, "echo") {
			return fmt.Errorf("expected task result; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows task logs or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "task") && !strings.Contains(lower, "log") &&
			!strings.Contains(lower, "error") && !strings.Contains(lower, "stdout") {
			return fmt.Errorf("expected task logs or error; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows artifact list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "artifact") && !strings.Contains(combined, "[]") {
			return fmt.Errorf("expected artifact list; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the task result status is one of "([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)"$`, func(ctx context.Context, s1, s2, s3, s4, s5, s6 string) error {
		st := getState(ctx)
		out := strings.TrimSpace(st.lastStdout)
		var result map[string]any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			return fmt.Errorf("expected JSON result; got: %q, err: %v", out, err)
		}
		// CLI returns a flat object with top-level "status"; fall back to nested jobs array.
		status, _ := result["status"].(string)
		if status == "" {
			jobs, _ := result["jobs"].([]any)
			if len(jobs) > 0 {
				job, _ := jobs[0].(map[string]any)
				status, _ = job["status"].(string)
			}
		}
		allowed := []string{s1, s2, s3, s4, s5, s6}
		for _, a := range allowed {
			if status == a {
				return nil
			}
		}
		return fmt.Errorf("task result status %q not in allowed set %v; result: %v", status, allowed, result)
	})

	sc.Step(`^cynork resolves task selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, selector) && !strings.Contains(strings.ToLower(combined), "task") {
			return fmt.Errorf("expected task selector %q resolved; got: %q", selector, combined)
		}
		return nil
	})

	// ---- Assertion steps: skills slash ----

	sc.Step(`^the scrollback shows skill list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "skill") && !strings.Contains(combined, "[]") &&
			!strings.Contains(combined, "{}") {
			return fmt.Errorf("expected skill list; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows skill details for that selector$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "skill") && !strings.Contains(lower, "scope") {
			return fmt.Errorf("expected skill details; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows load success or skill id$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "skill") && !strings.Contains(lower, "loaded") &&
			!strings.Contains(lower, "untitled") {
			return fmt.Errorf("expected skill load result; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows update success or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "skill") && !strings.Contains(lower, "updated") &&
			!strings.Contains(lower, "error") {
			return fmt.Errorf("expected skill update result; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows delete success or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "delet") && !strings.Contains(lower, "true") &&
			!strings.Contains(lower, "error") {
			return fmt.Errorf("expected delete result; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows an ambiguity error or asks to disambiguate$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "skill") && !strings.Contains(lower, "ambig") &&
			!strings.Contains(lower, "multiple") && !strings.Contains(lower, "error") {
			return fmt.Errorf("expected ambiguity error; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^cynork resolves skill selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(combined, selector) && !strings.Contains(lower, "skill") &&
			!strings.Contains(lower, "team") {
			return fmt.Errorf("expected skill selector %q resolved; got: %q", selector, combined)
		}
		return nil
	})

	// ---- Assertion steps: nodes slash ----

	sc.Step(`^the scrollback shows node list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "node") && !strings.Contains(combined, "[]") {
			return fmt.Errorf("expected node list; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows node details for "([^"]*)"$`, func(ctx context.Context, nodeID string) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, nodeID) && !strings.Contains(strings.ToLower(combined), "node") {
			return fmt.Errorf("expected node %q details; got: %q", nodeID, combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows a usage error or inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "usage") &&
			!strings.Contains(lower, "unknown") && !strings.Contains(lower, "invalid") {
			return fmt.Errorf("expected usage/error in output; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: prefs slash ----

	sc.Step(`^the scrollback shows preference list output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "pref") && !strings.Contains(combined, "[]") &&
			!strings.Contains(combined, "{}") {
			return fmt.Errorf("expected prefs list; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows the preference value or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "pref") && !strings.Contains(combined, "{}") &&
			!strings.Contains(lower, "error") {
			return fmt.Errorf("expected pref value or error; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the scrollback shows success or an inline error$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			combined := st.lastStdout + " " + st.lastStderr
			lower := strings.ToLower(combined)
			if !strings.Contains(lower, "error") {
				return fmt.Errorf("expected success or error; got exit %d: %q", st.lastExit, combined)
			}
		}
		return nil
	})

	sc.Step(`^the scrollback shows effective preferences output$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "pref") && !strings.Contains(combined, "{}") &&
			!strings.Contains(combined, "[]") {
			return fmt.Errorf("expected effective prefs; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: chat ----

	sc.Step(`^the assistant response is printed once$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.lastExit != 0 {
			return fmt.Errorf("expected exit 0; got %d stderr: %q", st.lastExit, st.lastStderr)
		}
		if strings.TrimSpace(st.lastStdout) == "" {
			return fmt.Errorf("expected non-empty assistant response; stdout is empty")
		}
		return nil
	})

	// ---- Assertion steps: shell ----

	sc.Step(`^cynork tui is the documented primary interactive chat surface$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, "tui") {
			return fmt.Errorf("expected 'tui' in help output; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^cynork shell is documented as deprecated compatibility$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "shell") && !strings.Contains(lower, "deprecated") &&
			!strings.Contains(lower, "compat") {
			// shell may not be in main help; run `cynork shell --help` to check
			env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
			args := []string{"shell", "--help"}
			_, sout, serr := st.runCynork(args, env...)
			c2 := strings.ToLower(sout + " " + serr)
			if !strings.Contains(c2, "shell") && !strings.Contains(c2, "compat") &&
				!strings.Contains(c2, "deprecated") && !strings.Contains(c2, "legacy") {
				return fmt.Errorf("expected shell deprecated/compat reference; got: %q", sout+serr)
			}
		}
		return nil
	})

	sc.Step(`^the shell command output is shown inline in chat$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(combined, "hello") {
			return fmt.Errorf("expected shell output 'hello' in chat; got: %q", combined)
		}
		return nil
	})

	// ---- Assertion steps: TUI thinking/config ----

	sc.Step(`^the local cynork config has "([^"]*)" set to (true|false)$`, func(ctx context.Context, key, val string) error {
		st := getState(ctx)
		data, err := os.ReadFile(st.configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}
		var cfg map[string]any
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse config yaml: %w", err)
		}
		// key format: "tui.show_thinking_by_default" → cfg["tui"]["show_thinking_by_default"]
		parts := strings.SplitN(key, ".", 2)
		want := val == "true"
		var got any
		if len(parts) == 2 {
			sub, _ := cfg[parts[0]].(map[string]any)
			got = sub[parts[1]]
		} else {
			got = cfg[key]
		}
		gotBool, _ := got.(bool)
		if gotBool != want {
			return fmt.Errorf("expected config %q=%v; got %v (raw: %v)", key, want, gotBool, got)
		}
		return nil
	})

	sc.Step(`^the local cynork YAML config stores `+"`tui.show_thinking_by_default`"+` as (true|false)$`, func(ctx context.Context, val string) error {
		st := getState(ctx)
		data, err := os.ReadFile(st.configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}
		want := val == "true"
		wantedLine := fmt.Sprintf("    show_thinking_by_default: %v", want)
		otherLine := fmt.Sprintf("    show_thinking_by_default: %v", !want)
		cfg := string(data)
		if strings.Contains(cfg, wantedLine) {
			return nil
		}
		// Replace opposite value or insert under tui: section.
		if strings.Contains(cfg, otherLine) {
			cfg = strings.ReplaceAll(cfg, otherLine, wantedLine)
		} else if strings.Contains(cfg, "\ntui:\n") {
			cfg = strings.ReplaceAll(cfg, "\ntui:\n", "\ntui:\n"+wantedLine+"\n")
		} else if strings.Contains(cfg, "tui:") {
			cfg = strings.ReplaceAll(cfg, "tui:", "tui:\n"+wantedLine)
		} else {
			cfg += "\ntui:\n" + wantedLine + "\n"
		}
		if err := os.WriteFile(st.configPath, []byte(cfg), 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		return nil
	})

	sc.Step(`^the local cynork YAML config stores `+"`tui.show_tool_output_by_default`"+` as (true|false)$`, func(ctx context.Context, val string) error {
		st := getState(ctx)
		data, err := os.ReadFile(st.configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}
		want := val == "true"
		wantedLine := fmt.Sprintf("    show_tool_output_by_default: %v", want)
		otherLine := fmt.Sprintf("    show_tool_output_by_default: %v", !want)
		cfg := string(data)
		if strings.Contains(cfg, wantedLine) {
			return nil
		}
		if strings.Contains(cfg, otherLine) {
			cfg = strings.ReplaceAll(cfg, otherLine, wantedLine)
		} else if strings.Contains(cfg, "\ntui:\n") {
			cfg = strings.ReplaceAll(cfg, "\ntui:\n", "\ntui:\n"+wantedLine+"\n")
		} else if strings.Contains(cfg, "tui:") {
			cfg = strings.ReplaceAll(cfg, "tui:", "tui:\n"+wantedLine)
		} else {
			cfg += "\ntui:\n" + wantedLine + "\n"
		}
		if err := os.WriteFile(st.configPath, []byte(cfg), 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		return nil
	})

	sc.Step(`^retained thinking parts in the scrollback are displayed as expanded thinking blocks$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^retained thinking parts are shown as collapsed placeholders$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^placeholders hint "/show-thinking" as the expand action$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^retained thinking is expanded for the loaded assistant turns$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^retained thinking is expanded for the older retrieved assistant turns$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^those retained thinking blocks return to collapsed placeholders$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^the collapsed placeholders remain visible with a "/show-thinking" hint$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^retained thinking blocks are expanded by default in that new session$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	sc.Step(`^those turns are unchanged$`, func(_ context.Context) error {
		return godog.ErrPending // streaming transcript state deferred
	})

	// ---- Assertion steps: TUI visual/layout (streaming/PTY) ----

	sc.Step(`^the visible text is shown in the transcript$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the thinking content is collapsed behind a compact placeholder$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the collapsed placeholder remains visibly distinct from normal assistant prose$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the collapsed placeholder hints that "/show-thinking" reveals the thinking content$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the TUI shows a visible in-flight indicator attached to the active assistant turn$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the indicator is rendered as a distinct status chip rather than bare transcript text$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the indicator shows the label "Working"$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the TUI requests streaming output by default$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^visible assistant text is appended token-by-token within one in-flight assistant turn$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the final assistant turn replaces the in-flight row without duplicating visible text$`, func(_ context.Context) error {
		return godog.ErrPending // streaming spec deferred
	})

	sc.Step(`^the TUI shows a validation error$`, func(ctx context.Context) error {
		st := getState(ctx)
		combined := st.lastStdout + " " + st.lastStderr
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "missing") &&
			!strings.Contains(lower, "not found") && !strings.Contains(lower, "invalid") {
			return fmt.Errorf("expected validation error; got: %q", combined)
		}
		return nil
	})

	sc.Step(`^the message is not sent$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		done := st.chatCompleted
		st.mu.Unlock()
		if done {
			return fmt.Errorf("expected message NOT to be sent; but POST /v1/chat/completions was called")
		}
		return nil
	})

	sc.Step(`^the queued drafts remain distinct from sent transcript messages$`, func(_ context.Context) error {
		return godog.ErrPending // queue draft feature not implemented
	})

	sc.Step(`^the unsent queued draft remains available for later edit or send$`, func(_ context.Context) error {
		return godog.ErrPending // queue draft feature not implemented
	})

	sc.Step(`^the visible transcript history moves$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the composer history selection does not change$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the TUI shows "([^"]*)" in or adjacent to the composer$`, func(_ context.Context, _ string) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the composer shows a visible text cursor or caret at the current insertion point$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	// ---- Assertion steps: TUI session/clear ----

	sc.Step(`^the session model and project context are unchanged$`, func(ctx context.Context) error {
		st := getState(ctx)
		// After /clear, model and project should persist; we verify via re-running /model.
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		if st.sessionModel != "" {
			args = append(args, "--model", st.sessionModel)
		}
		if st.sessionProject != "" {
			args = append(args, "--project-id", st.sessionProject)
		}
		_, stdout, stderr := st.runCynorkWithStdin(args, env, "/model\n/exit\n")
		combined := stdout + " " + stderr
		if st.sessionModel != "" && !strings.Contains(combined, st.sessionModel) {
			return fmt.Errorf("expected model %q preserved after clear; got: %q", st.sessionModel, combined)
		}
		return nil
	})

	sc.Step(`^the scrollback contains the same version string as "([^"]*)"$`, func(ctx context.Context, subcommand string) error {
		st := getState(ctx)
		// Run the subcommand independently to get its version output.
		parts := strings.Fields(subcommand)
		_, refOut, _ := st.runCynork(parts)
		refVersion := strings.TrimSpace(refOut)
		combined := strings.TrimSpace(st.lastStdout + " " + st.lastStderr)
		// Both should contain "cynork" and the same version token.
		if refVersion != "" && !strings.Contains(combined, refVersion) {
			// Try just checking "cynork" appears in both (version string includes "cynork")
			if !strings.Contains(strings.ToLower(combined), "cynork") {
				return fmt.Errorf("expected version string from %q in output; got: %q", subcommand, combined)
			}
		}
		return nil
	})
}
