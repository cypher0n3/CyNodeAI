package bdd

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/cypher0n3/cynodeai/cynork/internal/tui"
)

// tuiBugfixAgentStreaming mirrors "agent streaming" in cynork_tui_bugfixes.feature (package scope keeps steps.go under line cap).
var tuiBugfixAgentStreaming bool

func registerCynorkTuiBugfixes(sc *godog.ScenarioContext, state *cynorkState) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		tuiBugfixAgentStreaming = false
		return ctx, nil
	})
	sc.Step(`^the ensure thread scrollback line for prior "([^"]*)" after "([^"]*)" resume "([^"]*)" contains "([^"]*)" and not "([^"]*)"$`, func(_ context.Context, prior, after, resume, wantSub, notWant string) error {
		line := tui.EnsureThreadScrollbackSystemLine(prior, after, resume)
		if !strings.Contains(line, wantSub) {
			return fmt.Errorf("expected %q in line %q", wantSub, line)
		}
		if strings.Contains(line, notWant) {
			return fmt.Errorf("expected %q not to appear in line %q", notWant, line)
		}
		return nil
	})

	sc.Step(`^loading is true$`, func(_ context.Context) error {
		state.mu.Lock()
		state.tuiBugfixLoading = true
		state.mu.Unlock()
		return nil
	})

	sc.Step(`^agent streaming is true$`, func(_ context.Context) error {
		tuiBugfixAgentStreaming = true
		return nil
	})

	sc.Step(`^enter is not blocked for composer input "([^"]*)"$`, func(_ context.Context, input string) error {
		state.mu.Lock()
		loading := state.tuiBugfixLoading
		state.mu.Unlock()
		if tui.EnterBlockedWhileLoading(loading, tuiBugfixAgentStreaming, input) {
			return fmt.Errorf("expected %q to be accepted while loading=%v streaming=%v", input, loading, tuiBugfixAgentStreaming)
		}
		return nil
	})

	sc.Step(`^enter is blocked for composer input "([^"]*)"$`, func(_ context.Context, input string) error {
		state.mu.Lock()
		loading := state.tuiBugfixLoading
		state.mu.Unlock()
		if !tui.EnterBlockedWhileLoading(loading, tuiBugfixAgentStreaming, input) {
			return fmt.Errorf("expected %q to be blocked while loading=%v streaming=%v", input, loading, tuiBugfixAgentStreaming)
		}
		return nil
	})
}
