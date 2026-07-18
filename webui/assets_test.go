package webui

import (
	"io/fs"
	"regexp"
	"testing"
)

func TestDistIndexReferencesEmbeddedAssets(t *testing.T) {
	data, err := fs.ReadFile(Files, "dist/index.html")
	if err != nil {
		t.Fatalf("read dist/index.html: %v", err)
	}
	index := string(data)
	re := regexp.MustCompile(`/(assets/[^"]+)`)
	matches := re.FindAllStringSubmatch(index, -1)
	if len(matches) == 0 {
		t.Fatal("no built assets found")
	}
	for _, match := range matches {
		if _, err := fs.Stat(Files, "dist/"+match[1]); err != nil {
			t.Fatalf("asset %s not embedded: %v", match[1], err)
		}
	}
}
