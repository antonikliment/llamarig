import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createLlamaRigState } from '../../lib/state/createLlamaRigState.svelte';
import Harness from '../../test/AppShellHarness.svelte';

const { setMode, resetMode } = vi.hoisted(() => ({ setMode: vi.fn(), resetMode: vi.fn() }));

vi.mock('mode-watcher', () => ({ setMode, resetMode }));

describe('AppShell', () => {
  beforeEach(() => {
    setMode.mockClear();
    resetMode.mockClear();
    localStorage.clear();
    document.documentElement.removeAttribute('style');
  });

  // The dropdown and sheet use bits-ui's body-scroll-lock, which schedules a
  // setTimeout cleanup on unmount. Unmount explicitly and let that timer fire
  // while the document still exists, instead of after jsdom is torn down.
  afterEach(async () => {
    cleanup();
    await new Promise((resolve) => setTimeout(resolve, 50));
  });

  it('navigates through shadcn sidebar controls', async () => {
    const app = createLlamaRigState();
    render(Harness, { app });

    await userEvent.click(screen.getByRole('button', { name: 'Presets' }));

    expect(app.activeSection).toBe('presets');
  });

  it('offers system, light, and dark theme choices', async () => {
    const app = createLlamaRigState();
    render(Harness, { app });

    await userEvent.click(screen.getByRole('button', { name: 'Choose color theme' }));
    await userEvent.click(await screen.findByText('Dark'));
    expect(setMode).toHaveBeenCalledWith('dark');

    await userEvent.click(screen.getByRole('button', { name: 'Choose color theme' }));
    await userEvent.click(await screen.findByText('System'));
    expect(resetMode).toHaveBeenCalledOnce();
  });

  it('persists separate light and dark primary overrides', async () => {
    const app = createLlamaRigState();
    render(Harness, { app });

    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));
    await fireEvent.input(screen.getByLabelText('Light mode primary'), { target: { value: '#123456' } });
    await fireEvent.input(screen.getByLabelText('Dark mode primary'), { target: { value: '#abcdef' } });

    expect(localStorage.getItem('llamarig.theme.primary.light')).toBe('#123456');
    expect(localStorage.getItem('llamarig.theme.primary.dark')).toBe('#abcdef');
    expect(document.documentElement.style.getPropertyValue('--user-primary-light')).toBe('#123456');

    await userEvent.click(screen.getByRole('button', { name: 'Reset theme colors' }));
    expect(localStorage.getItem('llamarig.theme.primary.light')).toBeNull();
  });
});
