package modelcatalog

import "fmt"

const fitOverheadBytes int64 = 512 * 1024 * 1024

func estimateFileFit(file File, machine MachineProfile) File {
	required := file.SizeBytes + fitOverheadBytes
	file.EstimatedRAMBytes = required
	if machine.HasGPU {
		file.EstimatedVRAMBytes = required
	}
	fit := estimateFit(required, machine)
	file.FitLevel = fit.Level
	file.FitReason = fit.Reason
	return file
}

func estimateFit(required int64, machine MachineProfile) MachineFit {
	capacity, resource := machine.AvailableRAMBytes, "RAM"
	fit := MachineFit{Level: "unknown", RequiredRAMBytes: required, AvailableRAMBytes: machine.AvailableRAMBytes}
	if machine.HasGPU {
		capacity, resource = machine.VRAMBytes, "VRAM"
		fit.RequiredVRAMBytes, fit.AvailableVRAMBytes = required, capacity
	}
	if required <= 0 || capacity <= 0 {
		fit.Reason = "memory capacity is unknown"
		return fit
	}
	if required <= int64(float64(capacity)*0.90) {
		fit.Level = "fits"
		fit.Reason = fmt.Sprintf("estimated %s %.1f GiB is within 90%% of capacity", resource, gib(required))
		return fit
	}
	if required <= capacity {
		fit.Level = "marginal"
		fit.Reason = fmt.Sprintf("estimated %s %.1f GiB is close to capacity", resource, gib(required))
		return fit
	}
	fit.Level = "too_large"
	fit.Reason = fmt.Sprintf("estimated %s %.1f GiB exceeds capacity", resource, gib(required))
	return fit
}

func gib(value int64) float64 {
	return float64(value) / (1024 * 1024 * 1024)
}

func fitRank(level string) int {
	switch level {
	case "fits":
		return 4
	case "marginal":
		return 3
	case "unknown":
		return 2
	case "too_large":
		return 1
	default:
		return 0
	}
}

func passesMinFit(level string, minFit string) bool {
	switch minFit {
	case "fits":
		return level == "fits"
	case "marginal":
		return level == "fits" || level == "marginal"
	default:
		return true
	}
}
