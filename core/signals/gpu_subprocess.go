package signals

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

func collectNVIDIAGPU(ctx context.Context, runner commandRunner) ([]GPUStats, []string) {
	out, err := runner.Run(ctx, "nvidia-smi", "--query-gpu=name,memory.total,memory.used,utilization.gpu,temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, []string{"nvidia-smi not found; GPU telemetry unavailable"}
		}
		return nil, []string{fmt.Sprintf("nvidia-smi failed: %v", err)}
	}
	gpus, err := parseNVIDIASMI(out)
	if err != nil {
		return nil, []string{err.Error()}
	}
	return gpus, nil
}

func parseNVIDIASMI(out []byte) ([]GPUStats, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	gpus := make([]GPUStats, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 5 {
			return nil, fmt.Errorf("parse nvidia-smi: expected 5 columns, got %d", len(parts))
		}
		total, err := parseMiB(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse nvidia-smi total VRAM: %w", err)
		}
		used, err := parseMiB(parts[2])
		if err != nil {
			return nil, fmt.Errorf("parse nvidia-smi used VRAM: %w", err)
		}
		util, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if err != nil {
			return nil, fmt.Errorf("parse nvidia-smi utilization: %w", err)
		}
		var temperature *float64
		if value, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err == nil {
			temperature = &value
		}
		gpus = append(gpus, GPUStats{Name: strings.TrimSpace(parts[0]), Backend: "nvidia", TotalVRAMBytes: total, UsedVRAMBytes: used, UtilizationPercent: util, TemperatureCelsius: temperature, Source: "nvidia-smi"})
	}
	return gpus, nil
}

func parseMiB(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed * 1024 * 1024, nil
}
