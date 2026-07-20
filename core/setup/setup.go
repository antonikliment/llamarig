package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"llamarig/config"

	"golang.org/x/term"
)

var ErrCancelled = errors.New("setup cancelled")

type Options struct {
	In         *os.File
	Out        *os.File
	IsTerminal func(fd int) bool
	RunWizard  func(context.Context, Paths) (Answers, error)
	// Force re-runs the wizard and overwrites an existing config instead of
	// skipping when one is already present.
	Force bool
}

type Paths struct {
	Home      string
	Config    string
	ModelsINI string
}

func Ensure(ctx context.Context) error {
	return EnsureWithOptions(ctx, Options{})
}

// Rerun re-runs the setup wizard even if a config already exists, overwriting
// it (with a timestamped backup) once the wizard completes.
func Rerun(ctx context.Context) error {
	return EnsureWithOptions(ctx, Options{Force: true})
}

func EnsureWithOptions(ctx context.Context, opts Options) error {
	if os.Getenv("LLAMARIG_CONFIG") != "" {
		return nil
	}
	paths, err := ResolvePaths()
	if err != nil {
		return err
	}
	exists, err := configExists(paths.Config)
	if err != nil {
		return err
	}
	if exists && !opts.Force {
		return nil
	}

	in, out, isTerminal := setupIO(opts)
	if !isInteractive(in, out, isTerminal) {
		return fmt.Errorf("no %s config found at %s; run `%s setup` in an interactive terminal", config.ProjectDisplayName, paths.Config, config.ProjectName)
	}
	if exists {
		_, _ = fmt.Fprintf(out, "A %s config already exists at %s — continuing will overwrite it (a backup will be kept).\n\n", config.ProjectDisplayName, paths.Config)
	}
	runWizard := opts.RunWizard
	if runWizard == nil {
		runWizard = RunWizard
	}
	answers, err := runWizard(ctx, paths)
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return err
		}
		return fmt.Errorf("run setup wizard: %w", err)
	}
	if err := WriteFiles(paths, answers, opts.Force); err != nil {
		return err
	}
	printSetupSummary(out, paths, answers)
	return nil
}

func printSetupSummary(out io.Writer, paths Paths, answers Answers) {
	_, _ = fmt.Fprintf(out, "%s setup complete.\n\nCreated:\n  %s\n  %s\n\nNext steps:\n"+
		"  1. Run `%s` to open the TUI (it starts %s automatically)\n"+
		"  2. Get a model into %s (or via the web GUI model browser)\n"+
		"  3. Start the \"default\" preset from the TUI, `%s start default`, or the GUI\n",
		config.ProjectDisplayName, paths.Config, paths.ModelsINI,
		config.ProjectName, strings.Join(answers.StartupServices, " + "),
		answers.LlamaModelsDir, config.ProjectName)
}

func configExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat config %q: %w", path, err)
	}
	return false, nil
}

func setupIO(opts Options) (*os.File, *os.File, func(int) bool) {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	isTerminal := opts.IsTerminal
	if isTerminal == nil {
		isTerminal = term.IsTerminal
	}
	return in, out, isTerminal
}

func isInteractive(in *os.File, out *os.File, isTerminal func(int) bool) bool {
	return isTerminal(int(in.Fd())) && isTerminal(int(out.Fd()))
}

func ResolvePaths() (Paths, error) {
	home, err := config.LlamaRigHome()
	if err != nil {
		return Paths{}, err
	}
	configPath, err := config.DefaultConfigPath()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Home: home, Config: configPath, ModelsINI: filepath.Join(home, "models.ini")}, nil
}
