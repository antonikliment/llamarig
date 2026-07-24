package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"llamarig/config"
	"llamarig/core/llamainstall"
)

// runForm runs a huh form and maps a user abort (ctrl+c/esc) to ErrCancelled.
// It is a package var so tests can stub the interactive form.
var runForm = func(ctx context.Context, form *huh.Form) error {
	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrCancelled
		}
		return err
	}
	return nil
}

// RunWizard collects setup answers through an interactive huh form, restoring
// the full-screen UI and back navigation while keeping validation and the
// remote/executable safety confirmations.
func RunWizard(ctx context.Context, paths Paths) (Answers, error) {
	answers := DefaultAnswers(paths)
	startup := "both"
	proceed := true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(config.ProjectDisplayName+" setup").
				Description("This will create:\n  "+paths.Config+"\n  "+paths.ModelsINI),
		),
		huh.NewGroup(
			huh.NewInput().Title("Control server listen address").Value(&answers.ListenAddr).Validate(validateListen),
			huh.NewInput().Title("Bearer token environment variable").Value(&answers.AuthTokenEnv),
			huh.NewInput().Title("llama-server executable").Value(&answers.LlamaExecutable),
			huh.NewInput().Title("Models directory").Value(&answers.LlamaModelsDir),
			huh.NewInput().Title("Model file (blank to use models directory)").Value(&answers.LlamaModelFile),
			huh.NewInput().Title("Router port").Value(&answers.LlamaPort).Validate(validatePort),
			huh.NewConfirm().Title("Autostart llama-server when "+config.ProjectDisplayName+" starts?").Value(&answers.AutoStart),
			huh.NewSelect[string]().
				Title("Start automatically").
				Options(
					huh.NewOption("control + web", "both"),
					huh.NewOption("control", "control"),
					huh.NewOption("web", "web"),
				).
				Value(&startup),
		),
		huh.NewGroup(
			huh.NewNote().Title("Review").DescriptionFunc(func() string {
				return reviewSummary(paths, &answers, startup)
			}, []any{&answers, &startup}),
			huh.NewConfirm().Title("Write configuration files?").Value(&proceed),
		),
	).WithTheme(huh.ThemeFunc(tealStyles))

	if err := runForm(ctx, form); err != nil {
		return Answers{}, err
	}
	if !proceed {
		return Answers{}, ErrCancelled
	}

	normalizeAnswers(&answers, startup)
	if !llamaExecutableResolves(answers.LlamaExecutable) {
		executable, err := offerManagedLlama(ctx)
		if err != nil {
			return Answers{}, err
		}
		if executable != "" {
			answers.LlamaExecutable = executable
		}
	}

	if err := confirmSafety(ctx, answers); err != nil {
		return Answers{}, err
	}
	return answers, nil
}

func offerManagedLlama(ctx context.Context) (string, error) {
	install := true
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title("llama-server was not found. Install a managed llama.cpp now?").
		Value(&install)))
	if err := runForm(ctx, form); err != nil || !install {
		return "", err
	}
	backend, err := llamainstall.Detect(ctx)
	if err != nil {
		return "", err
	}
	options := llamainstall.Options{Backend: llamainstall.BackendAuto, Progress: os.Stderr}
	if runtime.GOOS == "linux" && backend == llamainstall.BackendCUDA {
		choice := "source"
		choiceForm := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().
			Title("CUDA has no upstream prebuilt. Choose an installation").
			Options(
				huh.NewOption("Build CUDA from source", "source"),
				huh.NewOption("Use Vulkan prebuilt", "vulkan"),
				huh.NewOption("Skip installation", "skip"),
			).Value(&choice)))
		if err := runForm(ctx, choiceForm); err != nil || choice == "skip" {
			return "", err
		}
		options.Source = choice == "source"
		if choice == "vulkan" {
			options.Backend = llamainstall.BackendVulkan
		}
	}
	result, err := llamainstall.Install(ctx, options)
	return result, err
}

// tealStyles is huh's Charm theme with its fuchsia/pink accents recolored teal,
// so the wizard matches the rest of the TUI instead of the library default pink.
func tealStyles(isDark bool) *huh.Styles {
	teal := lipgloss.Color("14")
	s := huh.ThemeCharm(isDark)
	s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(teal)
	s.Focused.NextIndicator = s.Focused.NextIndicator.Foreground(teal)
	s.Focused.PrevIndicator = s.Focused.PrevIndicator.Foreground(teal)
	s.Focused.MultiSelectSelector = s.Focused.MultiSelectSelector.Foreground(teal)
	s.Focused.FocusedButton = s.Focused.FocusedButton.Background(teal)
	s.Focused.Next = s.Focused.FocusedButton
	s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(teal)
	// Re-derive Blurred from the recolored Focused (matching ThemeCharm's own
	// derivation) so the pink does not survive on unfocused fields.
	s.Blurred = s.Focused
	s.Blurred.Base = s.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	s.Blurred.Card = s.Blurred.Base
	s.Blurred.NextIndicator = lipgloss.NewStyle()
	s.Blurred.PrevIndicator = lipgloss.NewStyle()
	return s
}

func normalizeAnswers(a *Answers, startup string) {
	a.ListenAddr = strings.TrimSpace(a.ListenAddr)
	a.AuthTokenEnv = strings.TrimSpace(a.AuthTokenEnv)
	a.LlamaModelsDir = strings.TrimSpace(a.LlamaModelsDir)
	a.LlamaModelFile = strings.TrimSpace(a.LlamaModelFile)
	a.LlamaPort = strings.TrimSpace(a.LlamaPort)
	a.LlamaExecutable = strings.TrimSpace(a.LlamaExecutable)
	if a.LlamaExecutable == "" {
		a.LlamaExecutable = config.DefaultLlamaExecutable
	}
	a.StartupServices = startupServices(startup)
}

func reviewSummary(paths Paths, a *Answers, startup string) string {
	return config.ProjectDisplayName + " home:       " + paths.Home + "\n" +
		"Config file:       " + paths.Config + "\n" +
		"Model presets:     " + paths.ModelsINI + "\n" +
		"Listen address:    " + a.ListenAddr + "\n" +
		"Starts automatically: " + strings.Join(startupServices(startup), ", ")
}

func validateListen(addr string) error {
	_, err := netSplitHostPort(strings.TrimSpace(addr))
	return err
}

func validatePort(port string) error {
	value, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	return nil
}

func startupServices(sel string) []string {
	switch sel {
	case "control":
		return []string{config.StartupServiceControl}
	case "web":
		return []string{config.StartupServiceWeb}
	default:
		return []string{config.StartupServiceControl, config.StartupServiceWeb}
	}
}

// confirmSafety asks the user to confirm the two risky-but-allowed conditions
// the wizard previously warned about: a remote-capable bind without a token
// env, and a llama-server executable that does not resolve.
func confirmSafety(ctx context.Context, a Answers) error {
	if remoteWithoutToken(a) {
		if err := requireConfirm(ctx, "Remote-capable bind without a token env configured — continue anyway?"); err != nil {
			return err
		}
	}
	if !llamaExecutableResolves(a.LlamaExecutable) {
		msg := fmt.Sprintf("%q not found; install llama.cpp and ensure llama-server is on PATH (or set the full path) — continue anyway?", a.LlamaExecutable)
		if err := requireConfirm(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func remoteWithoutToken(a Answers) bool {
	return (&config.Config{ListenAddr: a.ListenAddr}).AllowsNonLoopback() && os.Getenv(a.AuthTokenEnv) == ""
}

// requireConfirm shows a single yes/no prompt and returns ErrCancelled if the
// user declines.
func requireConfirm(ctx context.Context, title string) error {
	proceed := false
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(title).Value(&proceed)))
	if err := runForm(ctx, form); err != nil {
		return err
	}
	if !proceed {
		return ErrCancelled
	}
	return nil
}

func llamaExecutableResolves(path string) bool {
	if filepath.IsAbs(path) || strings.ContainsRune(path, filepath.Separator) {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(path)
	return err == nil
}
