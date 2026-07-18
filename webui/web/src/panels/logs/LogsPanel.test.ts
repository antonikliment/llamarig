import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { createLlamaRigState } from '../../lib/state/createLlamaRigState.svelte';
import type { LlamaRigClient } from '../../lib/setup/createLlamaRigClient.svelte';
import LogsPanel from './LogsPanel.svelte';

describe('LogsPanel', () => {
  it('switches sources, pauses live tail, and lists archives', async () => {
    const state = createLlamaRigState();
    state.logArchives = [{ id: 'gateway-20260703T120000.000000000Z.log', source: 'gateway', size_bytes: 42, archived_at: '2026-07-03T12:00:00Z' }];
    const app = {
      state,
      refreshLogs: vi.fn(async () => undefined),
      refreshEvents: vi.fn(async () => undefined),
      resumeLogs: vi.fn(async () => undefined),
      loadLogArchives: vi.fn(async () => undefined),
      selectLogArchive: vi.fn(async () => undefined),
      deleteLogArchive: vi.fn(async () => undefined),
      clearLogArchives: vi.fn(async () => undefined)
    } as unknown as LlamaRigClient;

    render(LogsPanel, { app });
    await userEvent.click(screen.getByRole('tab', { name: 'Gateway' }));
    expect(state.logSource).toBe('gateway');
    await userEvent.click(screen.getByRole('button', { name: 'Pause' }));
    expect(state.logPaused).toBe(true);
    await userEvent.click(screen.getByRole('tab', { name: /Archives/ }));
    expect(screen.getByText(/42 bytes/)).toBeInTheDocument();
  });
});
