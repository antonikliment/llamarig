package modelpresets

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInspectSource(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model.gguf")
	if err := os.WriteFile(model, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		section Section
		state   string
	}{
		"model":        {Section{Values: map[string]string{"model": model}}, SourceReady},
		"directory":    {Section{Values: map[string]string{"models-dir": root}}, SourceReady},
		"missing":      {Section{Values: map[string]string{"model": filepath.Join(root, "missing.gguf")}}, SourceUnavailable},
		"wrong type":   {Section{Values: map[string]string{"model": root}}, SourceUnavailable},
		"unconfigured": {Section{Values: map[string]string{}}, SourceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			if got := InspectSource(test.section); got.State != test.state {
				t.Fatalf("InspectSource() = %#v", got)
			}
		})
	}
}

func TestInspectSourceHandlesNilValues(t *testing.T) {
	if status := InspectSource(Section{}); status.State != SourceUnavailable || status.Error == "" {
		t.Fatalf("InspectSource() = %#v", status)
	}
}

func TestFindReferencesSeparatesModelAndDirectory(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "owner", "model.gguf")
	refs := FindReferences([]Section{
		{Name: "exact", Values: map[string]string{"model": model}},
		{Name: "directory", Values: map[string]string{"models-dir": root}},
		{Name: "other", Values: map[string]string{"model": filepath.Join(root, "other.gguf")}},
	}, model)
	if !reflect.DeepEqual(refs.ModelPaths, []string{"exact"}) || !reflect.DeepEqual(refs.ModelDirs, []string{"directory"}) {
		t.Fatalf("FindReferences() = %#v", refs)
	}
}

func TestFindReferencesCanonical(t *testing.T) {
	root := t.TempDir()
	model := filepath.Join(root, "model.gguf")
	sections := CanonicalizeSections([]Section{{Name: "exact", Values: map[string]string{"model": model}}, {Name: "dir", Values: map[string]string{"models-dir": root}}})
	refs := FindReferencesCanonical(sections, model)
	if !reflect.DeepEqual(refs.ModelPaths, []string{"exact"}) || !reflect.DeepEqual(refs.ModelDirs, []string{"dir"}) {
		t.Fatalf("FindReferencesCanonical() = %#v", refs)
	}
}
