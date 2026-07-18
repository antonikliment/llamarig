package modelcatalog

import (
	"path"
	"regexp"
	"strings"
)

var quantPattern = regexp.MustCompile(`(?i)(?:^|[-_/])((?:UD-)?(?:Q[0-9]_K(?:_[A-Z]+)?|IQ[0-9]_[A-Z0-9]+|Q[0-9]_[0-9]|Q[0-9]|BF16|F16|F32))(?:\.gguf|[-_/])`)

func InferQuant(filename string) string {
	name := path.Base(filename)
	matches := quantPattern.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	return strings.ToUpper(matches[1])
}
