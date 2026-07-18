package ui

import (
	"strings"
	"testing"
)

func TestFlowWrapsAtWidth(t *testing.T) {
	got := Flow(6, 1, []string{"aaa", "bb", "cc"})
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || strings.TrimRight(lines[0], " ") != "aaa bb" || strings.TrimRight(lines[1], " ") != "cc" {
		t.Fatalf("Flow() = %q", got)
	}
}

func TestFlowRejectsNonPositiveWidth(t *testing.T) {
	if got := Flow(0, 2, []string{"a"}); got != "" {
		t.Fatalf("Flow() = %q", got)
	}
}
