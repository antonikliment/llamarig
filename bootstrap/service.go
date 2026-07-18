package bootstrap

import (
	"context"
	"io/fs"
	"llamarig/adapters/public_http"
	platformconfig "llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/control"
	"llamarig/core/modelcatalog"
	"llamarig/core/modeldownload"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	"llamarig/core/rpc"
	platformruntime "llamarig/core/runtime"
	"llamarig/core/setup"
	"llamarig/core/signals"
	"llamarig/platform/audit"
	"llamarig/webui"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type Service struct {
	Config      platformconfig.Config
	Manager     *control.Manager
	Events      *control.EventStore
	stopWatcher context.CancelFunc

	ControlRPCServer     *http.Server
	ControlRPCListener   net.Listener
	ControlRPCSocketPath string
}

type Options struct {
	Logger *zap.Logger
	Env    func(string) string
}

type Gateway struct {
	Config     platformconfig.Config
	HTTPServer *http.Server
}

func appFileSystem() fs.FS {
	if dir := os.Getenv(platformconfig.ProjectAppDirEnv); dir != "" {
		return os.DirFS(dir)
	}
	dist, err := fs.Sub(webui.Files, "dist")
	if err != nil {
		panic(err)
	}
	return dist
}

func NewService(ctx context.Context, opts Options) (*Service, error) {
	logger := opts.Logger
	cfg, authToken, err := loadConfig(ctx, opts)
	if err != nil {
		return nil, err
	}
	logger.Info("loaded config",
		zap.Bool("disable_origin_check", cfg.Security.DisableOriginCheck),
		zap.String("listen_addr", cfg.ListenAddr),
	)
	warnUnsafeSecurityConfig(logger, cfg, authToken)
	configPath, err := platformconfig.ConfigPath()
	if err != nil {
		return nil, err
	}
	llamaRigHome, err := platformconfig.LlamaRigHome()
	if err != nil {
		return nil, err
	}
	modelsINI := filepath.Join(llamaRigHome, "models.ini")
	modelStorageDir, err := platformconfig.ResolveModelStorageDir(cfg.ModelStorageDir)
	if err != nil {
		return nil, err
	}
	catalogCacheDir, err := platformconfig.ResolveCatalogCacheDir(cfg.CatalogCacheDir)
	if err != nil {
		return nil, err
	}
	modelCatalog := modelcatalog.NewHuggingFaceCatalog(modelcatalog.HuggingFaceCatalogOptions{
		ModelStorageDir: modelStorageDir,
		CacheDir:        catalogCacheDir,
		CacheTTL:        cfg.CatalogCacheTTL,
	})
	modelDownloader := modeldownload.NewManager(modeldownload.Dependencies{Catalog: modelCatalog, DownloadURL: modelCatalog})
	events := control.NewEventStore(control.DefaultEventLimit)
	routerRuntime := platformruntime.BuildRouter(cfg.Router, modelStorageDir, modelsINI)
	managerDeps := control.Dependencies{
		Config:        configstore.NewFileStore(configPath, configstore.DefaultLimitBytes),
		Presets:       modelpresets.NewStore(modelsINI),
		Router:        router.NewClient("http://"+net.JoinHostPort(platformconfig.DefaultLlamaHost, strconv.Itoa(cfg.Router.Port)), &http.Client{Timeout: cfg.Router.ReadinessTimeout}),
		RouterRuntime: routerRuntime,
		RouterConfig:  cfg.Router,
		Audit:         control.MultiAuditSink{audit.NewSink(logger), events},
		LocalModels:   modelCatalog,
	}
	manager := control.NewManager(managerDeps)
	if err := manager.RecoverRouterRuntime(ctx); err != nil {
		logger.Error("failed to recover router runtime", zap.Error(err))
	}
	for _, err := range manager.StartAutostartPresets(ctx) {
		logger.Warn("skip Preset autostart", zap.Error(err))
	}
	watchCtx, stopWatcher := context.WithCancel(context.Background())
	go watchModelStorage(watchCtx, modelCatalog.WatchLocal(watchCtx, time.Second), manager, logger)
	servers, err := rpc.NewControlRPCServer(controlRPCDeps(manager, routerRuntime, modelCatalog, modelDownloader, events, modelStorageDir))
	if err != nil {
		stopWatcher()
		return nil, err
	}
	return &Service{Config: cfg, Manager: manager, Events: events, stopWatcher: stopWatcher, ControlRPCServer: servers.Server, ControlRPCListener: servers.Listener, ControlRPCSocketPath: servers.SocketPath}, nil
}

func (s *Service) Close() {
	if s.stopWatcher != nil {
		s.stopWatcher()
	}
}

func watchModelStorage(ctx context.Context, changes <-chan struct{}, manager *control.Manager, logger *zap.Logger) {
	for range changes {
		for {
			_, err := manager.RefreshRouterSources(ctx, "model storage changed")
			if control.Kind(err) != control.ErrorConflict {
				if err != nil && ctx.Err() == nil {
					logger.Error("refresh router model sources", zap.Error(err))
				}
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
}

func NewGateway(ctx context.Context, opts Options) (*Gateway, error) {
	cfg, authToken, err := loadConfig(ctx, opts)
	if err != nil {
		return nil, err
	}
	socketPath, err := platformconfig.ControlSocketPath()
	if err != nil {
		return nil, err
	}
	warnUnsafeSecurityConfig(opts.Logger, cfg, authToken)
	return &Gateway{Config: cfg, HTTPServer: public_http.PublicHttpServer(cfg, appFileSystem(), authToken, socketPath)}, nil
}

func loadConfig(ctx context.Context, opts Options) (platformconfig.Config, string, error) {
	if err := setup.Ensure(ctx); err != nil {
		return platformconfig.Config{}, "", err
	}
	cfg, err := platformconfig.Load()
	if err != nil {
		return platformconfig.Config{}, "", err
	}
	return cfg, opts.Env(cfg.Security.AuthTokenEnv), nil
}

func controlRPCDeps(
	manager *control.Manager,
	routerRuntime signals.RuntimeStatusProvider,
	modelCatalog interface {
		modelcatalog.Catalog
		modelcatalog.Discoverer
		modelcatalog.RefreshNotifier
		modelcatalog.LocalLister
	}, modelDownloader *modeldownload.Manager, events *control.EventStore, modelStorageDir string) rpc.RPCDependencies {
	collector := signals.NewGopsutilCollector(routerRuntime, []signals.DiskTarget{{Label: "root", Path: "/"}, {Label: "model_storage", Path: modelStorageDir}})
	return rpc.RPCDependencies{Manager: manager, ModelCatalog: modelCatalog, ModelDiscoverer: modelCatalog, LocalModels: modelCatalog, RefreshNotifier: modelCatalog, ModelDownloader: modelDownloader, Events: events, Signals: collector, Machine: collector, ServiceName: rpc.ServiceName}
}

func warnUnsafeSecurityConfig(logger *zap.Logger, cfg platformconfig.Config, authToken string) {
	if cfg.AllowsNonLoopback() && authToken == "" {
		logger.Warn("listen address is remote-capable and bearer auth is not configured; set security.auth_token_env to a non-empty token env var before exposing this service", zap.String("listen_addr", cfg.ListenAddr), zap.String("auth_token_env", cfg.Security.AuthTokenEnv))
	}
	if cfg.Security.DisableOriginCheck && authToken == "" {
		logger.Warn("origin check is disabled and bearer auth is not configured; browser clients on the network may be able to control " + platformconfig.ProjectDisplayName)
	}
	if cfg.RouterAllowsNonLoopback() {
		logger.Warn("router.host is not loopback; the llama-server API has no authentication of its own and will be reachable by anyone who can route to it", zap.String("router_host", cfg.Router.Host))
	}
}
