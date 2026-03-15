package secretutil

import "testing"

func TestRunWithSecret_invokesCallback(t *testing.T) {
	var called bool
	RunWithSecret(func() {
		called = true
	})
	if !called {
		t.Error("RunWithSecret did not invoke the callback")
	}
}
