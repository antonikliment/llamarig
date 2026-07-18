import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { createLlamaRigState } from '../../lib/state/createLlamaRigState.svelte';
import ModelsPanel from './ModelsPanel.svelte';

describe('ModelsPanel', () => {
  it('shows partial catalog errors without hiding successful content', async () => {
    const state = createLlamaRigState();
    state.catalogErrors = ['owner/broken: temporary failure'];
    const app = {
      state,
      activeDownload: () => null,
      canApplyDownload: () => false,
      refreshResourcesAndCatalog: vi.fn(),
      loadModelCatalog: vi.fn(),
      loadLocalModels: vi.fn()
    };
    render(ModelsPanel, { app: app as never });
    await userEvent.click(screen.getByRole('tab', { name: 'Catalog' }));
    expect(screen.getByRole('alert')).toHaveTextContent('1 catalog model could not be loaded');
    await userEvent.click(screen.getByRole('button', { name: 'Show details' }));
    expect(screen.getByText('owner/broken: temporary failure')).toBeInTheDocument();
  });
});
