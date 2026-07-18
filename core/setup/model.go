package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"llamarig/config"
)

type step int

const (
	stepWelcome step = iota
	stepListen
	stepToken
	stepLlamaExe
	stepLlamaModelsDir
	stepLlamaModel
	stepLlamaPort
	stepAutoStart
	stepStartupServices
	stepReview
	stepDone
)

type model struct {
	paths                    Paths
	answers                  Answers
	step                     step
	input                    textinput.Model
	cancelled                bool
	confirmedRemoteNoToken   bool
	confirmedMissingLlamaExe bool
	err                      error
}

var titleStyle = lipgloss.NewStyle().Bold(true)

var stepPrompts = map[step]string{
	stepListen:          "Control server listen address",
	stepToken:           "Bearer token environment variable",
	stepLlamaExe:        "llama-server executable",
	stepLlamaModelsDir:  "Models directory",
	stepLlamaModel:      "Model file (blank to use models directory)",
	stepLlamaPort:       "Router port",
	stepAutoStart:       "Autostart llama-server",
	stepStartupServices: "Start automatically: control, web, or both",
}

func RunWizard(ctx context.Context, paths Paths) (Answers, error) {
	m := newModel(paths)
	program := tea.NewProgram(m)
	final, err := program.Run()
	if err != nil {
		return Answers{}, err
	}
	result, ok := final.(*model)
	if !ok {
		return Answers{}, fmt.Errorf("unexpected setup model %T", final)
	}
	if result.err != nil {
		return Answers{}, result.err
	}
	if result.cancelled {
		return Answers{}, ErrCancelled
	}
	_ = ctx
	return result.answers, nil
}

func newModel(paths Paths) *model {
	input := textinput.New()
	input.Focus()
	input.CharLimit = 512
	m := &model{paths: paths, answers: DefaultAnswers(paths), input: input}
	m.setInput()
	return m
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "b":
			m.back()
			return m, nil
		case "enter":
			return m.advance()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) View() tea.View {
	if m.step == stepDone {
		return tea.NewView(titleStyle.Render(config.ProjectDisplayName+" setup complete.") +
			"\n\nCreated:\n  " + m.paths.Config + "\n  " + m.paths.ModelsINI +
			"\n\nNext steps:\n  1. Run `" + config.ProjectName + "` to open the TUI (it starts " + strings.Join(m.answers.StartupServices, " + ") + " automatically)\n" +
			"  2. Get a model into " + m.answers.LlamaModelsDir + " (or via the web GUI model browser)\n" +
			"  3. Start the \"default\" preset from the TUI, `" + config.ProjectName + " start default`, or the GUI\n")
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(config.ProjectDisplayName + " setup"))
	b.WriteString("\n\n")
	switch m.step {
	case stepWelcome:
		b.WriteString("This will create:\n  " + m.paths.Config + "\n  " + m.paths.ModelsINI + "\n\nEnter: continue  q: cancel\n")
	case stepAutoStart:
		b.WriteString("Start llama-server automatically when " + config.ProjectDisplayName + " starts? yes/no\n\n")
		b.WriteString(m.input.View())
	case stepReview:
		b.WriteString("Review\n\n")
		b.WriteString(config.ProjectDisplayName + " home:       " + m.paths.Home + "\n")
		b.WriteString("Config file:       " + m.paths.Config + "\n")
		b.WriteString("Model presets:     " + m.paths.ModelsINI + "\n")
		b.WriteString("Listen address:    " + m.answers.ListenAddr + "\n")
		b.WriteString("Starts automatically: " + strings.Join(m.answers.StartupServices, ", ") + "\n")
		b.WriteString("Enter: write files  b: back  q: cancel\n")
	default:
		b.WriteString(stepPrompts[m.step] + "\n\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\nEnter: continue  b: back  q: cancel\n")
	}
	if m.err != nil {
		b.WriteString("\nError: " + m.err.Error() + "\n")
	}
	return tea.NewView(b.String())
}

func (m *model) advance() (tea.Model, tea.Cmd) {
	m.err = nil
	if m.step == stepReview {
		m.step = stepDone
		return m, tea.Quit
	}
	if err := m.captureStepValue(); err != nil {
		m.err = err
		return m, nil
	}
	m.step++
	m.setInput()
	return m, nil
}

func (m *model) captureStepValue() error {
	switch m.step {
	case stepListen:
		return m.captureListen()
	case stepToken:
		return m.captureToken()
	case stepLlamaExe:
		return m.captureLlamaExe()
	case stepLlamaModelsDir, stepLlamaModel:
		*m.fields()[m.step] = strings.TrimSpace(m.input.Value())
	case stepLlamaPort:
		return m.capturePort()
	case stepAutoStart:
		m.answers.AutoStart = strings.ToLower(strings.TrimSpace(m.input.Value())) == "yes"
	case stepStartupServices:
		return m.captureStartupServices()
	}
	return nil
}

func (m *model) captureLlamaExe() error {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		value = config.DefaultLlamaExecutable
	}
	m.answers.LlamaExecutable = value
	if llamaExecutableResolves(value) || m.confirmedMissingLlamaExe {
		return nil
	}
	m.confirmedMissingLlamaExe = true
	return fmt.Errorf("%q not found; install llama.cpp and ensure llama-server is on PATH (or set the full path) — press Enter again to continue anyway", value)
}

func llamaExecutableResolves(path string) bool {
	if filepath.IsAbs(path) || strings.ContainsRune(path, filepath.Separator) {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(path)
	return err == nil
}

func (m *model) captureStartupServices() error {
	value := strings.ToLower(strings.TrimSpace(m.input.Value()))
	switch value {
	case "", "both":
		m.answers.StartupServices = []string{config.StartupServiceControl, config.StartupServiceWeb}
	case "control":
		m.answers.StartupServices = []string{config.StartupServiceControl}
	case "web":
		m.answers.StartupServices = []string{config.StartupServiceWeb}
	default:
		return fmt.Errorf("enter control, web, or both")
	}
	return nil
}

func (m *model) captureListen() error {
	m.answers.ListenAddr = strings.TrimSpace(m.input.Value())
	_, err := netSplitHostPort(m.answers.ListenAddr)
	return err
}

func (m *model) captureToken() error {
	m.answers.AuthTokenEnv = strings.TrimSpace(m.input.Value())
	if !(&config.Config{ListenAddr: m.answers.ListenAddr}).AllowsNonLoopback() || os.Getenv(m.answers.AuthTokenEnv) != "" || m.confirmedRemoteNoToken {
		return nil
	}
	m.confirmedRemoteNoToken = true
	return fmt.Errorf("remote-capable bind without token env configured; press Enter again to confirm")
}

func (m *model) capturePort() error {
	port := strings.TrimSpace(m.input.Value())
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	m.answers.LlamaPort = port
	return nil
}

func (m *model) back() {
	if m.step > stepWelcome {
		m.step--
	}
	m.setInput()
}

func (m *model) setInput() {
	m.input.SetValue(m.value())
	m.input.Placeholder = m.value()
	m.input.Focus()
	m.input.CursorEnd()
}

func (m *model) value() string {
	if m.step == stepStartupServices {
		return "both"
	}
	if field := m.fields()[m.step]; field != nil {
		return *field
	}
	return "no"
}

func (m *model) fields() map[step]*string {
	return map[step]*string{
		stepListen: &m.answers.ListenAddr, stepToken: &m.answers.AuthTokenEnv,
		stepLlamaExe: &m.answers.LlamaExecutable, stepLlamaModelsDir: &m.answers.LlamaModelsDir,
		stepLlamaModel: &m.answers.LlamaModelFile, stepLlamaPort: &m.answers.LlamaPort,
	}
}
