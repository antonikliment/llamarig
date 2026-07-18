import type { ModelPreset } from '../../lib/types';

export function uniquePresetName(filename: string, presets: ModelPreset[]) {
  const stem = filename.replace(/\.gguf$/i, '');
  const base = stem.replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^-+|-+$/g, '') || 'model';
  const used = new Set(presets.map((p) => p.name));
  if (!used.has(base)) return base;
  for (let suffix = 2; ; suffix += 1) {
    const candidate = `${base}-${suffix}`;
    if (!used.has(candidate)) return candidate;
  }
}
