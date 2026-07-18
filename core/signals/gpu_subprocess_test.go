package signals

import (
	"context"
	"testing"
)

type machineTestRunner struct{ out []byte }

func (r machineTestRunner) Run(context.Context, string, ...string) ([]byte, error) { return r.out, nil }

func TestParseNVIDIASMI(t *testing.T) {
	gpus, err := parseNVIDIASMI([]byte("NVIDIA RTX 4090, 24564, 1024, 12, 63\n"))
	if err != nil {
		t.Fatalf("parseNVIDIASMI returned error: %v", err)
	}
	if len(gpus) != 1 {
		t.Fatalf("gpus = %#v", gpus)
	}
	if gpus[0].Name != "NVIDIA RTX 4090" || gpus[0].TotalVRAMBytes != 24564*1024*1024 || gpus[0].UsedVRAMBytes != 1024*1024*1024 || gpus[0].UtilizationPercent != 12 || gpus[0].TemperatureCelsius == nil || *gpus[0].TemperatureCelsius != 63 {
		t.Fatalf("gpu = %#v", gpus[0])
	}
}

func TestParseNVIDIASMITemperatureUnavailable(t *testing.T) {
	gpus, err := parseNVIDIASMI([]byte("NVIDIA RTX 4090, 24564, 1024, 12, [N/A]\n"))
	if err != nil || len(gpus) != 1 || gpus[0].TemperatureCelsius != nil {
		t.Fatalf("gpus = %#v, error = %v", gpus, err)
	}
}

func TestParseNVIDIASMIMalformed(t *testing.T) {
	if _, err := parseNVIDIASMI([]byte("bad,row\n")); err == nil {
		t.Fatal("expected malformed output error")
	}
}

func TestGopsutilCollectorMachineCollectsMemoryAndGPU(t *testing.T) {
	collector := &GopsutilCollector{Runner: machineTestRunner{out: []byte("NVIDIA RTX, 16384, 1024, 12, 63\n")}}
	machine, err := collector.Machine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if machine.Memory.TotalBytes == 0 || len(machine.GPU) != 1 || machine.GPU[0].TotalVRAMBytes != 16384*1024*1024 {
		t.Fatalf("machine = %#v", machine)
	}
}
