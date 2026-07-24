package llamainstall

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func (i *installer) detect(ctx context.Context) Backend {
	if i.goos == "darwin" {
		return BackendMetal
	}
	if i.commandWorks(ctx, "nvidia-smi", "-L") {
		return BackendCUDA
	}
	if i.commandWorks(ctx, "rocminfo") {
		return BackendROCm
	}
	if i.commandWorks(ctx, "vulkaninfo", "--summary") {
		return BackendVulkan
	}
	return BackendCPU
}

func (i *installer) commandWorks(ctx context.Context, command string, args ...string) bool {
	if _, err := exec.LookPath(command); err != nil {
		return false
	}
	_, err := i.output(ctx, command, args...)
	return err == nil
}

func (i *installer) validateChoice(backend Backend, source bool) error {
	if i.goos == "darwin" {
		if backend != BackendCPU && backend != BackendMetal {
			return fmt.Errorf("backend %s is unsupported on macOS", backend)
		}
		if !source && backend == BackendCPU {
			return fmt.Errorf("%w for macOS/cpu; use --source or --backend metal", ErrNoPrebuilt)
		}
		return nil
	}
	if backend == BackendMetal {
		return fmt.Errorf("backend metal is unsupported on Linux")
	}
	return nil
}

func (i *installer) latest(ctx context.Context) (release, error) {
	var value release
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return value, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "llamarig")
	token := cmp.Or(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return value, fmt.Errorf("resolve llama.cpp release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return value, fmt.Errorf("resolve llama.cpp release: %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return value, err
	}
	if value.TagName == "" || value.TarballURL == "" {
		return value, fmt.Errorf("llama.cpp release metadata is incomplete")
	}
	return value, nil
}

func (i *installer) releaseAsset(rel release, backend Backend) (asset, error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[i.goarch]
	platform := "ubuntu"
	if i.goos == "darwin" {
		platform = "macos"
	}
	part := ""
	if backend != BackendCPU && backend != BackendMetal {
		part = "-" + string(backend)
	}
	name := fmt.Sprintf("llama-%s-bin-%s%s-%s.tar.gz", rel.TagName, platform, part, arch)
	for _, candidate := range rel.Assets {
		matchesROCm := backend == BackendROCm && strings.HasPrefix(candidate.Name, strings.TrimSuffix(name, arch+".tar.gz")) && strings.HasSuffix(candidate.Name, "-"+arch+".tar.gz")
		if candidate.Name == name || matchesROCm {
			return candidate, nil
		}
	}
	return asset{}, fmt.Errorf("%w for %s/%s/%s", ErrNoPrebuilt, i.goos, i.goarch, backend)
}
