import { formatBytes, formatContextLength, formatParamCount } from '../../lib/formatting';
import type { CatalogModel, GPUStats, LocalModel, MachineProfile, SignalsSnapshot } from '../../lib/types';

export function modelMetadataChips(model: CatalogModel): { primary: string[]; capability: string[] } {
  const tags = model.tags || [];
  const primary = [
    formatParamCount(model.params),
    model.is_moe ? 'MoE' : '',
    model.context_length ? `${formatContextLength(model.context_length)} ctx` : ''
  ].filter(Boolean);
  const capability = new Set<string>();
  for (const tag of tags) {
    const value = tag.toLowerCase();
    if (value.includes('image') || value.includes('vision') || value.includes('multimodal')) capability.add('vision');
    if (value.includes('agent') || value.includes('tool') || value.includes('function-calling')) capability.add('agentic');
    if (value.includes('embedding') || value.includes('sentence-transformers')) capability.add('embedding');
  }
  if (model.license) capability.add(model.license);
  return { primary, capability: Array.from(capability) };
}

function largestGPU(signals: SignalsSnapshot | null): GPUStats | undefined {
  return (signals?.gpu || []).reduce<GPUStats | undefined>(
    (best, gpu) => ((gpu.total_vram_bytes || 0) > (best?.total_vram_bytes || 0) ? gpu : best),
    undefined
  );
}

export function bestGPUTotalVRAM(signals: SignalsSnapshot | null) {
  return largestGPU(signals)?.total_vram_bytes || 0;
}

export function rankedResourceSummary(machine: MachineProfile | null, signals: SignalsSnapshot | null) {
  const ram = machine?.available_ram_bytes || signals?.memory?.available_bytes;
  const vram = machine?.vram_bytes || bestGPUTotalVRAM(signals);
  return `${ram ? `${formatBytes(ram)} RAM available` : 'RAM unknown'} / ${vram ? `${formatBytes(vram)} VRAM capacity` : 'RAM-only ranking'}`;
}

export type FitEstimate = {
  level: 'fits' | 'marginal' | 'too_large';
  needBytes: number;
  capacityBytes: number;
  usedBytes: number;
  usedPct: number;
  needPct: number;
  target: 'VRAM' | 'RAM';
};

const FOOTPRINT_OVERHEAD = 1.1;

function fitLevelFor(needBytes: number, freeBytes: number): FitEstimate['level'] {
  if (needBytes <= freeBytes) return 'fits';
  if (needBytes <= freeBytes * 1.1) return 'marginal';
  return 'too_large';
}

// estimateLocalFit approximates whether a downloaded GGUF will fit against the
// machine's raw hardware capacity (total VRAM, falling back to total system
// RAM when there's no GPU) rather than capacity currently free. Currently-free
// capacity is misleading here: it's usually depressed by whichever model is
// already loaded, which frees up once the router switches to a different one.
export function estimateLocalFit(model: LocalModel, signals: SignalsSnapshot | null, machine: MachineProfile | null): FitEstimate | null {
  const sizeBytes = model.size_bytes || 0;
  if (!sizeBytes) return null;
  const needBytes = sizeBytes * FOOTPRINT_OVERHEAD;

  const gpu = largestGPU(signals);
  const hasGpu = machine?.has_gpu || Boolean(gpu?.total_vram_bytes);
  if (hasGpu) {
    const capacityBytes = machine?.vram_bytes || gpu?.total_vram_bytes || 0;
    const usedBytes = gpu?.used_vram_bytes || 0;
    return {
      level: fitLevelFor(needBytes, capacityBytes),
      needBytes,
      capacityBytes,
      usedBytes,
      usedPct: capacityBytes ? Math.min(100, (usedBytes / capacityBytes) * 100) : 0,
      needPct: capacityBytes ? Math.min(100, (needBytes / capacityBytes) * 100) : 0,
      target: 'VRAM'
    };
  }

  const capacityBytes = machine?.total_ram_bytes || signals?.memory?.total_bytes || 0;
  const usedBytes = signals?.memory?.used_bytes || 0;
  return {
    level: fitLevelFor(needBytes, capacityBytes),
    needBytes,
    capacityBytes,
    usedBytes,
    usedPct: capacityBytes ? Math.min(100, (usedBytes / capacityBytes) * 100) : 0,
    needPct: capacityBytes ? Math.min(100, (needBytes / capacityBytes) * 100) : 0,
    target: 'RAM'
  };
}

export function quantFromFilename(filename: string | undefined | null): string {
  if (!filename) return '';
  const match = filename.match(/(Q\d(?:_[A-Z0-9]+)*|F16|F32|BF16|IQ\d(?:_[A-Z0-9]+)*)/i);
  return match ? match[1].toUpperCase() : '';
}

export type LocalModelFilter = 'all' | 'serving' | 'in_preset' | 'unused';

export function filterLocalModels(models: LocalModel[], filter: LocalModelFilter, activePresetNames: string[]) {
  if (filter === 'all') return models;
  const active = new Set(activePresetNames);
  return models.filter((model) => {
    const presets = model.used_by_presets || [];
    const serving = presets.some((name) => active.has(name));
    if (filter === 'serving') return serving;
    if (filter === 'in_preset') return presets.length > 0;
    return presets.length === 0;
  });
}

export function localModelFilterCounts(models: LocalModel[], activePresetNames: string[]) {
  const active = new Set(activePresetNames);
  let serving = 0;
  let inPreset = 0;
  let unused = 0;
  for (const model of models) {
    const presets = model.used_by_presets || [];
    if (presets.some((name) => active.has(name))) serving += 1;
    if (presets.length > 0) inPreset += 1;
    else unused += 1;
  }
  return { all: models.length, serving, in_preset: inPreset, unused };
}

export type FitBadge = { label: string; class: string };

// fitBadge maps a model/file fit level to a colour-coded badge so headroom is
// visible at a glance, reusing the success/warning/destructive token language.
export function fitBadge(level: string | undefined): FitBadge | null {
  switch ((level || '').toLowerCase()) {
    case 'fits':
      return { label: 'Fits', class: 'bg-success/15 text-success border-success/30' };
    case 'marginal':
      return { label: 'Marginal', class: 'bg-warning/15 text-warning-foreground border-warning/30 dark:text-warning' };
    case 'too_large':
    case 'too large':
    case 'oversized':
      return { label: 'Too large', class: 'bg-destructive/15 text-destructive border-destructive/30' };
    default:
      // Unknown / not-estimated levels get no badge rather than a misleading red one.
      return null;
  }
}

// fitMeterColor maps a fit level to the bar color used in the stacked usage meter.
export function fitMeterColor(level: string | undefined): string {
  switch ((level || '').toLowerCase()) {
    case 'fits':
      return 'bg-success';
    case 'marginal':
      return 'bg-warning';
    case 'too_large':
    case 'too large':
    case 'oversized':
      return 'bg-destructive';
    default:
      return 'bg-muted-foreground';
  }
}

export function downloadStatusClass(state: string | undefined, error: string | undefined) {
  if (error || state === 'failed') return 'bg-destructive/15 text-destructive border-destructive/30';
  if (state === 'completed' || state === 'already_downloaded') return 'bg-success/15 text-success border-success/30';
  return undefined;
}
