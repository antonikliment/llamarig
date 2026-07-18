import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { createLlamaRigState } from '../../lib/state/createLlamaRigState.svelte';
import type { LlamaRigClient } from '../../lib/setup/createLlamaRigClient.svelte';
import PresetsPanel from './PresetsPanel.svelte';

describe('PresetsPanel', () => {
  it('visually distinguishes selected and selectable presets', () => {
    const state = createLlamaRigState();
    state.selectedPresetName = 'coder';
    state.presets = [
      { name: 'coder', entries: [{ key: 'model', value: 'coder.gguf' }] },
      { name: 'chat', entries: [{ key: 'model', value: 'chat.gguf' }] }
    ];
    const app = {
      state,
      errorMessage: '',
      isPresetActive: () => false,
      loadPresets: vi.fn(),
      selectPreset: vi.fn()
    } as unknown as LlamaRigClient;

    render(PresetsPanel, { app });
    const selected = screen.getByRole('button', { name: /coder/ });
    const option = screen.getByRole('button', { name: /chat/ });
    expect(selected).toHaveAttribute('aria-pressed', 'true');
    expect(selected).toHaveClass('cursor-pointer');
    expect(selected.closest('[data-slot="item"]')).toHaveClass('border-primary/50', 'bg-primary/10');
    expect(option.closest('[data-slot="item"]')).toHaveClass('hover:bg-muted/50');
  });

  it('does not delete a preset before AlertDialog confirmation', async () => {
    const state = createLlamaRigState();
    state.selectedPresetName = 'coder';
    state.currentPreset = { name: 'coder', entries: [{ key: 'model', value: 'coder.gguf' }] };
    const deletePreset = vi.fn();
    const app = {
      state,
      errorMessage: '',
      deletePreset,
      isPresetActive: () => false,
      loadPresets: vi.fn(),
      selectPreset: vi.fn(),
      createPreset: vi.fn(),
      duplicatePreset: vi.fn()
    } as unknown as LlamaRigClient;

    render(PresetsPanel, { app });
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(deletePreset).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole('button', { name: 'Delete preset' }));
    expect(deletePreset).toHaveBeenCalledOnce();
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  });

  it('shows unavailable reason and confirms one-action cleanup', async () => {
    const state = createLlamaRigState();
    state.selectedPresetName = 'broken';
    state.currentPreset = {
      name: 'broken',
      entries: [{ key: 'model', value: '/missing.gguf' }],
      source_status: 'unavailable',
      source_error: 'source "/missing.gguf" does not exist'
    };
    state.presets = [state.currentPreset];
    const cleanupPreset = vi.fn();
    const app = {
      state,
      errorMessage: '',
      cleanupPreset,
      isPresetActive: () => false,
      loadPresets: vi.fn(),
      selectPreset: vi.fn(),
      createPreset: vi.fn(),
      duplicatePreset: vi.fn()
    } as unknown as LlamaRigClient;

    render(PresetsPanel, { app });
    expect(screen.getAllByText('unavailable').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/missing\.gguf/).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Start' })).toBeDisabled();

    await userEvent.click(screen.getByRole('button', { name: 'Cleanup preset' }));
    expect(cleanupPreset).not.toHaveBeenCalled();
    const cleanupButtons = await screen.findAllByRole('button', { name: 'Cleanup preset' });
    await userEvent.click(cleanupButtons[cleanupButtons.length - 1]);
    expect(cleanupPreset).toHaveBeenCalledOnce();
  });
});
