// Package bdd – additional step definitions for cynork BDD suite (Task 3).
// Covers: model, project, dispatch, auth, thread, TUI, task, skills, nodes,
// prefs, connect, session, shell, and chat feature files.
// Streaming/in-flight steps and PTY-only steps are marked godog.ErrPending.
package bdd

import (
	"github.com/cucumber/godog"
)

// InitializeCynorkSuiteExtra registers additional step definitions.
// Called at the end of InitializeCynorkSuite.
func InitializeCynorkSuiteExtra(sc *godog.ScenarioContext, state *cynorkState) {
	registerCynorkExtraMockGateway(sc, state)
	registerCynorkExtraTUIDeferred(sc, state)
	registerCynorkExtraScrollbackConfig(sc, state)
}
