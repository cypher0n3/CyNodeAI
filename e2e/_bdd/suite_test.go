// Package bdd runs the e2e Godog BDD suite.
// Feature files live under repo features/e2e/.
// Scenarios call the real gateway when E2E_GATEWAY_URL is set and gateway is up; otherwise they skip.
package bdd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
)

func featurePath() string {
	for _, p := range []string{"features/e2e", "../../features/e2e", "../features/e2e"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return "../../features/e2e"
}

func TestE2EBDD(t *testing.T) {
	state := &e2eState{}
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeE2ESuite(sc, state)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{featurePath()},
			TestingT: t,
			Tags:     "~@wip",
		},
	}
	if suite.Run() != 0 {
		t.Fatal("e2e BDD suite failed")
	}
}

func TestFeaturePathResolves(t *testing.T) {
	path := featurePath()
	if path == "" {
		t.Fatal("featurePath returned empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("featurePath abs: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("featurePath %s not found: %v", abs, err)
	}
}
