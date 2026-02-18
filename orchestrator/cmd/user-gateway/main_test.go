package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_UG_ENV")
	if getEnv("TEST_UG_ENV", "def") != "def" {
		t.Error("default")
	}
	os.Setenv("TEST_UG_ENV", "val")
	defer os.Unsetenv("TEST_UG_ENV")
	if getEnv("TEST_UG_ENV", "def") != "val" {
		t.Error("from env")
	}
}
