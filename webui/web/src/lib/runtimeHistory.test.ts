import { describe, expect, it, vi } from 'vitest';
import { appendRuntimeSample, type RuntimeHistorySample } from './runtimeHistory';

describe('appendRuntimeSample', () => {
  it('keeps timestamps, missing values, and independent GPU samples', () => {
    const history = appendRuntimeSample([], {
      captured_at: '2026-07-11T12:00:00Z',
      cpu: { used_percent: 125 },
      memory: {},
      gpu: [
        { name: 'A', backend: 'nvidia', utilization_percent: 40, total_vram_bytes: 100, used_vram_bytes: 25, temperature_celsius: 63 },
        { name: 'B', backend: 'nvidia' }
      ]
    });

    expect(history[0]).toMatchObject({ capturedAt: '2026-07-11T12:00:00Z', cpu: 100, memory: null });
    expect(history[0].gpu[0]).toMatchObject({ key: 'nvidia:A:0', utilization: 40, vram: 25, temperature: 63 });
    expect(history[0].gpu[1]).toMatchObject({ utilization: null, vram: null, temperature: null });
  });

  it('deduplicates captures and caps history at 60 samples', () => {
    vi.useFakeTimers();
    let history: RuntimeHistorySample[] = [];
    for (let index = 0; index < 61; index++) {
      history = appendRuntimeSample(history, { captured_at: String(index), cpu: { used_percent: index } });
    }
    expect(history).toHaveLength(60);
    expect(history[0].capturedAt).toBe('1');
    expect(appendRuntimeSample(history, { captured_at: '60' })).toBe(history);
    vi.useRealTimers();
  });
});
