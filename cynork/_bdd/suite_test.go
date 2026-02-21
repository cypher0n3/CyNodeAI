// Package bdd runs the cynork CLI Godog BDD suite.
package bdd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
)

func featurePath() string {
	for _, p := range []string{"features/cynork", "../../features/cynork", "../features/cynork"} {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return "../../features/cynork"
}

func TestCynorkBDD(t *testing.T) {
	state := &cynorkState{}
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeCynorkSuite(sc, state)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{featurePath()},
			TestingT: t,
			Tags:     "~@wip",
			NoColors: true,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cynork BDD suite failed")
	}
}

// Ensure feature path resolves when run from repo root (just test-bdd).
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
