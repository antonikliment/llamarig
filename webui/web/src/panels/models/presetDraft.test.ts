import { describe, expect, it } from 'vitest';
import { uniquePresetName } from './presetDraft';
import type { ModelPreset } from '../../lib/types';

function preset(name: string): ModelPreset {
  return {
    name,
    entries: []
  };
}

describe('model preset draft', () => {
  it('creates a safe unique preset name', () => {
    expect(uniquePresetName('Qwen 2.5 (Q4).gguf', [preset('Qwen-2.5-Q4')])).toBe('Qwen-2.5-Q4-2');
  });
});
