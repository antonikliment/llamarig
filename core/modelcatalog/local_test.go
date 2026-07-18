package modelcatalog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestListLocalModels(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "owner", "repo")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"b.GGUF": "bb", "a.gguf": "a", "notes.txt": "x"} {
		if err := os.WriteFile(filepath.Join(nested, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(nested, "a.gguf"), filepath.Join(root, "linked.gguf")); err != nil {
		t.Fatal(err)
	}
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root})
	models, err := catalog.ListLocal(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Filename != "a.gguf" || models[1].Filename != "b.GGUF" {
		t.Fatalf("models = %#v", models)
	}
	if models[0].SizeBytes != 1 {
		t.Fatalf("size = %d", models[0].SizeBytes)
	}
}

func TestListLocalModelsMissingRoot(t *testing.T) {
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: filepath.Join(t.TempDir(), "missing")})
	models, err := catalog.ListLocal(context.Background())
	if err != nil || len(models) != 0 {
		t.Fatalf("models = %#v, err = %v", models, err)
	}
}

func TestDeleteLocal_happy(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(nested, "m.gguf")
	if err := os.WriteFile(path, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root})
	if err := catalog.DeleteLocal(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file stat err = %v", err)
	}
	for _, dir := range []string{filepath.Join(root, "a", "b"), filepath.Join(root, "a")} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pruned dir %s stat err = %v", dir, err)
		}
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("storage root stat = %v, err = %v", info, err)
	}
}

func TestDeleteLocal_outsideRoot(t *testing.T) {
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: t.TempDir()})
	err := catalog.DeleteLocal(context.Background(), "/etc/passwd")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDeleteLocal_pathTraversal(t *testing.T) {
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: t.TempDir()})
	err := catalog.DeleteLocal(context.Background(), "../../x.gguf")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestDeleteLocal_notGguf(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "m.bin")
	if err := os.WriteFile(path, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: root})
	err := catalog.DeleteLocal(context.Background(), path)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}
