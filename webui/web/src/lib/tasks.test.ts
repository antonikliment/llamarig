import { describe, expect, it } from 'vitest';
import { canApplyDownload, choosePresetSelection, isTerminalDownloadState } from './tasks';

describe('download states', () => {
  it('knows terminal and apply states', () => {
    expect(isTerminalDownloadState('completed')).toBe(true);
    expect(isTerminalDownloadState('running')).toBe(false);
    expect(canApplyDownload('already_downloaded')).toBe(true);
    expect(canApplyDownload('failed')).toBe(false);
  });
});

describe('choosePresetSelection', () => {
  const presets = [{ name: 'default' }, { name: 'active' }, { name: 'other' }];

  it('prefers explicit selection', () => {
    expect(choosePresetSelection(presets, 'other', ['active'], 'default', '')).toBe('other');
  });

  it('falls back through active, default, selected, first', () => {
    expect(choosePresetSelection(presets, '', ['active'], 'default', '')).toBe('active');
    expect(choosePresetSelection(presets, '', [], 'default', '')).toBe('default');
    expect(choosePresetSelection(presets, '', [], '', 'other')).toBe('other');
    expect(choosePresetSelection(presets, '', [], '', '')).toBe('default');
  });
});
