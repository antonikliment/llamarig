package llamainstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (i *installer) installAsset(ctx context.Context, opts Options, rel release, backend Backend, payload, work string) error {
	selected, err := i.releaseAsset(rel, backend)
	if err != nil {
		return err
	}
	if selected.Size <= 0 || !strings.HasPrefix(selected.Digest, "sha256:") {
		return fmt.Errorf("release asset %s lacks a GitHub SHA256 digest or size", selected.Name)
	}
	i.phase(opts, "download "+selected.Name)
	archive := filepath.Join(work, "release.tar.gz")
	hash, size, err := i.download(ctx, selected.URL, archive)
	if err != nil {
		return err
	}
	if size != selected.Size || !strings.EqualFold(hash, strings.TrimPrefix(selected.Digest, "sha256:")) {
		return fmt.Errorf("release asset integrity check failed")
	}
	i.phase(opts, "extract prebuilt")
	return extractTarGz(ctx, archive, payload)
}

func (i *installer) download(ctx context.Context, url, destination string) (string, int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	request.Header.Set("User-Agent", "llamarig")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("download %s: %s", url, response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	return hex.EncodeToString(hash.Sum(nil)), size, closeErr
}

func extractTarGz(ctx context.Context, archive, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	if output, err := exec.CommandContext(ctx, "tar", "-xzf", archive, "-C", destination).CombinedOutput(); err != nil {
		return fmt.Errorf("extract archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func findExecutable(root string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "llama-server" {
			matches = append(matches, path)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("expected one llama-server, found %d", len(matches))
	}
	return matches[0], nil
}
