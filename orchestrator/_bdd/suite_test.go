// Package bdd runs the orchestrator Godog BDD suite.
// TestMain starts Postgres via testcontainers when POSTGRES_TEST_DSN is unset (see testmain_test.go).
// Set SKIP_TESTCONTAINERS=1 to run without a DB; scenarios that need the DB will skip.
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
