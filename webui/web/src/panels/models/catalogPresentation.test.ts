import { describe, expect, it } from 'vitest';
import {
  estimateLocalFit,
  downloadStatusClass,
  filterLocalModels,
  fitBadge,
  localModelFilterCounts,
  modelMetadataChips,
  quantFromFilename,
  rankedResourceSummary
} from './catalogPresentation';

describe('catalogPresentation', () => {
  it('builds metadata chips from tags and license', () => {
    expect(
      modelMetadataChips({
        id: 'a/b',
        owner: 'a',
        repo: 'b',
        url: '',
        params: 7_615_616_512,
        context_length: 32768,
        is_moe: true,
        tags: ['text-generation', 'function-calling', 'vision'],
        license: 'mit'
      })
    ).toEqual({
      primary: ['7.6B', 'MoE', '32K ctx'],
      capability: ['agentic', 'vision', 'mit']
    });
  });

  it('formats ranked resources', () => {
    expect(rankedResourceSummary({ available_ram_bytes: 1024, vram_bytes: 2048 }, null)).toContain('2.0 KB VRAM capacity');
  });

  it('maps known fit levels and hides unknown ones', () => {
    expect(fitBadge('fits')?.label).toBe('Fits');
    expect(fitBadge('marginal')?.label).toBe('Marginal');
    expect(fitBadge('too_large')?.label).toBe('Too large');
    expect(fitBadge('unknown')).toBeNull();
    expect(fitBadge('')).toBeNull();
    expect(fitBadge(undefined)).toBeNull();
  });

  it('maps download states emitted by the API', () => {
    expect(downloadStatusClass('completed', undefined)).toContain('success');
    expect(downloadStatusClass('already_downloaded', undefined)).toContain('success');
    expect(downloadStatusClass('failed', undefined)).toContain('destructive');
    expect(downloadStatusClass('cancelled', undefined)).toBeUndefined();
  });

  it('estimates local fit against free VRAM', () => {
    const model = { path: '/m/a.gguf', filename: 'a.gguf', size_bytes: 8_000_000_000 };
    const estimate = estimateLocalFit(
      model,
      { gpu: [{ total_vram_bytes: 16_000_000_000, used_vram_bytes: 2_000_000_000 }] },
      { has_gpu: true }
    );
    expect(estimate?.target).toBe('VRAM');
    expect(estimate?.level).toBe('fits');
  });

  it('falls back to RAM when there is no GPU', () => {
    const model = { path: '/m/a.gguf', filename: 'a.gguf', size_bytes: 60_000_000_000 };
    const estimate = estimateLocalFit(model, { memory: { total_bytes: 32_000_000_000, available_bytes: 4_000_000_000 } }, { has_gpu: false });
    expect(estimate?.target).toBe('RAM');
    expect(estimate?.level).toBe('too_large');
  });

  it('ignores currently-used capacity when deciding fit, since it reflects the model already loaded', () => {
    const model = { path: '/m/a.gguf', filename: 'a.gguf', size_bytes: 8_000_000_000 };
    const estimate = estimateLocalFit(
      model,
      { gpu: [{ total_vram_bytes: 16_000_000_000, used_vram_bytes: 14_000_000_000 }] },
      { has_gpu: true }
    );
    expect(estimate?.level).toBe('fits');
  });

  it('estimates fit against the GPU with the largest VRAM', () => {
    const estimate = estimateLocalFit(
      { path: '/m/a.gguf', filename: 'a.gguf', size_bytes: 10_000_000_000 },
      {
        gpu: [
          { total_vram_bytes: 8_000_000_000, used_vram_bytes: 7_000_000_000 },
          { total_vram_bytes: 16_000_000_000, used_vram_bytes: 2_000_000_000 }
        ]
      },
      { has_gpu: true }
    );
    expect(estimate).toMatchObject({ level: 'fits', capacityBytes: 16_000_000_000, usedBytes: 2_000_000_000 });
  });

  it('returns null when size is unknown', () => {
    expect(estimateLocalFit({ path: '/m/a.gguf', filename: 'a.gguf' }, null, null)).toBeNull();
  });

  it('parses quant from filename', () => {
    expect(quantFromFilename('ornith-9b-mtp-kl-Q8_0.gguf')).toBe('Q8_0');
    expect(quantFromFilename('model-f16.gguf')).toBe('F16');
    expect(quantFromFilename('model.gguf')).toBe('');
  });

  it('filters local models by serving/preset status', () => {
    const models = [
      { path: '/a', filename: 'a.gguf', used_by_presets: ['p1'] },
      { path: '/b', filename: 'b.gguf', used_by_presets: ['p2'] },
      { path: '/c', filename: 'c.gguf', used_by_presets: [] }
    ];
    expect(filterLocalModels(models, 'serving', ['p1']).map((m) => m.filename)).toEqual(['a.gguf']);
    expect(filterLocalModels(models, 'in_preset', ['p1']).map((m) => m.filename)).toEqual(['a.gguf', 'b.gguf']);
    expect(filterLocalModels(models, 'unused', ['p1']).map((m) => m.filename)).toEqual(['c.gguf']);
    expect(filterLocalModels(models, 'all', ['p1'])).toBe(models);
  });

  it('counts local models by filter bucket', () => {
    const models = [
      { path: '/a', filename: 'a.gguf', used_by_presets: ['p1'] },
      { path: '/b', filename: 'b.gguf', used_by_presets: ['p2'] },
      { path: '/c', filename: 'c.gguf', used_by_presets: [] }
    ];
    expect(localModelFilterCounts(models, ['p1'])).toEqual({ all: 3, serving: 1, in_preset: 2, unused: 1 });
  });
});
