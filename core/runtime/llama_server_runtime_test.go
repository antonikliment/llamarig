package runtime

import (
	"context"
	"reflect"
	"testing"
	"time"

	"llamarig/config"
)

func TestBuildRouterCommand(t *testing.T) {
	router := BuildRouter(config.RouterConfig{
		Executable:        "/usr/local/bin/llama-server",
		Host:              "127.0.0.1",
		Port:              18080,
		ModelsMax:         3,
		StopTimeout:       time.Second,
		ReadinessTimeout:  time.Second,
		ReadinessInterval: time.Millisecond,
	}, "/models", "/config/models.ini")

	command, err := router.cfg.command()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--models-dir", "/models", "--models-preset", "/config/models.ini", "--models-max", "3", "--host", "127.0.0.1", "--port", "18080"}
	if !reflect.DeepEqual(command.Argv, want) {
		t.Fatalf("argv = %#v, want %#v", command.Argv, want)
	}
}

func TestLlamaServerDefaults(t *testing.T) {
	runtime := NewLlamaServer(LlamaServerConfig{Timeout: time.Second})
	if runtime.cfg.Executable != "llama-server" || runtime.cfg.Host != "127.0.0.1" || runtime.cfg.Port != 8080 {
		t.Fatalf("defaults = %#v", runtime.cfg)
	}
}

func TestLlamaServerRejectsUnsafePIDFileName(t *testing.T) {
	runtime := NewLlamaServer(LlamaServerConfig{Name: "../router", PIDDir: t.TempDir()})
	if _, err := runtime.pidFile(); err == nil {
		t.Fatal("PID file accepted unsafe name")
	}
}

func TestLlamaServerStatusIncludesRouterProcess(t *testing.T) {
	runtime := NewLlamaServer(LlamaServerConfig{Name: "router", Host: "127.0.0.1", Port: 8080})
	status, err := runtime.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != Stopped || len(status.Processes) != 1 || status.Processes[0].Name != "router" {
		t.Fatalf("status = %#v", status)
	}
}
