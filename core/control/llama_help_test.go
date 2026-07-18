package control

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

const sampleLlamaServerHelp = `----- common params -----

-h,    --help, --usage                  print usage and exit
    --version                           show version and build info
    -t,    --threads N                  number of threads to use during generation (default: -1)
    -tb,   --threads-batch N            number of threads to use during batch and prompt processing
                                         (default: same as --threads)
    -C,    --cpu-mask M                 CPU affinity mask: arbitrarily long hex
    -ngl,  --gpu-layers, --n-gpu-layers N
                                         number of layers to store in VRAM
           --no-mmap                    do not memory-map model (slower load but may reduce pageouts
                                         if not using mmap) (default: false)

----- server params -----

           --host HOST                  ip address to listen (default: 127.0.0.1)
           --port PORT                  port to listen (default: 8080)
`

func buildHelpParamIndex(t *testing.T) map[string]LlamaServerParam {
	t.Helper()
	params := parseLlamaServerHelp(sampleLlamaServerHelp)
	if len(params) == 0 {
		t.Fatal("expected at least one parsed param")
	}
	byKey := map[string]LlamaServerParam{}
	for _, p := range params {
		byKey[p.Key] = p
	}
	return byKey
}

type blockingHelpRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r blockingHelpRunner) Run(context.Context, string, ...string) ([]byte, error) {
	r.started <- struct{}{}
	<-r.release
	return []byte(sampleLlamaServerHelp), nil
}

func TestManagerGetLlamaServerParamsDoesNotLockAcrossSubprocess(t *testing.T) {
	runner := blockingHelpRunner{started: make(chan struct{}, 2), release: make(chan struct{})}
	manager := NewManager(Dependencies{})
	manager.helpRunner = runner
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := manager.GetLlamaServerParams(context.Background())
			errs <- err
		}()
	}
	for range 2 {
		select {
		case <-runner.started:
		case <-time.After(time.Second):
			t.Fatal("concurrent cache miss blocked behind cache mutex")
		}
	}
	close(runner.release)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := manager.GetLlamaServerParams(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.started:
		t.Fatal("cache hit launched another subprocess")
	default:
	}
}

func TestParseLlamaServerHelpThreads(t *testing.T) {
	byKey := buildHelpParamIndex(t)
	threads, ok := byKey["threads"]
	if !ok {
		t.Fatalf("expected to find threads param, got %#v", byKey)
	}
	if threads.ValueHint != "N" || threads.Default != "-1" {
		t.Fatalf("threads = %#v", threads)
	}
}

func TestParseLlamaServerHelpThreadsBatch(t *testing.T) {
	byKey := buildHelpParamIndex(t)
	threadsBatch, ok := byKey["threads-batch"]
	if !ok {
		t.Fatalf("expected to find threads-batch param")
	}
	if threadsBatch.Default != "same as --threads" {
		t.Fatalf("threads-batch did not absorb continuation line: %#v", threadsBatch)
	}
}

func TestParseLlamaServerHelpGpuLayers(t *testing.T) {
	byKey := buildHelpParamIndex(t)
	gpuLayers, ok := byKey["gpu-layers"]
	if !ok {
		t.Fatalf("expected gpu-layers as canonical key, got %#v", byKey)
	}
	found := false
	for _, alias := range gpuLayers.Aliases {
		if alias == "n-gpu-layers" || alias == "ngl" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ngl/n-gpu-layers as aliases, got %#v", gpuLayers.Aliases)
	}
	if gpuLayers.Description == "" {
		t.Fatal("expected description carried over from continuation line")
	}
}

func TestParseLlamaServerHelpNoMmap(t *testing.T) {
	byKey := buildHelpParamIndex(t)
	noMmap, ok := byKey["no-mmap"]
	if !ok {
		t.Fatalf("expected no-mmap param")
	}
	if noMmap.Default != "false" {
		t.Fatalf("no-mmap = %#v", noMmap)
	}
}

func TestParseLlamaServerHelpHost(t *testing.T) {
	byKey := buildHelpParamIndex(t)
	host, ok := byKey["host"]
	if !ok || host.Default != "127.0.0.1" {
		t.Fatalf("host = %#v, ok=%v", host, ok)
	}
}

func TestParseLlamaServerHelpEmpty(t *testing.T) {
	if params := parseLlamaServerHelp(""); len(params) != 0 {
		t.Fatalf("expected no params, got %#v", params)
	}
}

type fakeHelpRunner struct {
	out []byte
	err error
}

func (f fakeHelpRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return f.out, f.err
}

func TestManagerGetLlamaServerParams(t *testing.T) {
	manager := NewManager(Dependencies{})
	manager.helpRunner = fakeHelpRunner{out: []byte(sampleLlamaServerHelp)}
	params, err := manager.GetLlamaServerParams(context.Background())
	if err != nil {
		t.Fatalf("GetLlamaServerParams returned error: %v", err)
	}
	if len(params) == 0 {
		t.Fatal("expected parsed params")
	}
}

func TestManagerGetLlamaServerParamsBinaryMissing(t *testing.T) {
	manager := NewManager(Dependencies{})
	manager.helpRunner = fakeHelpRunner{err: exec.ErrNotFound}
	if _, err := manager.GetLlamaServerParams(context.Background()); err == nil {
		t.Fatal("expected error when binary is missing")
	}
}

func TestManagerGetLlamaServerParamsUnparseable(t *testing.T) {
	manager := NewManager(Dependencies{})
	manager.helpRunner = fakeHelpRunner{out: []byte("\n\n")}
	if _, err := manager.GetLlamaServerParams(context.Background()); err == nil {
		t.Fatal("expected error when no flags can be parsed")
	}
}
