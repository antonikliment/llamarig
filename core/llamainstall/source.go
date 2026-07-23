package llamainstall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func (i *installer) buildSource(ctx context.Context, opts Options, rel release, backend Backend, payload, work string) error {
	i.phase(opts, "download llama.cpp source")
	archive := filepath.Join(work, "source.tar.gz")
	if _, _, err := i.download(ctx, rel.TarballURL, archive); err != nil {
		return err
	}
	source := filepath.Join(work, "source")
	if err := extractTarGz(ctx, archive, source); err != nil {
		return err
	}
	root, err := findSourceRoot(source)
	if err != nil {
		return err
	}
	build := filepath.Join(work, "build")
	args := cmakeArgs(root, build, backend)
	i.phase(opts, "configure source build")
	if err := i.run(ctx, root, opts.Progress, "cmake", args...); err != nil {
		return fmt.Errorf("configure llama.cpp: %w", err)
	}
	i.phase(opts, "build llama-server")
	if err := i.run(ctx, root, opts.Progress, "cmake", "--build", build, "--config", "Release", "--target", "llama-server", "-j", strconv.Itoa(opts.Jobs)); err != nil {
		return fmt.Errorf("build llama.cpp: %w", err)
	}
	bin := filepath.Join(build, "bin")
	if _, err := os.Stat(filepath.Join(bin, "llama-server")); err != nil {
		bin = filepath.Join(bin, "Release")
	}
	if err := os.CopyFS(payload, os.DirFS(bin)); err != nil {
		return fmt.Errorf("stage llama.cpp build: %w", err)
	}
	return nil
}

func cmakeArgs(root, build string, backend Backend) []string {
	args := []string{
		"-S", root, "-B", build, "-DCMAKE_BUILD_TYPE=Release",
		"-DLLAMA_BUILD_TESTS=OFF", "-DLLAMA_BUILD_EXAMPLES=OFF",
		"-DLLAMA_BUILD_SERVER=ON", "-DLLAMA_CURL=OFF",
		"-DGGML_NATIVE=OFF",
	}
	switch backend {
	case BackendCUDA:
		args = append(args, "-DGGML_CUDA=ON")
	case BackendROCm:
		args = append(args, "-DGGML_HIP=ON")
	case BackendVulkan:
		args = append(args, "-DGGML_VULKAN=ON")
	case BackendMetal:
		args = append(args, "-DGGML_METAL=ON")
	case BackendCPU:
		args = append(args, "-DGGML_METAL=OFF")
	}
	return args
}

func findSourceRoot(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) == 0 || !entries[0].IsDir() {
		return "", fmt.Errorf("invalid source archive")
	}
	return filepath.Join(root, entries[0].Name()), nil
}
