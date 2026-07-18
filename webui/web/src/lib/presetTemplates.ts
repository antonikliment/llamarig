import type { PresetEntry } from './types';

export function templateEntries(kind: string): PresetEntry[] {
  if (kind === 'blank') {
    return [{ key: 'model', value: '/path/to/model.gguf' }];
  }
  if (kind === 'directory') {
    return [
      { key: 'models-dir', value: '/path/to/models' },
      { key: 'cont-batching', value: 'true' },
      { key: 'ctx-size', value: '65536' },
      { key: 'flash-attn', value: 'on' },
      { key: 'models-max', value: '1' },
      { key: 'n-gpu-layers', value: 'auto' },
      { key: 'prio', value: '2' },
      { key: 'threads', value: '14' },
      { key: 'threads-batch', value: '24' },
      { key: 'ubatch-size', value: '4096' },
      { key: 'warmup', value: 'true' }
    ];
  }
  // single (default)
  return [
    { key: 'model', value: '/path/to/model.gguf' },
    { key: 'cont-batching', value: 'true' },
    { key: 'ctx-size', value: '65536' },
    { key: 'flash-attn', value: 'on' },
    { key: 'n-gpu-layers', value: 'auto' },
    { key: 'prio', value: '2' },
    { key: 'threads', value: '14' },
    { key: 'threads-batch', value: '24' },
    { key: 'ubatch-size', value: '2048' },
    { key: 'warmup', value: 'true' }
  ];
}

export function modelPresetEntries(modelPath: string): PresetEntry[] {
  return [
    { key: 'model', value: modelPath },
    { key: 'cont-batching', value: 'true' },
    { key: 'ctx-size', value: '65536' },
    { key: 'flash-attn', value: 'on' },
    { key: 'n-gpu-layers', value: 'auto' },
    { key: 'prio', value: '2' },
    { key: 'threads', value: '14' },
    { key: 'threads-batch', value: '24' },
    { key: 'ubatch-size', value: '2048' },
    { key: 'warmup', value: 'true' }
  ];
}
