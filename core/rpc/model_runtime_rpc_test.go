package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	"llamarig/core/control"
	"llamarig/core/modelcatalog"
	"llamarig/core/modeldownload"
	"llamarig/core/modelpresets"
	"llamarig/core/router"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/runtime"
	"llamarig/core/signals"
)

type rejectModelCatalog struct{}

func (rejectModelCatalog) Resolve(context.Context, string) (modelcatalog.Resolution, error) {
	return modelcatalog.Resolution{}, nil
}

type rejectModelDownloader struct{}

func (rejectModelDownloader) Start(context.Context, modeldownload.Request) (modeldownload.Job, error) {
	return modeldownload.Job{}, nil
}

func (rejectModelDownloader) Get(context.Context, string) (modeldownload.Job, error) {
	return modeldownload.Job{}, nil
}

func (rejectModelDownloader) Cancel(context.Context, string) (modeldownload.Job, error) {
	return modeldownload.Job{}, nil
}

type completedModelDownloader struct{ job modeldownload.Job }

func (d completedModelDownloader) Start(context.Context, modeldownload.Request) (modeldownload.Job, error) {
	return d.job, nil
}

func (d completedModelDownloader) Get(context.Context, string) (modeldownload.Job, error) {
	return d.job, nil
}

func (d completedModelDownloader) Cancel(context.Context, string) (modeldownload.Job, error) {
	return d.job, nil
}

type recordingRouter struct {
	loaded   []string
	unloaded []string
}

func (r *recordingRouter) List(context.Context) ([]router.Model, error)   { return nil, nil }
func (r *recordingRouter) Reload(context.Context) ([]router.Model, error) { return nil, nil }
func (r *recordingRouter) Load(_ context.Context, name string) error {
	r.loaded = append(r.loaded, name)
	return nil
}
func (r *recordingRouter) Unload(_ context.Context, name string) error {
	r.unloaded = append(r.unloaded, name)
	return nil
}

type recordingRuntime struct {
	state  runtime.State
	starts int
}

func (r *recordingRuntime) Status(context.Context) (runtime.Status, error) {
	return runtime.Status{State: r.state, CheckedAt: time.Now().UTC()}, nil
}
func (r *recordingRuntime) Start(context.Context) (runtime.CommandResult, error) {
	r.state = runtime.Running
	r.starts++
	return runtime.CommandResult{Action: "start"}, nil
}
func (r *recordingRuntime) Stop(context.Context) (runtime.CommandResult, error) {
	r.state = runtime.Stopped
	return runtime.CommandResult{Action: "stop"}, nil
}
func (r *recordingRuntime) Recover(context.Context) (bool, error) { return false, nil }

func TestModelRPCRejectsNilRequests(t *testing.T) {
	svc := NewControlService(RPCDependencies{Manager: control.NewManager(control.Dependencies{})})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "resolve",
			call: func() error {
				_, err := svc.ResolveModel(ctx, nil)
				return err
			},
		},
		{
			name: "list catalog",
			call: func() error {
				_, err := svc.ListModelCatalog(ctx, nil)
				return err
			},
		},
		{
			name: "start download",
			call: func() error {
				_, err := svc.StartModelDownload(ctx, nil)
				return err
			},
		},
		{
			name: "get download",
			call: func() error {
				_, err := svc.GetModelDownload(ctx, nil)
				return err
			},
		},
		{
			name: "apply download",
			call: func() error {
				_, err := svc.ApplyModelDownloadToPreset(ctx, nil)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
				t.Fatalf("error kind = %q, want %q; err = %v", ErrorKindFromRPC(err), control.ErrorInvalidInput, err)
			}
		})
	}
}

func TestRuntimeRPCRejectsNilTargetRequests(t *testing.T) {
	var nextCalled bool
	interceptor := validateRequestInterceptor()
	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		return nil, nil
	}

	var request *controlv1.RuntimeTargetRequest
	_, err := interceptor(next)(context.Background(), connect.NewRequest(request))

	if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
		t.Fatalf("error kind = %q, want %q; err = %v", ErrorKindFromRPC(err), control.ErrorInvalidInput, err)
	}
	if nextCalled {
		t.Fatal("next handler was called for typed nil request")
	}
}

func TestSignalsAndRuntimeResourcesMapDisks(t *testing.T) {
	temperature := 63.0
	snapshot := signals.Snapshot{Disks: []signals.DiskStats{
		{Label: "root", Path: "/", UsedBytes: 4, TotalBytes: 10, FreeBytes: 6, UsedPercent: 40},
		{Label: "model_storage", Path: "/models", UsedBytes: 7, TotalBytes: 10, FreeBytes: 3, UsedPercent: 70},
	}, GPU: []signals.GPUStats{{Name: "GPU", TemperatureCelsius: &temperature}}}

	signalsProto := signalsSnapshotProto(snapshot)
	if len(signalsProto.GetDisks()) != 2 || signalsProto.GetDisks()[1].GetLabel() != "model_storage" || signalsProto.GetDisks()[1].GetPath() != "/models" {
		t.Fatalf("signals disks = %#v", signalsProto.GetDisks())
	}
	if len(signalsProto.GetGpu()) != 1 || signalsProto.GetGpu()[0].TemperatureCelsius == nil || signalsProto.GetGpu()[0].GetTemperatureCelsius() != temperature {
		t.Fatalf("signals gpu = %#v", signalsProto.GetGpu())
	}

	resourcesProto := runtimeResourcesProto(snapshot)
	if len(resourcesProto.GetDisks()) != 2 || resourcesProto.GetDisks()[0].GetUsedPercent() != 40 || resourcesProto.GetDisks()[1].GetFreeBytes() != 3 {
		t.Fatalf("resource disks = %#v", resourcesProto.GetDisks())
	}
}

func TestConfigRPCRejectsNilRequest(t *testing.T) {
	svc := NewControlService(RPCDependencies{Manager: control.NewManager(control.Dependencies{})})
	_, err := svc.SetStartupServices(context.Background(), nil)
	if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
		t.Fatalf("error kind = %q, want %q; err = %v", ErrorKindFromRPC(err), control.ErrorInvalidInput, err)
	}
}

func TestModelRPCRejectsEmptyRequiredFields(t *testing.T) {
	svc := NewControlService(RPCDependencies{
		Manager:         control.NewManager(control.Dependencies{}),
		ModelCatalog:    rejectModelCatalog{},
		ModelDownloader: rejectModelDownloader{},
	})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "resolve raw",
			call: func() error {
				_, err := svc.ResolveModel(ctx, &controlv1.ResolveModelRequest{})
				return err
			},
		},
		{
			name: "start download url",
			call: func() error {
				_, err := svc.StartModelDownload(ctx, &controlv1.StartModelDownloadRequest{})
				return err
			},
		},
		{
			name: "get download id",
			call: func() error {
				_, err := svc.GetModelDownload(ctx, &controlv1.GetModelDownloadRequest{})
				return err
			},
		},
		{
			name: "apply download id",
			call: func() error {
				_, err := svc.ApplyModelDownloadToPreset(ctx, &controlv1.ApplyModelDownloadToPresetRequest{Preset: "qwen"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if ErrorKindFromRPC(err) != control.ErrorInvalidInput {
				t.Fatalf("error kind = %q, want %q; err = %v", ErrorKindFromRPC(err), control.ErrorInvalidInput, err)
			}
		})
	}
}

func TestApplyModelDownloadToPresetWithoutRestartKeepsRouterStopped(t *testing.T) {
	resp, routerRuntime, routerClient := applyModelDownloadToPreset(t)
	if resp.GetResult() != nil || routerRuntime.starts != 0 || len(routerClient.loaded) != 0 {
		t.Fatalf("response=%#v starts=%d loaded=%v", resp, routerRuntime.starts, routerClient.loaded)
	}
}

func applyModelDownloadToPreset(t *testing.T) (*controlv1.MutationResponse, *recordingRuntime, *recordingRouter) {
	t.Helper()
	modelPath := filepath.Join(t.TempDir(), "demo.gguf")
	if err := os.WriteFile(modelPath, []byte("gguf"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := modelpresets.NewStore(filepath.Join(t.TempDir(), "models.ini"))
	if err := store.Put(context.Background(), modelpresets.Section{Name: "demo", Values: map[string]string{"models-dir": "/models"}}, true); err != nil {
		t.Fatal(err)
	}
	routerClient := &recordingRouter{}
	routerRuntime := &recordingRuntime{state: runtime.Stopped}
	manager := control.NewManager(control.Dependencies{Presets: store, Router: routerClient, RouterRuntime: routerRuntime})
	svc := NewControlService(RPCDependencies{
		Manager: manager,
		ModelDownloader: completedModelDownloader{job: modeldownload.Job{
			ID: "download", State: modeldownload.StateCompleted, TargetPath: modelPath,
		}},
	})
	resp, err := svc.ApplyModelDownloadToPreset(context.Background(), &controlv1.ApplyModelDownloadToPresetRequest{
		Id: "download", Preset: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp, routerRuntime, routerClient
}
