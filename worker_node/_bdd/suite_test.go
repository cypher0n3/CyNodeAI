// Package bdd runs the worker_node Godog BDD suite.
package bdd

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

func featurePath() string {
	for _, p := range []string{"features/worker_node", "../../features/worker_node", "../features/worker_node"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return "../../features/worker_node"
}

func TestWorkerNodeBDD(t *testing.T) {
	state := &workerTestState{}
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeWorkerNodeSuite(sc, state)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{featurePath()},
			TestingT: t,
			Tags:     "~@wip",
		},
	}
	if suite.Run() != 0 {
		t.Fatal("worker_node BDD suite failed")
	}
}
