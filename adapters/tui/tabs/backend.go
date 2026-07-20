package tabs

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"
	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"llamarig/platform/audit"
	"llamarig/platform/process"

	tea "charm.land/bubbletea/v2"
)

type dashboardSnapshot struct {
	daemon      process.DetachedStatus
	gateway     process.DetachedStatus
	config      config.Config
	runtime     *controlv1.RuntimeStatus
	resources   *controlv1.SignalsSnapshot
	presets     []presetView
	localModels []*controlv1.LocalModel
	warnings    map[string]string
	refreshed   time.Time
	configPath  string
	logPath     string
	daemonLog   []zapEntry
	llamaLog    []string
}

type pollResult struct {
	snapshot                                                          dashboardSnapshot
	configOK, runtimeOK, resourcesOK, presetsOK, localModelsOK, logOK bool
}

type fetchResult[T any] struct {
	value T
	err   error
}

type dashboardBackend struct {
	ctx        context.Context
	client     controlv1connect.ControlServiceClient
	configPath string
	logPath    string
}

func newDashboardBackend(ctx context.Context) dashboardBackend {
	client, _ := newControlClient()
	configPath, _ := config.ConfigPath()
	logPath, _ := audit.GetLogPath(config.ProjectName)
	return dashboardBackend{ctx: ctx, client: client, configPath: shortPath(configPath), logPath: shortPath(logPath)}
}

func (b dashboardBackend) poll() tea.Cmd {
	return func() tea.Msg {
		result := pollResult{snapshot: dashboardSnapshot{warnings: map[string]string{}, refreshed: time.Now(), configPath: b.configPath, logPath: b.logPath}}
		result.snapshot.daemon, _ = process.StatusDetached(config.ProjectName)
		result.snapshot.gateway, _ = process.StatusDetached("gateway")
		daemonLog, llamaLog, logErr := readDaemonLog()
		result.snapshot.daemonLog, result.snapshot.llamaLog, result.logOK = daemonLog, llamaLog, logErr == nil
		addWarning(result.snapshot.warnings, "logs", logErr)
		cfg, configErr := config.Load()
		result.snapshot.config, result.configOK = cfg, configErr == nil
		addWarning(result.snapshot.warnings, "config", configErr)
		if b.client == nil {
			result.snapshot.warnings["control"] = "control socket not available"
			return result
		}
		ctx, cancel := context.WithTimeout(b.ctx, 2*time.Second)
		defer cancel()
		var runtime fetchResult[*controlv1.RuntimeStatus]
		var resources fetchResult[*controlv1.SignalsSnapshot]
		var presets fetchResult[[]presetView]
		var localModels fetchResult[[]*controlv1.LocalModel]
		group, groupCtx := errgroup.WithContext(ctx)
		group.Go(func() error {
			out, err := b.client.GetRuntimeStatus(groupCtx, &controlv1.GetRuntimeStatusRequest{})
			runtime = fetched(out.GetStatus(), err)
			return nil
		})
		group.Go(func() error {
			out, err := b.client.GetSignals(groupCtx, &controlv1.GetSignalsRequest{})
			resources = fetched(out.GetSignals(), err)
			return nil
		})
		group.Go(func() error {
			out, err := b.client.ListPresets(groupCtx, &controlv1.ListPresetsRequest{})
			presets = fetched(presetsToViews(out.GetPresets()), err)
			return nil
		})
		group.Go(func() error {
			out, err := b.client.ListLocalModels(groupCtx, &controlv1.ListLocalModelsRequest{})
			localModels = fetched(out.GetModels(), err)
			return nil
		})
		_ = group.Wait()
		if runtime.err == nil {
			result.snapshot.runtime, result.runtimeOK = runtime.value, true
		}
		if resources.err == nil {
			result.snapshot.resources, result.resourcesOK = resources.value, true
		}
		if presets.err == nil {
			result.snapshot.presets, result.presetsOK = presets.value, true
		}
		if localModels.err == nil {
			result.snapshot.localModels, result.localModelsOK = localModels.value, true
		}
		addWarning(result.snapshot.warnings, "runtime", runtime.err)
		addWarning(result.snapshot.warnings, "resources", resources.err)
		addWarning(result.snapshot.warnings, "presets", presets.err)
		addWarning(result.snapshot.warnings, "models", localModels.err)
		return result
	}
}

// presetView is the dashboard's projection of a models.ini preset.
type presetView struct {
	Name         string
	Model        string
	ModelsDir    string
	SourceStatus string
	SourceError  string
	Autostart    bool
}

// presetsToViews projects models.ini presets into the view the dashboard
// renders, deriving the model path from the section entries.
func presetsToViews(presets []*controlv1.ModelPreset) []presetView {
	out := make([]presetView, 0, len(presets))
	for _, preset := range presets {
		view := presetView{Name: preset.GetName(), SourceStatus: preset.GetSourceStatus(), SourceError: preset.GetSourceError(), Autostart: preset.GetAutostart()}
		for _, entry := range preset.GetEntries() {
			switch entry.GetKey() {
			case "model":
				view.Model = entry.GetValue()
			case "models-dir":
				view.ModelsDir = entry.GetValue()
			}
		}
		out = append(out, view)
	}
	return out
}

// fetched wraps an RPC's extracted value and error into a fetchResult.
func fetched[T any](value T, err error) fetchResult[T] {
	if err != nil {
		return fetchResult[T]{err: err}
	}
	return fetchResult[T]{value: value}
}

func addWarning(warnings map[string]string, source string, err error) {
	if err != nil {
		warnings[source] = err.Error()
	}
}

func mergeSnapshot(current dashboardSnapshot, result pollResult) dashboardSnapshot {
	next := result.snapshot
	if !result.configOK {
		next.config = current.config
	}
	if !result.runtimeOK {
		next.runtime = current.runtime
	}
	if !result.resourcesOK {
		next.resources = current.resources
	}
	if !result.presetsOK {
		next.presets = current.presets
	}
	if !result.localModelsOK {
		next.localModels = current.localModels
	}
	if !result.logOK {
		next.daemonLog, next.llamaLog = current.daemonLog, current.llamaLog
	}
	return next
}

type actionTarget int

const (
	actionDaemon actionTarget = iota
	actionGateway
	actionRuntime
)

type actionRequestMsg struct {
	target actionTarget
	index  int
	name   string
}

type actionResultMsg struct {
	target actionTarget
	index  int
	err    error
}

type modelDeleteRequestMsg struct{ path string }
type modelDeleteResultMsg struct{ err error }
type presetCleanupRequestMsg struct{ name string }
type presetCleanupResultMsg struct{ err error }
type presetAutostartRequestMsg struct {
	name    string
	enabled bool
}
type presetAutostartResultMsg struct {
	enabled bool
	err     error
}

// rpcCmd guards against a nil client and wraps an RPC call in a tea.Cmd.
// call is invoked only when the client is available; msg builds the result message.
func (b dashboardBackend) rpcCmd(call func() error, msg func(error) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if b.client == nil {
			return msg(fmt.Errorf("control socket not available"))
		}
		return msg(call())
	}
}

func (b dashboardBackend) startPreset(name string) tea.Cmd {
	return b.rpcCmd(func() error {
		_, err := b.client.StartRuntime(b.ctx, &controlv1.RuntimeTargetRequest{Target: name})
		return err
	}, func(err error) tea.Msg { return presetStartResultMsg{err: err} })
}

func (b dashboardBackend) deleteLocalModel(path string) tea.Cmd {
	return b.rpcCmd(func() error {
		_, err := b.client.DeleteLocalModel(b.ctx, &controlv1.DeleteLocalModelRequest{Path: path, CascadePresets: true})
		return err
	}, func(err error) tea.Msg { return modelDeleteResultMsg{err: err} })
}

func (b dashboardBackend) setPresetAutostart(name string, enabled bool) tea.Cmd {
	return b.rpcCmd(func() error {
		_, err := b.client.SetPresetAutostart(b.ctx, &controlv1.PresetAutostartRequest{Name: name, Enabled: enabled})
		return err
	}, func(err error) tea.Msg { return presetAutostartResultMsg{enabled: enabled, err: err} })
}

func (b dashboardBackend) cleanupPreset(name string) tea.Cmd {
	return b.rpcCmd(func() error {
		_, err := b.client.CleanupPreset(b.ctx, &controlv1.CleanupPresetRequest{Name: name})
		return err
	}, func(err error) tea.Msg { return presetCleanupResultMsg{err: err} })
}

func (b dashboardBackend) run(request actionRequestMsg, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch request.target {
		case actionDaemon:
			err = runProcessAction(config.ProjectName, request.index, []string{"serve"})
		case actionGateway:
			if request.index == 2 {
				err = openBrowser(publicBaseURL(cfg.ListenAddr))
			} else if err = runProcessAction("gateway", request.index, []string{"gateway", "--foreground"}); err == nil {
				err = b.persistGatewayStartup(cfg, request.index == 0)
			}
		case actionRuntime:
			if b.client == nil {
				err = fmt.Errorf("control socket not available")
			} else {
				_, err = b.client.StopRuntime(b.ctx, &controlv1.RuntimeTargetRequest{Target: request.name})
			}
		}
		return actionResultMsg{target: request.target, index: request.index, err: err}
	}
}

// persistGatewayStartup records whether the web gateway should auto-start on
// the next TUI launch, so an explicit Stop in the TUI is remembered.
func (b dashboardBackend) persistGatewayStartup(cfg config.Config, enabled bool) error {
	base := cfg.StartupServices
	if len(base) == 0 {
		base = config.DefaultStartupServices()
	}
	desired := slices.DeleteFunc(slices.Clone(base), func(s string) bool { return s == config.StartupServiceWeb })
	if enabled {
		desired = append(desired, config.StartupServiceWeb)
	}
	if slices.Equal(desired, base) {
		return nil
	}
	if b.client == nil {
		return fmt.Errorf("control socket not available")
	}
	_, err := b.client.SetStartupServices(b.ctx, &controlv1.SetStartupServicesRequest{Services: desired})
	return err
}

// autostartResultMsg reports which configured startup services the TUI
// started on launch, for display in the footer notice.
type autostartResultMsg struct {
	started []string
	errs    map[string]string
}

var startupServiceArgs = map[string][]string{
	config.StartupServiceControl: {config.ProjectName, "serve"},
	config.StartupServiceWeb:     {"gateway", "gateway", "--foreground"},
}

// autostart starts the services listed in config.StartupServices that are
// not already running, mirroring the manual Start action on the Services tab.
func (b dashboardBackend) autostart() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return autostartResultMsg{}
		}
		services := cfg.StartupServices
		if services == nil {
			services = config.DefaultStartupServices()
		}
		result := autostartResultMsg{errs: map[string]string{}}
		for _, name := range services {
			args, ok := startupServiceArgs[name]
			if !ok {
				continue
			}
			processName := args[0]
			if status, _ := process.StatusDetached(processName); status.Running {
				continue
			}
			if err := process.StartDetached(processName, args[1:]...); err != nil {
				result.errs[name] = err.Error()
				continue
			}
			result.started = append(result.started, name)
		}
		return result
	}
}

func runProcessAction(name string, index int, args []string) error {
	switch index {
	case 0:
		return process.StartDetached(name, args...)
	case 1:
		return process.StopDetached(name)
	default:
		return nil
	}
}

func openBrowser(target string) error {
	command := browserOpenCommand(target)
	cmd := exec.Command(command[0], command[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func browserOpenCommand(target string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"open", target}
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", target}
	default:
		return []string{"xdg-open", target}
	}
}
