package secretutil

import "testing"

func TestTokenEquals(t *testing.T) {
	if !TokenEquals("same", "same") {
		t.Error("same strings should match")
	}
	if TokenEquals("a", "b") {
		t.Error("different strings should not match")
	}
	if TokenEquals("secret", "secres") {
		t.Error("prefix should not match")
	}
	if TokenEquals("", "x") || TokenEquals("x", "") {
		t.Error("empty vs non-empty should not match")
	}
	if !TokenEquals("", "") {
		t.Error("two empty strings match")
	}
}

func TestRunWithSecret_invokesCallback(t *testing.T) {
	var called bool
	RunWithSecret(func() {
		called = true
	})
	if !called {
		t.Error("RunWithSecret did not invoke the callback")
	}
}
