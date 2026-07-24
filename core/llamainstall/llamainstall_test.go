package llamainstall

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"llamarig/config"
)

func TestReleaseAsset(t *testing.T) {
	rel := release{TagName: "b123", Assets: []asset{
		{Name: "llama-b123-bin-ubuntu-x64.tar.gz"},
		{Name: "llama-b123-bin-ubuntu-vulkan-arm64.tar.gz"},
		{Name: "llama-b123-bin-ubuntu-rocm-7.0-x64.tar.gz"},
		{Name: "llama-b123-bin-macos-arm64.tar.gz"},
	}}
	tests := []struct {
		os, arch string
		backend  Backend
		want     string
	}{
		{"linux", "amd64", BackendCPU, rel.Assets[0].Name},
		{"linux", "arm64", BackendVulkan, rel.Assets[1].Name},
		{"linux", "amd64", BackendROCm, rel.Assets[2].Name},
		{"darwin", "arm64", BackendMetal, rel.Assets[3].Name},
	}
	for _, test := range tests {
		got, err := (&installer{goos: test.os, goarch: test.arch}).releaseAsset(rel, test.backend)
		if err != nil || got.Name != test.want {
			t.Errorf("%s/%s/%s: got %q, %v; want %q", test.os, test.arch, test.backend, got.Name, err, test.want)
		}
	}
}

func TestDetectPriority(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	writeCommand(t, dir, "nvidia-smi", 0)
	writeCommand(t, dir, "rocminfo", 0)
	writeCommand(t, dir, "vulkaninfo", 0)
	i := &installer{goos: "linux"}
	if got := i.detect(context.Background()); got != BackendCUDA {
		t.Fatalf("detect = %s, want cuda", got)
	}
	writeCommand(t, dir, "nvidia-smi", 1)
	if got := i.detect(context.Background()); got != BackendROCm {
		t.Fatalf("detect = %s, want rocm", got)
	}
	writeCommand(t, dir, "rocminfo", 1)
	if got := i.detect(context.Background()); got != BackendVulkan {
		t.Fatalf("detect = %s, want vulkan", got)
	}
}

func TestInstallUpgradeAndRetention(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("fixture names target linux/amd64")
	}
	archive := serverArchive(t)
	sum := sha256.Sum256(archive)
	tag := "b1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			_, _ = w.Write(archive)
			return
		}
		_ = json.NewEncoder(w).Encode(release{
			TagName: tag, TarballURL: serverURL(r) + "/source",
			Assets: []asset{{Name: "llama-" + tag + "-bin-ubuntu-x64.tar.gz", URL: serverURL(r) + "/asset", Digest: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(archive))}},
		})
	}))
	defer server.Close()
	oldURL, oldClient := latestReleaseURL, httpClient
	latestReleaseURL, httpClient = server.URL, server.Client()
	t.Cleanup(func() { latestReleaseURL, httpClient = oldURL, oldClient })

	home := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, home)
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("router:\n  executable: llama-server\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := Options{Backend: BackendCPU, BackendSet: true}
	first, err := Install(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertExecutable(t, first)
	assertConfigured(t, home, first)

	tag = "b2"
	second, err := Upgrade(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	tag = "b3"
	third, err := Upgrade(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertExecutable(t, third)
	firstDir := filepath.Join(home, "llama.cpp", "b1", "linux-amd64-cpu-prebuilt")
	if _, err := os.Stat(firstDir); !os.IsNotExist(err) {
		t.Errorf("oldest install retained: %v", err)
	}
	value, err := (&installer{root: filepath.Join(home, "llama.cpp")}).readState()
	if err != nil || value.Previous == nil || value.Previous.Executable != second {
		t.Fatalf("previous state = %#v, %v", value.Previous, err)
	}
}

func TestInstallRejectsDigestMismatch(t *testing.T) {
	body := []byte("not the declared asset")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	oldClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = oldClient })
	rel := release{TagName: "b1", Assets: []asset{{
		Name: "llama-b1-bin-ubuntu-x64.tar.gz", URL: server.URL,
		Digest: "sha256:" + strings.Repeat("0", 64), Size: int64(len(body)),
	}}}
	err := (&installer{goos: "linux", goarch: "amd64"}).installAsset(context.Background(), Options{Progress: new(bytes.Buffer)}, rel, BackendCPU, filepath.Join(t.TempDir(), "payload"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("error = %v, want integrity failure", err)
	}
}

func TestCMakeArgs(t *testing.T) {
	for backend, flag := range map[Backend]string{
		BackendCPU: "-DGGML_METAL=OFF", BackendCUDA: "-DGGML_CUDA=ON",
		BackendROCm: "-DGGML_HIP=ON", BackendVulkan: "-DGGML_VULKAN=ON",
		BackendMetal: "-DGGML_METAL=ON",
	} {
		if got := strings.Join(cmakeArgs("source", "build", backend), " "); !strings.Contains(got, flag) {
			t.Errorf("%s args %q lack %q", backend, got, flag)
		}
	}
}

func serverArchive(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\nexit 0\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "llama/bin/llama-server", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func writeCommand(t *testing.T, dir, name string, exit int) {
	t.Helper()
	content := fmt.Sprintf("#!/bin/sh\nexit %d\n", exit)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func serverURL(r *http.Request) string { return "http://" + r.Host }

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%q is not executable: %v", path, err)
	}
}

func assertConfigured(t *testing.T, home, executable string) {
	t.Helper()
	cfg, err := config.LoadFile(filepath.Join(home, "config.yaml"))
	if err != nil || cfg.Router.Executable != executable {
		t.Fatalf("configured executable = %q, %v; want %q", cfg.Router.Executable, err, executable)
	}
}
