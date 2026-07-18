package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llamarig/config"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	"llamarig/core/runtime"
)

type fakeRouterClient struct {
	models       []router.Model
	reloadModels []router.Model
	loaded       []string
	unloaded     []string
	reloads      int
	reloadErr    error
	listErr      error
	lists        int
}

type deleteFailingPresetStore struct {
	*modelpresets.Store
	err error
}

func (s deleteFailingPresetStore) Delete(context.Context, string) error { return s.err }

func (f *fakeRouterClient) List(context.Context) ([]router.Model, error) {
	f.lists++
	return f.models, f.listErr
}

func (f *fakeRouterClient) Reload(context.Context) ([]router.Model, error) {
	f.reloads++
	if f.reloadModels != nil {
		return f.reloadModels, f.reloadErr
	}
	return f.models, f.reloadErr
}

func (f *fakeRouterClient) Load(_ context.Context, name string) error {
	f.loaded = append(f.loaded, name)
	return nil
}

func (f *fakeRouterClient) Unload(_ context.Context, name string) error {
	f.unloaded = append(f.unloaded, name)
	return nil
}

func TestManagerRoutesPresetOperationsThroughRouter(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	modelPath := writeControlTestModel(t)
	if err := store.Put(context.Background(), modelpresets.Section{Name: "demo", Values: map[string]string{"model": modelPath}}, true); err != nil {
		t.Fatal(err)
	}
	routerClient := &fakeRouterClient{}
	routerRuntime := &fakeRuntime{name: "router"}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: routerRuntime})

	if _, err := manager.StartOperation(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !routerRuntime.running || len(routerClient.loaded) != 1 || routerClient.loaded[0] != "demo" {
		t.Fatalf("runtime running=%v, loaded=%v", routerRuntime.running, routerClient.loaded)
	}
	model := router.Model{ID: "demo"}
	model.Status.Value = "loaded"
	routerClient.models = []router.Model{model}
	status, err := manager.Status(context.Background())
	if err != nil || status.State != string(runtime.Running) || len(status.Presets) != 1 {
		t.Fatalf("status=%#v, err=%v", status, err)
	}
	if _, err := manager.StopOperation(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if len(routerClient.unloaded) != 1 || routerClient.unloaded[0] != "demo" {
		t.Fatalf("unloaded=%v", routerClient.unloaded)
	}
}

func writeControlTestModel(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartRejectsUnavailablePresetBeforeRouterCall(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "broken", Values: map[string]string{"model": "/missing.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	routerClient := &fakeRouterClient{}
	routerRuntime := &fakeRuntime{name: "router"}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: routerRuntime})

	_, err := manager.StartOperation(context.Background(), "broken")
	if Kind(err) != ErrorConflict || len(routerClient.loaded) != 0 || routerRuntime.starts != 0 {
		t.Fatalf("err=%v loaded=%v starts=%d", err, routerClient.loaded, routerRuntime.starts)
	}
}

func TestManagerEmptyStopStopsRouter(t *testing.T) {
	routerClient := &fakeRouterClient{}
	routerRuntime := &fakeRuntime{name: "router", running: true}
	events := NewEventStore(10)
	manager := NewManager(Dependencies{Router: routerClient, RouterRuntime: routerRuntime, Audit: events})

	result, err := manager.StopOperation(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Target != "all" || routerRuntime.running {
		t.Fatalf("result=%#v, running=%v", result, routerRuntime.running)
	}
	if got := events.List(); len(got) != 1 || got[0].Action != "stop_router" || !got[0].Success {
		t.Fatalf("events = %#v", got)
	}
}

func TestManagerEmptyStopUsesMutationGate(t *testing.T) {
	routerRuntime := &fakeRuntime{name: "router", running: true}
	manager := NewManager(Dependencies{RouterRuntime: routerRuntime})
	manager.busy.Store(true)
	defer manager.busy.Store(false)

	if _, err := manager.StopOperation(context.Background(), ""); Kind(err) != ErrorConflict {
		t.Fatalf("StopOperation() error = %v", err)
	}
	if routerRuntime.stops != 0 {
		t.Fatalf("stops = %d", routerRuntime.stops)
	}
}

func TestManagerHandlesMissingRouterDependencies(t *testing.T) {
	manager := NewManager(Dependencies{})
	ctx := context.Background()

	status, err := manager.Status(ctx)
	if err != nil || status.State != string(runtime.Stopped) || status.Detail != "router runtime is not configured" {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	if _, err := manager.StopOperation(ctx, ""); Kind(err) != ErrorInvalidInput {
		t.Fatalf("StopOperation() error = %v", err)
	}
	if err := manager.RecoverRouterRuntime(ctx); Kind(err) != ErrorInvalidInput {
		t.Fatalf("RecoverRouterRuntime() error = %v", err)
	}
	if err := manager.applyRouterAction(ctx, "start", "demo"); Kind(err) != ErrorInvalidInput {
		t.Fatalf("applyRouterAction() error = %v", err)
	}
	if err := manager.ensureRouter(ctx); Kind(err) != ErrorInvalidInput {
		t.Fatalf("ensureRouter() error = %v", err)
	}
	if running, err := manager.modelRunning(ctx, "demo"); err != nil || running {
		t.Fatalf("modelRunning() = %v, %v", running, err)
	}
	if _, err := manager.GetInfo(ctx); Kind(err) != ErrorInvalidInput {
		t.Fatalf("GetInfo() error = %v", err)
	}
}

func TestRouterStatusPreservesProcessStateWhenListFails(t *testing.T) {
	for _, state := range []runtime.State{runtime.Starting, runtime.Running, runtime.Stopping, runtime.Failed} {
		t.Run(string(state), func(t *testing.T) {
			manager := NewManager(Dependencies{
				Router:        &fakeRouterClient{listErr: errors.New("router unavailable")},
				RouterRuntime: &fakeRuntime{name: "router", state: state},
			})
			status, err := manager.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if status.State != string(state) || status.Detail != "router process "+string(state)+"; model list unavailable" {
				t.Fatalf("status = %#v", status)
			}
		})
	}
}

func TestRouterStatusReportsStoppedWhenListFailsAndProcessStopped(t *testing.T) {
	manager := NewManager(Dependencies{
		Router:        &fakeRouterClient{listErr: errors.New("router unavailable")},
		RouterRuntime: &fakeRuntime{name: "router", state: runtime.Stopped},
	})
	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != string(runtime.Stopped) || status.Detail != "router stopped" {
		t.Fatalf("status = %#v", status)
	}
}

func TestPutPresetRefreshesRunningRouter(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	model := router.Model{ID: "demo"}
	model.Status.Value = "loaded"
	routerClient := &fakeRouterClient{models: []router.Model{model}}
	routerRuntime := &fakeRuntime{name: "router", running: true}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: routerRuntime})

	_, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "new", Values: map[string]string{"model": "/new.gguf"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if routerClient.reloads != 1 || routerRuntime.stops != 0 {
		t.Fatalf("reloads=%d stops=%d", routerClient.reloads, routerRuntime.stops)
	}
}

func TestPutPresetKeepsStoppedRouterStopped(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	routerClient := &fakeRouterClient{}
	routerRuntime := &fakeRuntime{name: "router"}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: routerRuntime})

	if _, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "new", Values: map[string]string{"model": "/new.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	if routerClient.reloads != 0 || routerRuntime.starts != 0 {
		t.Fatalf("reloads=%d starts=%d", routerClient.reloads, routerRuntime.starts)
	}
}

func TestRefreshFallsBackToRouterRestart(t *testing.T) {
	model := router.Model{ID: "demo"}
	model.Status.Value = "loaded"
	routerClient := &fakeRouterClient{
		models:    []router.Model{model},
		reloadErr: errors.New("reload unsupported"),
	}
	routerRuntime := &fakeRuntime{name: "router", running: true}
	manager := NewManager(Dependencies{Router: routerClient, RouterRuntime: routerRuntime})

	if _, err := manager.RefreshRouterSources(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if routerRuntime.stops != 1 || routerRuntime.starts != 1 {
		t.Fatalf("stops=%d starts=%d", routerRuntime.stops, routerRuntime.starts)
	}
}

func TestRefreshRouterLifecycleSurvivesRequestCancellation(t *testing.T) {
	routerClient := &fakeRouterClient{reloadErr: errors.New("reload unsupported")}
	routerRuntime := &fakeRuntime{name: "router", running: true}
	manager := NewManager(Dependencies{Router: routerClient, RouterRuntime: routerRuntime})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := manager.RefreshRouterSources(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	if routerRuntime.lifecycleErr != nil || routerRuntime.stops != 1 || routerRuntime.starts != 1 {
		t.Fatalf("lifecycle error=%v stops=%d starts=%d", routerRuntime.lifecycleErr, routerRuntime.stops, routerRuntime.starts)
	}
}

func TestRefreshReloadsChangedActivePreset(t *testing.T) {
	active := router.Model{ID: "demo"}
	active.Status.Value = "loaded"
	changed := router.Model{ID: "demo"}
	changed.Status.Value = "unloaded"
	routerClient := &fakeRouterClient{models: []router.Model{active}, reloadModels: []router.Model{changed}}
	manager := NewManager(Dependencies{Router: routerClient, RouterRuntime: &fakeRuntime{name: "router", running: true}})

	if _, err := manager.RefreshRouterSources(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if len(routerClient.loaded) != 1 || routerClient.loaded[0] != "demo" {
		t.Fatalf("loaded=%v", routerClient.loaded)
	}
}

func TestPutPresetRevertsCreationWhenRefreshFails(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	routerClient := &fakeRouterClient{reloadErr: errors.New("reload failed")}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: &fakeRuntime{name: "router", running: true, startErr: errors.New("start failed")}})

	_, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "new", Values: map[string]string{"model": "/new.gguf"}}, true)
	if Kind(err) != ErrorRuntime {
		t.Fatalf("PutPreset() error = %v", err)
	}
	if _, getErr := store.Get(context.Background(), "new"); !errors.Is(getErr, modelpresets.ErrNotFound) {
		t.Fatalf("expected reverted creation to be removed, getErr = %v", getErr)
	}
}

func TestPutPresetReportsRollbackDeleteError(t *testing.T) {
	rollbackErr := errors.New("delete failed")
	store := deleteFailingPresetStore{Store: modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini")), err: rollbackErr}
	manager := NewManager(Dependencies{
		Presets:       store,
		Router:        &fakeRouterClient{reloadErr: errors.New("reload failed")},
		RouterRuntime: &fakeRuntime{name: "router", running: true, startErr: errors.New("start failed")},
	})

	_, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "new", Values: map[string]string{"model": "/new.gguf"}}, true)
	if !errors.Is(err, rollbackErr) || !strings.Contains(err.Error(), rollbackErr.Error()) {
		t.Fatalf("PutPreset() error = %v", err)
	}
}

func TestPutPresetRestoresRouterAfterRevertingCreation(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	active := router.Model{ID: "demo"}
	active.Status.Value = "loaded"
	routerClient := &fakeRouterClient{models: []router.Model{active}, reloadErr: errors.New("reload failed")}
	routerRuntime := &fakeRuntime{name: "router", running: true, startErrors: []error{errors.New("bad preset")}}
	routerRuntime.onStart = func() {
		unloaded := active
		unloaded.Status.Value = "unloaded"
		routerClient.models = []router.Model{unloaded}
	}
	manager := NewManager(Dependencies{
		Presets:       store,
		Router:        routerClient,
		RouterRuntime: routerRuntime,
	})

	_, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "new", Values: map[string]string{"model": "/new.gguf"}}, true)
	if Kind(err) != ErrorRuntime || !routerRuntime.running || routerRuntime.stops != 2 || routerRuntime.starts != 1 || len(routerClient.loaded) != 1 || routerClient.loaded[0] != "demo" {
		t.Fatalf("error=%v running=%v stops=%d starts=%d loaded=%v", err, routerRuntime.running, routerRuntime.stops, routerRuntime.starts, routerClient.loaded)
	}
}

func TestPutPresetKeepsExistingContentWhenRefreshFails(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "existing", Values: map[string]string{"model": "/old.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	routerClient := &fakeRouterClient{reloadErr: errors.New("reload failed")}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: &fakeRuntime{name: "router", running: true, startErr: errors.New("start failed")}})

	_, err := manager.PutPreset(context.Background(), modelpresets.Section{Name: "existing", Values: map[string]string{"model": "/saved.gguf"}}, false)
	if Kind(err) != ErrorRuntime {
		t.Fatalf("PutPreset() error = %v", err)
	}
	saved, getErr := store.Get(context.Background(), "existing")
	if getErr != nil {
		t.Fatalf("existing preset missing after refresh error: %v", getErr)
	}
	if saved.Values["model"] != "/saved.gguf" {
		t.Fatalf("expected edit to persist despite refresh error, got %v", saved.Values)
	}
}

func TestStartRejectsMissingPresetBeforeRouterCall(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	routerClient := &fakeRouterClient{}
	manager := NewManager(Dependencies{Presets: store, Router: routerClient, RouterRuntime: &fakeRuntime{name: "router", running: true}})

	_, err := manager.StartOperation(context.Background(), "missing")
	if Kind(err) != ErrorNotFound || len(routerClient.loaded) != 0 {
		t.Fatalf("err=%v loaded=%v", err, routerClient.loaded)
	}
}

func TestDeleteRejectsConfiguredDefaultPreset(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "default", Values: map[string]string{"model": "/model.gguf"}}, true); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Dependencies{Presets: store, RouterConfig: config.RouterConfig{DefaultPreset: "default"}})

	if err := manager.DeletePreset(context.Background(), "default"); Kind(err) != ErrorConflict {
		t.Fatalf("DeletePreset() error = %v", err)
	}
}

func TestDeleteMissingPresetIsSuccessfulNoOp(t *testing.T) {
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	routerClient := &fakeRouterClient{}
	manager := NewManager(Dependencies{
		Presets:      store,
		Router:       routerClient,
		RouterConfig: config.RouterConfig{DefaultPreset: "missing", AutostartPresets: []string{"missing"}},
	})

	if err := manager.DeletePreset(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}
	if routerClient.reloads != 0 {
		t.Fatalf("reloads = %d", routerClient.reloads)
	}
}
