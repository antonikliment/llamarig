import { describe, expect, it } from 'vitest';
import { llamaServerParams } from './data/llamaServerParams';
import { isKnownPresetKey, unknownPresetKeys } from './presetValidation';

describe('presetValidation', () => {
  it('accepts real llama-server flags', () => {
    expect(isKnownPresetKey('prio', llamaServerParams)).toBe(true);
    expect(isKnownPresetKey('ctx-size', llamaServerParams)).toBe(true);
  });

  it('accepts flag aliases', () => {
    expect(isKnownPresetKey('t', llamaServerParams)).toBe(true);
  });

  it('accepts exact LLAMA_ARG environment keys', () => {
    expect(isKnownPresetKey('LLAMA_ARG_CTX_SIZE', llamaServerParams)).toBe(true);
  });

  it('rejects key forms the preset store rejects', () => {
    expect(isKnownPresetKey('--ctx-size', llamaServerParams)).toBe(false);
    expect(isKnownPresetKey('CTX-SIZE', llamaServerParams)).toBe(false);
    expect(isKnownPresetKey('LLAMA_ARG_ctx_size', llamaServerParams)).toBe(false);
  });

  it('accepts router meta keys that are not llama-server flags', () => {
    expect(isKnownPresetKey('models-dir', llamaServerParams)).toBe(true);
    expect(isKnownPresetKey('models-max', llamaServerParams)).toBe(true);
  });

  it('rejects a typo like priority instead of prio', () => {
    expect(isKnownPresetKey('priority', llamaServerParams)).toBe(false);
  });

  it('treats an empty key as valid (not yet filled in)', () => {
    expect(isKnownPresetKey('', llamaServerParams)).toBe(true);
  });

  it('collects unique unknown keys from a draft', () => {
    const entries = [
      { key: 'model', value: '/m.gguf' },
      { key: 'priority', value: '2' },
      { key: 'priority', value: '2' },
      { key: 'bogus-flag', value: 'x' },
      { key: '', value: '' }
    ];
    expect(unknownPresetKeys(entries, llamaServerParams)).toEqual(['priority', 'bogus-flag']);
  });
});
