package buildinfo

// Populated with -ldflags at release build time.
var Version, Commit, CommitTime = "dev", "unknown", "unknown"
