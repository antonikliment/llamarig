package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

type runCommandTest struct {
	name    string
	args    []string
	textOut string
	jsonOut string
}

func TestRunCommands(t *testing.T) {
	server := &fakeCLIControl{}
	socketPath := serveCLIRPC(t, server)
	tests := []runCommandTest{
		{
			name:    "info",
			textOut: "status: running\ndefault: qwen\npresets: 1\nautostart: qwen\n",
			jsonOut: "{\"router\":{\"status\":\"running\",\"detail\":\"\",\"checked_at\":\"\"},\"presets_count\":1,\"default_preset\":\"qwen\",\"autostart_presets\":[\"qwen\"]}\n",
		},
		{
			name:    "status",
			textOut: "status: running\ndetail: ready\n",
			jsonOut: "{\"state\":\"running\",\"detail\":\"ready\",\"checked_at\":\"\",\"presets\":null}\n",
		},
		{
			name:    "presets",
			textOut: "qwen  model.gguf\n",
			jsonOut: "{\"presets\":[{\"name\":\"qwen\",\"entries\":[{\"key\":\"model\",\"value\":\"model.gguf\"}],\"source_status\":\"\",\"source_error\":\"\",\"autostart\":false}]}\n",
		},
		{
			name:    "preset",
			args:    []string{"qwen"},
			textOut: "{\n  \"name\": \"qwen\",\n  \"entries\": [\n    {\n      \"key\": \"model\",\n      \"value\": \"model.gguf\"\n    }\n  ],\n  \"source_status\": \"\",\n  \"source_error\": \"\",\n  \"autostart\": false\n}\n",
			jsonOut: "{\"preset\":{\"name\":\"qwen\",\"entries\":[{\"key\":\"model\",\"value\":\"model.gguf\"}],\"source_status\":\"\",\"source_error\":\"\",\"autostart\":false}}\n",
		},
		{name: "start", args: []string{"qwen"}, textOut: "start: succeeded\n", jsonOut: "{\"result\":{\"target\":\"qwen\",\"action\":\"start\",\"status\":\"succeeded\",\"message\":\"\",\"duration_ms\":0}}\n"},
		{name: "stop", args: []string{"qwen"}, textOut: "stop: succeeded\n", jsonOut: "{\"result\":{\"target\":\"qwen\",\"action\":\"stop\",\"status\":\"succeeded\",\"message\":\"\",\"duration_ms\":0}}\n"},
		{name: "restart", args: []string{"qwen"}, textOut: "restart: succeeded\n", jsonOut: "{\"result\":{\"target\":\"qwen\",\"action\":\"restart\",\"status\":\"succeeded\",\"message\":\"\",\"duration_ms\":0}}\n"},
	}
	for _, test := range tests {
		for _, jsonOut := range []bool{false, true} {
			runCommand(t, socketPath, test, jsonOut)
		}
	}
	assertActionTargets(t, server.calledTargets())
}

func runCommand(t *testing.T, socketPath string, test runCommandTest, jsonOut bool) {
	t.Helper()
	mode, want := "text", test.textOut
	if jsonOut {
		mode, want = "json", test.jsonOut
	}
	t.Run(test.name+"/"+mode, func(t *testing.T) {
		var out strings.Builder
		err := Run(context.Background(), Options{
			Command: test.name, Args: test.args, Socket: socketPath, JSON: jsonOut, Out: &out,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if out.String() != want {
			t.Fatalf("out=%q want=%q", out.String(), want)
		}
	})
}

func assertActionTargets(t *testing.T, targets []string) {
	t.Helper()
	if len(targets) != 6 {
		t.Fatalf("action targets=%v", targets)
	}
	for _, target := range targets {
		if target != "qwen" {
			t.Fatalf("action target=%q", target)
		}
	}
}

func TestRunValidatesBeforeDialing(t *testing.T) {
	tests := []Options{
		{Command: "info", Args: []string{"extra"}},
		{Command: "preset"},
		{Command: "start", Args: []string{"one", "two"}},
		{Command: "missing"},
	}
	for _, opts := range tests {
		if err := Run(context.Background(), opts); err == nil {
			t.Fatalf("Run(%q, %v) returned nil", opts.Command, opts.Args)
		}
	}
}

func TestCommandsReturnsRegistrationOrder(t *testing.T) {
	specs := Commands()
	want := []string{"info", "status", "presets", "preset", "start", "stop", "restart"}
	if len(specs) != len(want) {
		t.Fatalf("len(Commands())=%d want=%d", len(specs), len(want))
	}
	for i, name := range want {
		if specs[i].Name != name {
			t.Fatalf("Commands()[%d].Name=%q want=%q", i, specs[i].Name, name)
		}
	}
}

type fakeCLIControl struct {
	controlv1connect.UnimplementedControlServiceHandler
	mu      sync.Mutex
	targets []string
}

func (*fakeCLIControl) GetInfo(context.Context, *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
	return &controlv1.GetInfoResponse{Ok: true, Info: &controlv1.RuntimeInfo{
		Router:           &controlv1.RouterInfo{Status: "running"},
		PresetsCount:     1,
		DefaultPreset:    "qwen",
		AutostartPresets: []string{"qwen"},
	}}, nil
}

func (*fakeCLIControl) GetRuntimeStatus(context.Context, *controlv1.GetRuntimeStatusRequest) (*controlv1.GetRuntimeStatusResponse, error) {
	return &controlv1.GetRuntimeStatusResponse{Ok: true, Status: &controlv1.RuntimeStatus{State: "running", Detail: "ready"}}, nil
}

func (*fakeCLIControl) ListPresets(context.Context, *controlv1.ListPresetsRequest) (*controlv1.ListPresetsResponse, error) {
	return &controlv1.ListPresetsResponse{Ok: true, Presets: []*controlv1.ModelPreset{{
		Name: "qwen", Entries: []*controlv1.PresetEntry{{Key: "model", Value: "model.gguf"}},
	}}}, nil
}

func (*fakeCLIControl) GetPreset(context.Context, *controlv1.GetPresetRequest) (*controlv1.PresetResponse, error) {
	return &controlv1.PresetResponse{Ok: true, Preset: &controlv1.ModelPreset{
		Name: "qwen", Entries: []*controlv1.PresetEntry{{Key: "model", Value: "model.gguf"}},
	}}, nil
}

func (f *fakeCLIControl) StartRuntime(_ context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return f.action("start", req.GetTarget()), nil
}

func (f *fakeCLIControl) StopRuntime(_ context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return f.action("stop", req.GetTarget()), nil
}

func (f *fakeCLIControl) RestartRuntime(_ context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	return f.action("restart", req.GetTarget()), nil
}

func (f *fakeCLIControl) action(action, target string) *controlv1.CommandResponse {
	f.mu.Lock()
	f.targets = append(f.targets, target)
	f.mu.Unlock()
	return &controlv1.CommandResponse{Ok: true, Result: &controlv1.CommandResult{
		Target: target, Action: action, Status: "succeeded",
	}}
}

func (f *fakeCLIControl) calledTargets() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.targets...)
}

func serveCLIRPC(t *testing.T, svc controlv1connect.ControlServiceHandler) string {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix socket: %v", err)
	}
	_, handler := controlv1connect.NewControlServiceHandler(svc)
	server := &http.Server{Handler: handler}
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
		err := <-errs
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("server: %v", err)
		}
	})
	return socketPath
}
