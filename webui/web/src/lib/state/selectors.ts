import { basename } from '../formatting';
import type { ModelPreset } from '../types';

export function presetTarget(preset: ModelPreset) {
  const model = preset.entries?.find((e) => e.key === 'model' || e.key === 'models-dir')?.value || '';
  return basename(model) || 'unconfigured';
}

export function isPresetActive(activePresetNames: string[], name: string) {
  return activePresetNames.includes(name);
}
