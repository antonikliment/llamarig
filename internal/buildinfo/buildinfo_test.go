package buildinfo

import "testing"

func TestDefaultsCanBeInjected(t *testing.T) {
	oldVersion, oldCommit, oldCommitTime := Version, Commit, CommitTime
	t.Cleanup(func() { Version, Commit, CommitTime = oldVersion, oldCommit, oldCommitTime })
	Version, Commit, CommitTime = "v0.1.0-alpha.1", "abc123", "2026-07-18T12:00:00Z"
	if Version != "v0.1.0-alpha.1" || Commit != "abc123" || CommitTime != "2026-07-18T12:00:00Z" {
		t.Fatal("build metadata injection failed")
	}
}
