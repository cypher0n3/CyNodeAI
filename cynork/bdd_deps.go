// Package cynork provides the cynork CLI.
//
// This file ensures that BDD (Behavior-Driven Development) test dependencies
// are preserved in go.mod when go mod tidy is run. The BDD test code in _bdd/
// imports godog; this file keeps that dependency in the module.
package main

import (
	_ "github.com/cucumber/godog"
)
