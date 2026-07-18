package control

import (
	"context"
	"sync/atomic"
	"time"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	"llamarig/core/runtime"
)

type Runtime interface {
	Status(context.Context) (runtime.Status, error)
	Start(context.Context) (runtime.CommandResult, error)
	Stop(context.Context) (runtime.CommandResult, error)
	Recover(context.Context) (bool, error)
}

type AuditSink interface {
	Record(ctx context.Context, event AuditEvent)
}

type LocalModelStore interface {
	DeleteLocal(context.Context, string) error
}

// PresetStore is the models.ini-native source for model presets.
type PresetStore interface {
	Root() string
	List(context.Context) ([]modelpresets.Section, error)
	Get(context.Context, string) (modelpresets.Section, error)
	Global(context.Context) (modelpresets.Section, error)
	Put(context.Context, modelpresets.Section, bool) error
	Delete(context.Context, string) error
}

type RouterClient interface {
	List(context.Context) ([]router.Model, error)
	Reload(context.Context) ([]router.Model, error)
	Load(context.Context, string) error
	Unload(context.Context, string) error
}

type AuditEvent struct {
	Protocol  string
	Action    string
	Success   bool
	ErrorKind ErrorKind
	Duration  time.Duration
}

type Manager struct {
	config        *configstore.FileStore
	presets       PresetStore
	router        RouterClient
	routerRuntime Runtime
	routerConfig  config.RouterConfig
	localModels   LocalModelStore
	audit         AuditSink
	busy          atomic.Bool
	helpRunner    commandRunner
	helpCache     helpParamsCache
}

type Dependencies struct {
	Config        *configstore.FileStore
	Presets       PresetStore
	Router        RouterClient
	RouterRuntime Runtime
	RouterConfig  config.RouterConfig
	Audit         AuditSink
	LocalModels   LocalModelStore
}

func NewManager(deps Dependencies) *Manager {
	return &Manager{
		config:        deps.Config,
		presets:       deps.Presets,
		router:        deps.Router,
		routerRuntime: deps.RouterRuntime,
		routerConfig:  deps.RouterConfig,
		audit:         deps.Audit,
		localModels:   deps.LocalModels,
	}
}

func (m *Manager) GetInfo(ctx context.Context) (RuntimeInfo, error) {
	if m.routerRuntime == nil {
		return RuntimeInfo{}, Errorf(ErrorInvalidInput, "router runtime is not configured")
	}
	status, err := m.routerRuntime.Status(ctx)
	if err != nil {
		return RuntimeInfo{}, err
	}
	presetsCount := 0
	if m.presets != nil {
		presets, err := m.presets.List(ctx)
		if err != nil {
			return RuntimeInfo{}, mapServerConfigError(err)
		}
		presetsCount = len(presets)
	}
	info := Info{Status: string(status.State), Detail: status.Detail, CheckedAt: status.CheckedAt.Format(time.RFC3339)}
	routerConfig := m.routerConfigSnapshot(ctx)
	return RuntimeInfo{Router: info, PresetsCount: presetsCount, DefaultPreset: routerConfig.DefaultPreset, AutostartPresets: append([]string(nil), routerConfig.AutostartPresets...)}, nil
}

func (m *Manager) routerConfigSnapshot(ctx context.Context) config.RouterConfig {
	if m.config != nil {
		if document, err := m.config.Read(ctx); err == nil {
			return document.Parsed.Router
		}
	}
	cfg := m.routerConfig
	cfg.AutostartPresets = append([]string(nil), cfg.AutostartPresets...)
	return cfg
}

// RouterAutostartInfo returns the current autostart presets list and the
// models_max cap from the persisted router config.
func (m *Manager) RouterAutostartInfo(ctx context.Context) (autostartPresets []string, modelsMax int) {
	cfg := m.routerConfigSnapshot(ctx)
	return append([]string(nil), cfg.AutostartPresets...), cfg.ModelsMax
}

func withMutationResult[T any](m *Manager, ctx context.Context, action string, zero T, fn func() (T, error)) (T, error) {
	if !m.busy.CompareAndSwap(false, true) {
		return zero, Errorf(ErrorConflict, "another mutating operation is running")
	}
	defer m.busy.Store(false)
	start := time.Now()
	result, err := fn()
	m.record(ctx, action, start, err)
	return result, err
}

func (m *Manager) record(ctx context.Context, action string, start time.Time, err error) {
	if m.audit == nil {
		return
	}
	m.audit.Record(ctx, AuditEvent{
		Action:    action,
		Success:   err == nil,
		ErrorKind: Kind(err),
		Duration:  time.Since(start),
	})
}
