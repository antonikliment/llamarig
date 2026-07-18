import { describe, expect, it } from 'vitest';
import { templateEntries } from '../../lib/presetTemplates';

describe('templateEntries', () => {
  it('creates single-model entries by default', () => {
    const entries = templateEntries('single');
    expect(entries[0]).toEqual({ key: 'model', value: '/path/to/model.gguf' });
    expect(entries.some((e) => e.key === 'ctx-size')).toBe(true);
    expect(entries.some((e) => e.key === 'flash-attn')).toBe(true);
    expect(entries.some((e) => ['fa', 't', 'tb', 'ub'].includes(e.key))).toBe(false);
    expect(entries.some((e) => e.key === 'prio')).toBe(true);
  });

  it('creates directory entries', () => {
    const entries = templateEntries('directory');
    expect(entries[0]).toEqual({ key: 'models-dir', value: '/path/to/models' });
    expect(entries.some((e) => e.key === 'models-max')).toBe(true);
  });

  it('creates blank entry', () => {
    const entries = templateEntries('blank');
    expect(entries).toEqual([{ key: 'model', value: '/path/to/model.gguf' }]);
  });
});
