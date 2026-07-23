package llamainstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/platform/filedoc"
)

const latestReleaseURL = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"

var (
	ErrNoManagedInstall = errors.New("no managed llama.cpp install")
	ErrNoPrebuilt       = errors.New("no matching llama.cpp prebuilt")
	httpClient          = &http.Client{Timeout: 30 * time.Minute}
)

type Backend string

const (
	BackendAuto   Backend = "auto"
	BackendCPU    Backend = "cpu"
	BackendCUDA   Backend = "cuda"
	BackendROCm   Backend = "rocm"
	BackendVulkan Backend = "vulkan"
	BackendMetal  Backend = "metal"
)

type Options struct {
	Backend               Backend
	BackendSet, SourceSet bool
	Source                bool
	Jobs                  int
	Progress              io.Writer
}

type record struct {
	Tag, Directory, Executable string
	Policy, Backend            Backend
	Source                     bool
}

type state struct {
	Active   *record `json:"active,omitempty"`
	Previous *record `json:"previous,omitempty"`
}

type release struct {
	TagName    string  `json:"tag_name"`
	TarballURL string  `json:"tarball_url"`
	Assets     []asset `json:"assets"`
}

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type installer struct {
	root, goos, goarch string
}

func newInstaller() (*installer, error) {
	home, err := config.LlamaRigHome()
	if err != nil {
		return nil, err
	}
	return &installer{root: filepath.Join(home, "llama.cpp"), goos: runtime.GOOS, goarch: runtime.GOARCH}, nil
}

func Detect(ctx context.Context) (Backend, error) {
	i, err := newInstaller()
	if err != nil {
		return "", err
	}
	return i.detect(ctx), nil
}

func Install(ctx context.Context, opts Options) (string, error) {
	i, err := newInstaller()
	if err != nil {
		return "", err
	}
	return i.execute(ctx, opts, false)
}

func Upgrade(ctx context.Context, opts Options) (string, error) {
	i, err := newInstaller()
	if err != nil {
		return "", err
	}
	return i.execute(ctx, opts, true)
}

func (i *installer) output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (i *installer) run(ctx context.Context, dir string, out io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, out, out
	return cmd.Run()
}

func (i *installer) execute(ctx context.Context, opts Options, upgrade bool) (string, error) {
	if err := i.normalize(&opts); err != nil {
		return "", err
	}
	if err := os.MkdirAll(i.root, 0o700); err != nil {
		return "", fmt.Errorf("create llama.cpp home: %w", err)
	}
	lock := flock.New(filepath.Join(i.root, "install.lock"), flock.SetPermissions(0o600))
	_, err := lock.TryLockContext(ctx, 200*time.Millisecond)
	if err != nil {
		return "", fmt.Errorf("lock llama.cpp installer: %w", err)
	}
	defer func() { _ = lock.Close() }()

	current, err := i.readState()
	if err != nil {
		return "", err
	}
	policy, source, err := resolvePolicy(opts, current.Active, upgrade)
	if err != nil {
		return "", err
	}
	backend := policy
	if backend == BackendAuto {
		backend = i.detect(ctx)
	}
	if err := i.validateChoice(backend, source); err != nil {
		return "", err
	}
	if !source && i.goos == "linux" && backend == BackendCUDA {
		return "", fmt.Errorf("%w for linux/cuda; use --source, --backend vulkan, or Docker", ErrNoPrebuilt)
	}

	return i.installLatest(ctx, opts, current, policy, backend, source)
}

func (i *installer) installLatest(ctx context.Context, opts Options, current state, policy, backend Backend, source bool) (string, error) {
	i.phase(opts, "resolve latest release")
	rel, err := i.latest(ctx)
	if err != nil {
		return "", err
	}
	if current.Active != nil && current.Active.Tag == rel.TagName && current.Active.Backend == backend && current.Active.Source == source {
		current.Active.Policy = policy
		if err := i.writeState(current); err != nil {
			return "", err
		}
		return i.configure(ctx, current.Active, current.Active)
	}

	work, err := os.MkdirTemp(i.root, ".staging-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(work) }()
	payload := filepath.Join(work, "payload")
	if source {
		err = i.buildSource(ctx, opts, rel, backend, payload, work)
	} else {
		err = i.installAsset(ctx, opts, rel, backend, payload, work)
	}
	if err != nil {
		return "", err
	}
	stagedExe, err := findExecutable(payload)
	if err != nil {
		return "", err
	}
	if err := i.validate(ctx, stagedExe); err != nil {
		return "", err
	}
	return i.activate(ctx, opts, current, rel, policy, backend, source, payload, stagedExe)
}

func (i *installer) activate(ctx context.Context, opts Options, current state, rel release, policy, backend Backend, source bool, payload, stagedExe string) (string, error) {
	final := filepath.Join(i.root, rel.TagName, fmt.Sprintf("%s-%s-%s-%s", i.goos, i.goarch, backend, mode(source)))
	if !managedPath(i.root, final) {
		return "", fmt.Errorf("unsafe install path %q", final)
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return "", err
	}
	if err := os.RemoveAll(final); err != nil {
		return "", fmt.Errorf("replace llama.cpp install: %w", err)
	}
	if err := os.Rename(payload, final); err != nil {
		return "", fmt.Errorf("activate llama.cpp: %w", err)
	}
	relExe, _ := filepath.Rel(payload, stagedExe)
	next := &record{Tag: rel.TagName, Policy: policy, Backend: backend, Source: source, Directory: final, Executable: filepath.Join(final, relExe)}
	oldPrevious, previous := current.Previous, current.Active
	current.Active, current.Previous = next, previous
	if err := i.writeState(current); err != nil {
		return "", err
	}
	if oldPrevious != nil && (previous == nil || oldPrevious.Directory != previous.Directory) {
		i.removeManaged(oldPrevious.Directory)
	}
	return i.configure(ctx, previous, next)
}

func (i *installer) normalize(opts *Options) error {
	if i.goarch != "amd64" && i.goarch != "arm64" || i.goos != "linux" && i.goos != "darwin" {
		return fmt.Errorf("unsupported host %s/%s", i.goos, i.goarch)
	}
	if opts.Jobs < 0 {
		return fmt.Errorf("jobs must be positive")
	}
	if opts.Jobs == 0 {
		opts.Jobs = 4
	}
	if opts.Progress == nil {
		opts.Progress = io.Discard
	}
	return nil
}

func resolvePolicy(opts Options, active *record, upgrade bool) (Backend, bool, error) {
	if upgrade && active == nil {
		return "", false, ErrNoManagedInstall
	}
	policy, source := opts.Backend, opts.Source
	if upgrade && !opts.BackendSet {
		policy = active.Policy
	}
	if upgrade && !opts.SourceSet {
		source = active.Source
	}
	if policy == "" {
		policy = BackendAuto
	}
	if !validBackend(policy) {
		return "", false, fmt.Errorf("unknown backend %q", policy)
	}
	return policy, source, nil
}

func (i *installer) phase(opts Options, text string) {
	_, _ = fmt.Fprintln(opts.Progress, text+"...")
}

func (i *installer) validate(ctx context.Context, executable string) error {
	if info, err := os.Stat(executable); err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("llama-server is not executable: %s", executable)
	}
	if out, err := i.output(ctx, executable, "--version"); err != nil {
		return fmt.Errorf("validate llama-server: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (i *installer) configure(ctx context.Context, old, next *record) (string, error) {
	path, err := config.ConfigPath()
	if err != nil {
		return next.Executable, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return next.Executable, nil
	} else if err != nil {
		return next.Executable, err
	}
	store := configstore.NewFileStore(path, configstore.DefaultLimitBytes)
	var replace string
	if old != nil {
		replace = old.Executable
	}
	if _, err := store.SetRouterExecutable(ctx, next.Executable, replace); err != nil {
		return next.Executable, err
	}
	return next.Executable, nil
}

func (i *installer) readState() (state, error) {
	data, err := os.ReadFile(filepath.Join(i.root, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return state{}, nil
	}
	var value state
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("read llama.cpp state: %w", err)
	}
	return value, nil
}

func (i *installer) writeState(value state) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = filedoc.WriteFile(filepath.Join(i.root, "state.json"), string(data)+"\n", filedoc.WriteOptions{Perm: 0o600})
	return err
}

func (i *installer) removeManaged(path string) {
	if managedPath(i.root, path) {
		_ = os.RemoveAll(path)
	}
}

func managedPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func validBackend(value Backend) bool {
	return value == BackendAuto || value == BackendCPU || value == BackendCUDA || value == BackendROCm || value == BackendVulkan || value == BackendMetal
}

func mode(source bool) string {
	if source {
		return "source"
	}
	return "prebuilt"
}
