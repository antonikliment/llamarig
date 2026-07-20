package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"llamarig/config"
	"llamarig/core/configstore"
	"llamarig/core/runtime"
)

func TestManagerSetStartupServices(t *testing.T) {
	path := writeControlConfig(t, "listen_addr: \"127.0.0.1:7000\"\n")
	manager := NewManager(Dependencies{Config: configstore.NewFileStore(path, configstore.DefaultLimitBytes)})
	result, err := manager.SetStartupServices(context.Background(), []string{config.StartupServiceControl})
	if err != nil {
		t.Fatalf("SetStartupServices returned error: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatalf("result = %#v", result)
	}
}

func writeControlConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, dir)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type fakeRuntime struct {
	name          string
	running       bool
	state         runtime.State
	statusErr     error
	starts, stops int
	startErr      error
	startErrors   []error
	onStart       func()
	lifecycleErr  error
	statusCalls   int
}

func (f *fakeRuntime) Status(context.Context) (runtime.Status, error) {
	f.statusCalls++
	if f.statusErr != nil {
		return runtime.Status{}, f.statusErr
	}
	state := runtime.Stopped
	if f.state != "" {
		state = f.state
	} else if f.running {
		state = runtime.Running
	}
	return runtime.Status{State: state, Detail: f.name, CheckedAt: time.Now().UTC()}, nil
}

func (f *fakeRuntime) Start(ctx context.Context) (runtime.CommandResult, error) {
	f.lifecycleErr = errors.Join(f.lifecycleErr, ctx.Err())
	if len(f.startErrors) > 0 {
		err := f.startErrors[0]
		f.startErrors = f.startErrors[1:]
		if err != nil {
			return runtime.CommandResult{}, err
		}
	}
	if f.startErr != nil {
		return runtime.CommandResult{}, f.startErr
	}
	f.running = true
	f.starts++
	if f.onStart != nil {
		f.onStart()
	}
	return runtime.CommandResult{Action: "start", ExitCode: 0}, nil
}

func (f *fakeRuntime) Stop(ctx context.Context) (runtime.CommandResult, error) {
	f.lifecycleErr = errors.Join(f.lifecycleErr, ctx.Err())
	f.running = false
	f.stops++
	return runtime.CommandResult{Action: "stop", ExitCode: 0}, nil
}

func (f *fakeRuntime) Recover(context.Context) (bool, error) { return false, nil }
