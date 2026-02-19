// Package bdd runs the orchestrator Godog BDD suite.
// Run with POSTGRES_TEST_DSN set for integration; scenarios are skipped otherwise.
package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// featurePath returns the path to features/orchestrator.
// When running from orchestrator/_bdd, repo features are at ../../features/orchestrator.
func featurePath() string {
	for _, p := range []string{"features/orchestrator", "../../features/orchestrator", "../features/orchestrator"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return "../../features/orchestrator"
}

func TestOrchestratorBDD(t *testing.T) {
	state := &testState{}
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeOrchestratorSuite(sc, state)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:   []string{featurePath()},
			TestingT: t,
			Tags:    "~@wip",
		},
	}
	if suite.Run() != 0 {
		t.Fatal("orchestrator BDD suite failed")
	}
}
