package modelcatalog

import (
	"path/filepath"
	"testing"
)

func TestParseHuggingFaceURL(t *testing.T) {
	source, err := ParseHuggingFaceURL("https://huggingface.co/unsloth/Qwen3.6-27B-MTP-GGUF")
	if err != nil {
		t.Fatalf("ParseHuggingFaceURL returned error: %v", err)
	}
	if source.Owner != "unsloth" || source.Repo != "Qwen3.6-27B-MTP-GGUF" || source.Kind != "huggingface" {
		t.Fatalf("source = %#v", source)
	}
}

func TestParseHuggingFaceURLRejectsInvalid(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/owner/repo",
		"http://huggingface.co/owner/repo",
		"https://huggingface.co/owner/repo/blob/main/model.gguf",
		"https://huggingface.co/owner",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := ParseHuggingFaceURL(rawURL); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestInferQuant(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{name: "model-Q2_K.gguf", want: "Q2_K"},
		{name: "model-Q3_K_S.gguf", want: "Q3_K_S"},
		{name: "model-Q4_K_M.gguf", want: "Q4_K_M"},
		{name: "model-Q5_K_S.gguf", want: "Q5_K_S"},
		{name: "model-Q6_K.gguf", want: "Q6_K"},
		{name: "model-Q8_0.gguf", want: "Q8_0"},
		{name: "BF16/model-BF16-00001-of-00002.gguf", want: "BF16"},
		{name: "model-UD-Q4_K_XL.gguf", want: "UD-Q4_K_XL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := InferQuant(tc.name); got != tc.want {
				t.Fatalf("InferQuant() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHuggingFaceCatalogLocalPathRejectsTraversal(t *testing.T) {
	catalog := NewHuggingFaceCatalog(HuggingFaceCatalogOptions{ModelStorageDir: t.TempDir()})
	source := Source{Owner: "owner", Repo: "repo"}
	if _, err := catalog.LocalPath(source, "../model.gguf"); err == nil {
		t.Fatal("expected traversal error")
	}
	path, err := catalog.LocalPath(source, "nested/model.gguf")
	if err != nil {
		t.Fatalf("LocalPath returned error: %v", err)
	}
	if filepath.Base(path) != "model.gguf" {
		t.Fatalf("path = %q", path)
	}
}
