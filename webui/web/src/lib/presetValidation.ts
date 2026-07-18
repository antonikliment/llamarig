import { createLlamaServerParamLookup, type LlamaServerParam } from './data/llamaServerParams';
import type { PresetEntry } from './types';

// Keys the router itself consumes (directory-mode scanning, model source) rather
// than passing through as llama-server CLI flags, so they won't appear in the
// llama-server flag catalog but are still legitimate in models.ini.
export const ROUTER_PRESET_KEYS = new Set(['model', 'models-dir', 'models-max', 'models-preset']);

function normalizedPresetKey(key: string): string | null {
  const trimmed = key.trim();
  if (!trimmed) return '';
  if (trimmed.startsWith('LLAMA_ARG_')) {
    if (trimmed !== trimmed.toUpperCase()) return null;
    return trimmed.slice('LLAMA_ARG_'.length).replace(/_/g, '-').toLowerCase();
  }
  return trimmed === trimmed.toLowerCase() && !trimmed.startsWith('-') ? trimmed : null;
}

// isKnownPresetKey checks a preset entry key against the router meta keys plus
// the llama-server flag catalog (aliases included), so entries can't silently
// reference a flag llama-server will reject at process start — the ini write
// and llama.cpp's own arg parsing don't catch this, they just fail the launch.
export function isKnownPresetKey(key: string, params: LlamaServerParam[]): boolean {
  const normalized = normalizedPresetKey(key);
  if (normalized === '') return true;
  if (!normalized) return false;
  if (ROUTER_PRESET_KEYS.has(normalized)) return true;
  return Boolean(createLlamaServerParamLookup(params)(normalized));
}

export function unknownPresetKeys(entries: PresetEntry[], params: LlamaServerParam[]): string[] {
  const unknown: string[] = [];
  for (const entry of entries) {
    const trimmed = entry.key.trim();
    if (!trimmed) continue;
    if (!isKnownPresetKey(trimmed, params) && !unknown.includes(trimmed)) unknown.push(trimmed);
  }
  return unknown;
}
