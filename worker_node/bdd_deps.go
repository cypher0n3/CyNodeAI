// This file ensures BDD (Behavior-Driven Development) test dependencies are
// properly imported. It contains build tags and dependency imports for BDD
// testing infrastructure. This file should contain only dependency imports
// and build tags for BDD tests.

// Package worker_node provides the worker_node BDD dependencies.
//
// This file ensures that BDD (Behavior-Driven Development) test dependencies
// are preserved in go.mod even when go mod tidy is run without the bdd build tag.
//
// Problem:
// The BDD test code in _bdd/ uses the //go:build bdd build tag, which means
// those files are only included when building with the bdd tag. When go mod tidy
// runs without this tag, it doesn't see the imports from _bdd/ files and removes
// the godog dependencies from go.mod.
//
// Solution:
// This file imports the BDD dependencies (godog) as part of the main package.
// By having these imports in a file that's always included in dependency analysis,
// `go mod tidy` will preserve the dependencies even when run without the bdd build tag.
// This ensures the dependencies remain in go.mod as long as this file exists.
//
// Usage:
// - With this file in place, go mod tidy works correctly without any additional flags
// - The Makefile 'tidy' target is available for convenience but not required
// - The actual BDD test code is in _bdd/ directory with //go:build bdd tags

package worker_node

import (
	_ "github.com/cucumber/godog"
	_ "github.com/cucumber/godog/colors"
)
