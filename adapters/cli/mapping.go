package cli

import (
	"strings"
)

func list(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}

func suffix(value string) string {
	if value == "" {
		return ""
	}
	return ": " + value
}
