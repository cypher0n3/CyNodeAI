// Package bdd runs the agents Godog BDD suite.
// Feature files live under repo features/agents/.
package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func featurePath() string {
	for _, p := range []string{"features/agents", "../../features/agents", "../features/agents"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return "../../features/agents"
}

func TestAgentsBDD(t *testing.T) {
	state := &agentsTestState{}
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeAgentsSuite(sc, state)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{featurePath()},
			TestingT: t,
			Tags:     "~@wip",
		},
	}
	if suite.Run() != 0 {
		t.Fatal("agents BDD suite failed")
	}
}
