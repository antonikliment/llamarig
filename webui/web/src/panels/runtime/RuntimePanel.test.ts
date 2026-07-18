import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { tick } from 'svelte';
import { createLlamaRigState } from '../../lib/state/createLlamaRigState.svelte';
import RuntimePanel from './RuntimePanel.svelte';

function dashboard() {
  const state = createLlamaRigState();
  state.runtimeStatus = { status: 'running', detail: 'router ready', checkedAt: '2026-07-11T12:00:00Z' };
  state.activePresetNames = ['qwen'];
  state.modelsMax = 2;
  state.localModels = [{ path: '/models/qwen.gguf', filename: 'qwen.gguf' }];
  state.signals = {
    captured_at: '2026-07-11T12:00:00Z',
    cpu: { logical_cores: 16, used_percent: 25 },
    memory: { total_bytes: 100, available_bytes: 40, used_percent: 60 },
    gpu: [{ name: 'RTX', backend: 'nvidia', utilization_percent: 50, total_vram_bytes: 100, used_vram_bytes: 30, temperature_celsius: 64 }]
  };
  const app = {
    state,
    runTask: vi.fn(async (_label: string, task: () => Promise<void>) => task()),
    refreshRuntimeStatus: vi.fn(),
    refreshSignals: vi.fn(),
    refreshEvents: vi.fn(),
    restartPreset: vi.fn(),
    stopPreset: vi.fn()
  };
  render(RuntimePanel, { app: app as never });
  return { app, state };
}

describe('RuntimePanel dashboard', () => {
  it('shows compact status and all available GPU readings', () => {
    dashboard();
    expect(screen.getByText('System overview')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getAllByText('RTX').length).toBeGreaterThan(0);
    expect(screen.getByText('64°C')).toBeInTheDocument();
  });

  it('confirms a targeted stop before invoking it', async () => {
    const user = userEvent.setup();
    const { app } = dashboard();
    await user.click(screen.getByRole('button', { name: 'Stop' }));
    expect(screen.getByText('Stop qwen?')).toBeInTheDocument();
    expect(app.stopPreset).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Stop preset' }));
    expect(app.stopPreset).toHaveBeenCalledWith('qwen');
  });

  it('retains metrics while marking a failed refresh stale', async () => {
    const { state } = dashboard();
    state.signalsLastError = 'collector unavailable';
    await tick();
    expect(screen.getByText('Stale')).toBeInTheDocument();
    expect(screen.getByText(/collector unavailable/)).toBeInTheDocument();
    expect(screen.getByText('25% · 16 cores')).toBeInTheDocument();
  });

  it.each([
    ['succeeded', 'text-success'],
    ['failed', 'text-destructive'],
    ['skipped', 'text-warning-foreground']
  ])('styles %s operation status', async (status, className) => {
    const { state } = dashboard();
    state.lastOperation = { status, action: 'refresh', target: 'router' };
    await tick();
    expect(screen.getByText(status)).toHaveClass(className);
  });
});
