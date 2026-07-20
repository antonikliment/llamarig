package cmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"llamarig/adapters/cli"
	"llamarig/config"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
)

func TestCLICommandParsesInterspersedFlags(t *testing.T) {
	fake := &fakeCLIControl{}
	root := NewRootCommand()
	t.Setenv(config.ProjectSocketEnv, serveCLIControl(t, fake))
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"start", "qwen", "--json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if want := "{\"result\":{\"target\":\"qwen\",\"action\":\"start\",\"status\":\"succeeded\",\"message\":\"\",\"duration_ms\":0}}\n"; out.String() != want {
		t.Fatalf("out=%q want=%q", out.String(), want)
	}
	if target := fake.calledTarget(); target != "qwen" {
		t.Fatalf("target=%q want=qwen", target)
	}
}

func TestCLICommandHelp(t *testing.T) {
	root := NewRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"preset", "--help"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	for _, want := range []string{
		"Show a preset",
		"llamarig preset <name> [flags]",
		"--json",
		"--socket string",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestCLICommandArgumentValidation(t *testing.T) {
	tests := [][]string{
		{"info", "extra"},
		{"preset"},
		{"start", "one", "two"},
		{"missing"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := NewRootCommand()
			root.SetArgs(args)
			if err := root.ExecuteContext(context.Background()); err == nil {
				t.Fatal("ExecuteContext returned nil")
			}
		})
	}
}

func TestCLIFlagsRemainLocal(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"serve", "--json"})
	if err := root.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "unknown flag: --json") {
		t.Fatalf("err=%v", err)
	}
}

func TestLogsCommandTailsAndRequiresClearConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.ProjectHomeEnv, home)
	path := filepath.Join(home, "run", config.ProjectName+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"logs", "--lines", "2"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.String() != "two\nthree\n" {
		t.Fatalf("output=%q", out.String())
	}

	root = NewRootCommand()
	root.SetArgs([]string{"logs", "archive", "clear"})
	if err := root.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("err=%v", err)
	}
}

func TestCLISingleDashLongFlagIsRejected(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"status", "-json"})
	if err := root.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("err=%v", err)
	}
}

func TestCLICommandsExposeStaticMetadata(t *testing.T) {
	for _, command := range cli.Commands() {
		if command.Short == "" {
			t.Errorf("%s has no Short description", command.Name())
		}
		if command.ValidArgsFunction == nil {
			t.Errorf("%s has no positional completion policy", command.Name())
		}
		for _, flag := range []string{"json", "socket"} {
			if command.Flags().Lookup(flag) == nil {
				t.Errorf("%s missing --%s", command.Name(), flag)
			}
		}
	}
}

type fakeCLIControl struct {
	controlv1connect.UnimplementedControlServiceHandler
	mu     sync.Mutex
	target string
}

func (f *fakeCLIControl) StartRuntime(_ context.Context, req *controlv1.RuntimeTargetRequest) (*controlv1.CommandResponse, error) {
	f.mu.Lock()
	f.target = req.GetTarget()
	f.mu.Unlock()
	return &controlv1.CommandResponse{Ok: true, Result: &controlv1.CommandResult{
		Target: req.GetTarget(), Action: "start", Status: "succeeded",
	}}, nil
}

func (f *fakeCLIControl) calledTarget() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.target
}

func serveCLIControl(t *testing.T, svc controlv1connect.ControlServiceHandler) string {
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
