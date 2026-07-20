package cmd

import (
	"context"
	"testing"

	"llamarig/core/setup"
)

func TestBareCommandRunsSetupBeforeTUI(t *testing.T) {
	called := false
	orig := setupEnsure
	setupEnsure = func(context.Context) error {
		called = true
		// Return ErrCancelled so the TUI never launches during the test; the
		// bare path should still exit cleanly.
		return setup.ErrCancelled
	}
	t.Cleanup(func() { setupEnsure = orig })

	root := NewRootCommand()
	root.SetArgs(nil)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if !called {
		t.Fatal("bare command did not invoke setup before launching the TUI")
	}
}
