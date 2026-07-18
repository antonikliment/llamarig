package modelcatalog

import (
	"strings"
	"testing"
)

func TestEstimateFileFitUsesGPUCapacity(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	file := estimateFileFit(File{SizeBytes: 8 * gib}, MachineProfile{HasGPU: true, GPUName: "large", VRAMBytes: 10 * gib, AvailableRAMBytes: gib})
	if file.FitLevel != "fits" || file.EstimatedVRAMBytes == 0 || file.EstimatedVRAMBytes != file.EstimatedRAMBytes {
		t.Fatalf("file = %#v", file)
	}
	if !strings.Contains(file.FitReason, "VRAM") {
		t.Fatalf("reason = %q", file.FitReason)
	}
}

func TestEstimateFileFitFallsBackToAvailableRAM(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	file := estimateFileFit(File{SizeBytes: 2 * gib}, MachineProfile{AvailableRAMBytes: 4 * gib})
	if file.FitLevel != "fits" || file.EstimatedVRAMBytes != 0 || !strings.Contains(file.FitReason, "RAM") {
		t.Fatalf("file = %#v", file)
	}
}
