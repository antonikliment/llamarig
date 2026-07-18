import type { SignalsSnapshot } from './types';

export type RuntimeHistorySample = {
  capturedAt: string;
  cpu: number | null;
  memory: number | null;
  gpu: Array<{
    key: string;
    utilization: number | null;
    vram: number | null;
    temperature: number | null;
  }>;
};

export function appendRuntimeSample(
  history: RuntimeHistorySample[],
  signals: SignalsSnapshot | null,
  limit = 60
) {
  if (!signals) return history;
  const capturedAt = signals.captured_at || new Date().toISOString();
  if (history[history.length - 1]?.capturedAt === capturedAt) return history;
  const memory = signals.memory;
  return [
    ...history,
    {
      capturedAt,
      cpu: value(signals.cpu?.used_percent),
      memory: value(
        memory?.used_percent ??
          (memory?.total_bytes
            ? (((memory.total_bytes || 0) - (memory.available_bytes || 0)) / memory.total_bytes) * 100
            : null)
      ),
      gpu: (signals.gpu || []).map((gpu, index) => ({
        key: gpuKey(gpu.backend, gpu.name, index),
        utilization: value(gpu.utilization_percent),
        vram: value(gpu.total_vram_bytes ? ((gpu.used_vram_bytes || 0) / gpu.total_vram_bytes) * 100 : null),
        temperature: value(gpu.temperature_celsius, false)
      }))
    }
  ].slice(-limit);
}

export function gpuKey(backend: string | undefined, name: string | undefined, index: number) {
  return `${backend || 'gpu'}:${name || 'device'}:${index}`;
}

function value(input: number | null | undefined, clamp = true) {
  if (input == null || !Number.isFinite(input)) return null;
  return clamp ? Math.max(0, Math.min(100, input)) : input;
}
