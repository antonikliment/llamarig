//go:build e2e_live

package e2e

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const llamaLatestReleaseURL = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func resolveLlamaServer(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("LLAMA_SERVER"); path != "" {
		requireExecutable(t, path)
		return path
	}
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	return installLatestLlamaServer(t)
}

func installLatestLlamaServer(t *testing.T) string {
	t.Helper()
	release := latestLlamaRelease(t)
	assetName, assetURL := selectLlamaAsset(t, release)
	dir := filepath.Join(llamaCacheDir(t), release.TagName, runtime.GOOS+"-"+runtime.GOARCH)
	if bin := cachedLlamaServer(dir); bin != "" {
		return bin
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	extractLlamaArchive(t, assetURL, dir)
	bin := cachedLlamaServer(dir)
	if bin == "" {
		t.Fatalf("llama-server not found in %s", assetURL)
	}
	t.Logf("installed llama-server from %s %s", release.TagName, assetName)
	return bin
}

func latestLlamaRelease(t *testing.T) githubRelease {
	t.Helper()
	resp, err := http.Get(llamaLatestReleaseURL)
	if err != nil {
		t.Fatalf("fetch llama.cpp latest release: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch llama.cpp latest release status=%d", resp.StatusCode)
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		t.Fatalf("decode llama.cpp latest release: %v", err)
	}
	if release.TagName == "" {
		t.Fatal("llama.cpp latest release missing tag_name")
	}
	return release
}

func selectLlamaAsset(t *testing.T, release githubRelease) (string, string) {
	t.Helper()
	want := llamaAssetNeedle()
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, want) && strings.HasSuffix(asset.Name, ".tar.gz") {
			return asset.Name, asset.URL
		}
	}
	t.Skipf("no llama.cpp release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	return "", ""
}

func llamaAssetNeedle() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "bin-ubuntu-x64"
	case "linux/arm64":
		return "bin-ubuntu-arm64"
	case "darwin/amd64":
		return "bin-macos-x64"
	case "darwin/arm64":
		return "bin-macos-arm64"
	default:
		return "\x00"
	}
}

func llamaCacheDir(t *testing.T) string {
	t.Helper()
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "llamarig", "e2e", "llama.cpp")
	}
	return filepath.Join(os.TempDir(), "llamarig-e2e-llama-cache")
}

func extractLlamaArchive(t *testing.T, url string, dst string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("download llama.cpp asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download llama.cpp asset status=%d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("open llama.cpp asset gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("read llama.cpp asset tar: %v", err)
		}
		path, ok := safeExtractPath(dst, header.Name)
		if !ok {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			writeFile(t, path, tr, 0o700)
		case tar.TypeSymlink:
			writeSymlink(t, path, header.Linkname)
		}
	}
}

func safeExtractPath(root, name string) (string, bool) {
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.Join(root, clean), true
}

func writeFile(t *testing.T, dst string, r io.Reader, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(file, r); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeSymlink(t *testing.T, dst string, target string) {
	t.Helper()
	if target == "" || filepath.IsAbs(target) || strings.HasPrefix(filepath.Clean(target), ".."+string(os.PathSeparator)) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, dst); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
}

func requireExecutable(t *testing.T, path string) {
	t.Helper()
	if !executable(path) {
		t.Fatalf("LLAMA_SERVER %q is not executable", path)
	}
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}

func cachedLlamaServer(dir string) string {
	var found string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Base(path) == "llama-server" && executable(path) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" || (runtime.GOOS == "linux" && (!hasFile(dir, "libllama-server-impl.so") || !hasFile(dir, "libllama-common.so.0"))) {
		return ""
	}
	return found
}

func hasFile(dir, name string) bool {
	ok := false
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Base(path) == name {
			ok = true
			return filepath.SkipAll
		}
		return nil
	})
	return ok
}
