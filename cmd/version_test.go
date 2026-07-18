package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"llamarig/internal/buildinfo"
)

func TestVersionCommand(t *testing.T) {
	oldVersion, oldCommit, oldCommitTime := buildinfo.Version, buildinfo.Commit, buildinfo.CommitTime
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.CommitTime = oldVersion, oldCommit, oldCommitTime
	})
	buildinfo.Version, buildinfo.Commit, buildinfo.CommitTime = "v0.1.0-alpha.1", "abc123", "2026-07-18T12:00:00Z"

	root := NewRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"version", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got["version"] != buildinfo.Version || got["commit"] != buildinfo.Commit || got["commit_time"] != buildinfo.CommitTime {
		t.Fatalf("version JSON = %#v", got)
	}
}

func TestRootVersionFlag(t *testing.T) {
	root := NewRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), buildinfo.Version) {
		t.Fatalf("version output = %q", out.String())
	}
}
