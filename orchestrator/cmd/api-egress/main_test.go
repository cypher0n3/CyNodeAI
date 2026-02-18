package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Unsetenv("TEST_AE_ENV")
	if getEnv("TEST_AE_ENV", "def") != "def" {
		t.Error("default")
	}
	os.Setenv("TEST_AE_ENV", "val")
	defer os.Unsetenv("TEST_AE_ENV")
	if getEnv("TEST_AE_ENV", "def") != "val" {
		t.Error("from env")
	}
}
