package modelpresets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTripPreservesGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	original := "version = 1\n\n; shared\n[*]\nc = 4096\n\n[old]\nmodel = /old.gguf\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	ctx := context.Background()
	if err := store.Put(ctx, Section{Name: "old", Values: map[string]string{"LLAMA_ARG_MODEL": "/new.gguf", "ctx-size": "8192"}}, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "; shared\n[*]\nc = 4096") || !strings.Contains(content, "model = /new.gguf") {
		t.Fatalf("content = %q", content)
	}
	sections, err := store.List(ctx)
	if err != nil || len(sections) != 1 || sections[0].Name != "old" {
		t.Fatalf("List() = %#v, %v", sections, err)
	}
	if err := store.Delete(ctx, "*"); err == nil {
		t.Fatal("Delete(*) succeeded")
	}
}

func TestStoreNormalizesTrailingCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	if err := os.WriteFile(path, []byte("version = 1\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if err := store.Put(context.Background(), Section{Name: "demo", Values: map[string]string{"model": "/demo.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("content contains carriage return: %q", data)
	}
}

func TestStoreCRUD(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
	ctx := context.Background()
	section := Section{Name: "demo", Values: map[string]string{"model": "/demo.gguf"}}
	if err := store.Put(ctx, section, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "demo"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsINIInjection(t *testing.T) {
	tests := []Section{
		{Name: "demo", Values: map[string]string{"model\n[other]": "/demo.gguf"}},
		{Name: "demo", Values: map[string]string{"[model]": "/demo.gguf"}},
		{Name: "demo", Values: map[string]string{"#model": "/demo.gguf"}},
		{Name: "demo", Values: map[string]string{"model": "/demo.gguf\n[other]\nmodel = /other.gguf"}},
		{Name: "demo", Values: map[string]string{"model": "/demo.gguf\rmodel = /other.gguf"}},
	}
	for i, section := range tests {
		store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
		if err := store.Put(context.Background(), section, true); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d: Put() error = %v", i, err)
		}
	}
}

func TestStoreRejectsDuplicateCanonicalKeys(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
	section := Section{Name: "demo", Values: map[string]string{
		"model":           "/first.gguf",
		"LLAMA_ARG_MODEL": "/second.gguf",
	}}
	if err := store.Put(context.Background(), section, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Put() error = %v", err)
	}
}

func TestStoreUsesUpstreamKeyCasing(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
	ctx := context.Background()
	if err := store.Put(ctx, Section{Name: "demo", Values: map[string]string{"Model": "/demo.gguf"}}, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mixed-case Put() error = %v", err)
	}
	if err := store.Put(ctx, Section{Name: "demo", Values: map[string]string{"LLAMA_ARG_model": "/demo.gguf"}}, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mixed-case environment Put() error = %v", err)
	}
	if err := store.Put(ctx, Section{Name: "demo", Values: map[string]string{"LLAMA_ARG_MODEL": "/demo.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	section, err := store.Get(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if section.Values["model"] != "/demo.gguf" {
		t.Fatalf("values = %#v", section.Values)
	}
}

func TestStoreCanonicalizesLegacyModelKeys(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
	section := Section{Name: "demo", Values: map[string]string{
		"models_dir": "/models", "models_preset": "template", "models_max": "2",
	}}
	if err := store.Put(context.Background(), section, true); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Values["models-dir"] != "/models" || got.Values["models-preset"] != "template" || got.Values["models-max"] != "2" {
		t.Fatalf("values = %#v", got.Values)
	}
}

func TestStoreRejectsMixedCaseFromHandEditedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	if err := os.WriteFile(path, []byte("[demo]\nModel = /demo.gguf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).List(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("List() error = %v", err)
	}
}

func TestStoreRejectsDuplicateSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	content := "[demo]\nmodel = /first.gguf\n\n[demo]\nctx-size = 4096\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(path).List(context.Background()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("List() error = %v", err)
	}
}

func TestStoreRejectsInvalidSectionsFromHandEditedFile(t *testing.T) {
	for _, name := range []string{" ", "bad=name", "bad#name"} {
		path := filepath.Join(t.TempDir(), "models.ini")
		if err := os.WriteFile(path, []byte("["+name+"]\nmodel = /demo.gguf\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewStore(path).List(context.Background()); !errors.Is(err, ErrInvalid) {
			t.Fatalf("section %q: List() error = %v", name, err)
		}
	}
}

func TestStoreRejectsMalformedINI(t *testing.T) {
	for _, content := range []string{"[demo\nmodel = /demo.gguf\n", "[demo]\nmodel /demo.gguf\n"} {
		path := filepath.Join(t.TempDir(), "models.ini")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewStore(path).List(context.Background()); !errors.Is(err, ErrInvalid) {
			t.Fatalf("content %q: List() error = %v", content, err)
		}
	}
}

func TestStoreStripsInlineComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	content := "[demo]\nmodel = /demo.gguf ; local model\nurl = https://example.test/model#fragment\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	section, err := NewStore(path).Get(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if section.Values["model"] != "/demo.gguf" || section.Values["url"] != "https://example.test/model#fragment" {
		t.Fatalf("values = %#v", section.Values)
	}
}

func TestRemoveSectionPreservesTrailingComments(t *testing.T) {
	content := "[remove]\nmodel = /demo.gguf\n; next section comment\n\n[keep]\nmodel = /keep.gguf\n"
	want := "; next section comment\n\n[keep]\nmodel = /keep.gguf\n"
	if got := removeSection(content, "remove"); got != want {
		t.Fatalf("removeSection() = %q, want %q", got, want)
	}
}

func TestStoreDeleteMissingIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.ini")
	original := "version = 1\n\n[kept]\nmodel = /kept.gguf\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if err := store.Delete(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("content changed: %q", data)
	}
}

func TestStoreDeleteValidatesName(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "models.ini"))
	for _, name := range []string{"", "bad]name", "bad\nname", "*"} {
		if err := store.Delete(context.Background(), name); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Delete(%q) error = %v", name, err)
		}
	}
}
