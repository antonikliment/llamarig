package rpc

import (
	"testing"

	"llamarig/core/signals"
)

func TestMachineProfileSelectsLargestGPU(t *testing.T) {
	profile := machineProfile(signals.MachineSnapshot{
		Memory: signals.MemoryStats{TotalBytes: 32, AvailableBytes: 16},
		GPU: []signals.GPUStats{
			{Name: "small", TotalVRAMBytes: 8},
			{Name: "large", TotalVRAMBytes: 16},
		},
	})
	if profile.TotalRAMBytes != 32 || profile.AvailableRAMBytes != 16 || !profile.HasGPU || profile.GPUName != "large" || profile.VRAMBytes != 16 {
		t.Fatalf("profile = %#v", profile)
	}
}
