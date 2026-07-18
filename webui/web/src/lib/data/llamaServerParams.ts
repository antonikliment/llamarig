// Catalog of llama-server CLI flags, used to drive autocomplete and inline
// hints in the preset editor. Keys mirror the INI key convention (dashes,
// no leading "--"); `aliases` covers short flags and synonyms accepted by
// llama-server so users can type either form and still get a match.
// Source: llama.cpp `common/arg.cpp` and `tools/server/README.md`
// (https://github.com/ggml-org/llama.cpp).
import type { LlamaServerParamInfo } from '../types';

export type LlamaParamType = 'bool' | 'number' | 'string' | 'enum' | 'path';

export interface LlamaServerParam {
  key: string;
  aliases?: string[];
  type: LlamaParamType;
  enumValues?: string[];
  default?: string;
  description: string;
}

/**
 * Converts a param parsed live from `llama-server --help` into the shape
 * used by the editor. The help text doesn't expose enum choices or precise
 * types, so this infers a best-effort `type` from the value hint/default;
 * the static catalog entry (if any) remains a richer fallback for the same
 * key via `mergeLlamaServerParams`.
 */
export function fromLlamaServerParamInfo(info: LlamaServerParamInfo): LlamaServerParam {
  const hint = info.value_hint?.toLowerCase() ?? '';
  let type: LlamaParamType = 'string';
  if (!hint) {
    type = 'bool';
  } else if (['n', 'num', 'int', 'float'].includes(hint)) {
    type = 'number';
  } else if (['fname', 'file', 'path'].includes(hint)) {
    type = 'path';
  }
  return {
    key: info.key,
    aliases: info.aliases,
    type,
    default: info.default_value || undefined,
    description: info.description
  };
}

export const llamaServerParams: LlamaServerParam[] = [
  // --- Model loading -------------------------------------------------
  { key: 'model', aliases: ['m'], type: 'path', description: 'Path to the GGUF model file to load.' },
  { key: 'model-url', aliases: ['mu'], type: 'string', description: 'URL to download the model from instead of a local path.' },
  { key: 'hf-repo', aliases: ['hf'], type: 'string', description: 'Hugging Face repo to pull the model from.' },
  { key: 'hf-file', type: 'string', description: 'Specific file name to fetch from the Hugging Face repo.' },
  { key: 'offline', type: 'bool', default: 'false', description: 'Skip network lookups and use only locally cached files.' },
  { key: 'alias', aliases: ['a'], type: 'string', description: 'Model name alias reported by the server (e.g. in /v1/models).' },
  { key: 'mmproj', aliases: ['mm'], type: 'path', description: 'Path to a multimodal projector file for vision/audio models.' },
  { key: 'mmproj-url', aliases: ['mmu'], type: 'string', description: 'URL to download the multimodal projector file from.' },
  { key: 'mmproj-auto', type: 'bool', default: 'true', description: 'Automatically load a multimodal projector if the model ships one.' },
  { key: 'mmproj-offload', type: 'bool', default: 'true', description: 'Offload the multimodal projector to the GPU.' },
  { key: 'lora', type: 'path', description: 'Path to a LoRA adapter to apply on top of the base model.' },
  { key: 'lora-scale', type: 'number', default: '1.0', description: 'Scaling factor applied to LoRA adapter weights.' },
  { key: 'control-vector', type: 'path', description: 'Path to a control vector file used to steer generation.' },
  { key: 'control-vector-layer-range', type: 'string', description: 'Layer range ("start-end") the control vector applies to.' },

  // --- Hardware / threads ---------------------------------------------
  { key: 'threads', aliases: ['t'], type: 'number', default: '-1', description: 'Number of CPU threads used during generation.' },
  { key: 'threads-batch', aliases: ['tb'], type: 'number', description: 'Number of CPU threads used during batch/prompt processing. Defaults to --threads.' },
  { key: 'cpu-mask', aliases: ['C'], type: 'string', description: 'CPU affinity mask (hex) for generation threads.' },
  { key: 'cpu-range', aliases: ['Cr'], type: 'string', description: 'Range of CPUs for affinity, e.g. "0-7".' },
  { key: 'cpu-strict', type: 'bool', default: 'false', description: 'Use strict CPU placement for generation threads.' },
  { key: 'cpu-mask-batch', aliases: ['Cb'], type: 'string', description: 'CPU affinity mask for batch/prompt-processing threads.' },
  { key: 'cpu-range-batch', aliases: ['Crb'], type: 'string', description: 'Range of CPUs for affinity during batch processing.' },
  { key: 'cpu-strict-batch', type: 'bool', description: 'Use strict CPU placement for batch threads. Defaults to --cpu-strict.' },
  { key: 'prio', type: 'number', default: '0', enumValues: ['0', '1', '2', '3'], description: 'Process/thread scheduling priority (0=normal .. 3=realtime).' },
  { key: 'prio-batch', type: 'number', default: '0', description: 'Scheduling priority for batch/prompt-processing threads.' },
  { key: 'poll', type: 'number', default: '50', description: 'Polling level (0-100) to use while waiting for work; higher uses more CPU for lower latency.' },
  { key: 'poll-batch', type: 'number', description: 'Polling level for batch threads. Defaults to --poll.' },
  { key: 'numa', type: 'enum', enumValues: ['distribute', 'isolate', 'numactl'], description: 'NUMA optimization strategy for multi-socket systems.' },

  // --- GPU offload ------------------------------------------------------
  { key: 'ngl', aliases: ['n-gpu-layers', 'gpu-layers'], type: 'number', description: 'Number of model layers to offload to the GPU. Use a high number (e.g. 999) to offload everything.' },
  { key: 'split-mode', aliases: ['sm'], type: 'enum', enumValues: ['none', 'layer', 'row'], default: 'layer', description: 'How to split the model across multiple GPUs.' },
  { key: 'tensor-split', aliases: ['ts'], type: 'string', description: 'Comma-separated fraction of the model to offload to each GPU.' },
  { key: 'main-gpu', aliases: ['mg'], type: 'number', default: '0', description: 'GPU index used for scratch/temporary buffers and small tensors.' },
  { key: 'device', type: 'string', description: 'Comma-separated list of devices to use for offloading (e.g. CUDA0,CUDA1).' },
  { key: 'flash-attn', aliases: ['fa'], type: 'enum', enumValues: ['auto', 'on', 'off'], default: 'auto', description: 'Enable Flash Attention kernels when supported by the backend.' },
  { key: 'cpu-moe', aliases: ['cmoe'], type: 'bool', default: 'false', description: 'Keep all Mixture-of-Experts weights on the CPU instead of the GPU.' },
  { key: 'n-cpu-moe', aliases: ['ncmoe'], type: 'number', description: 'Number of MoE layers to keep on the CPU (partial offload), from the last layer backwards.' },
  { key: 'override-tensor', aliases: ['ot'], type: 'string', description: 'Override the buffer type for matching tensors, e.g. "exps=CPU" to force experts onto CPU.' },
  { key: 'no-kv-offload', aliases: ['nkvo'], type: 'bool', default: 'false', description: 'Disable offloading the KV cache to the GPU.' },
  { key: 'no-host', type: 'bool', description: 'Bypass pinned host-memory buffers; can help with some backends but may be slower.' },

  // --- Context / cache ---------------------------------------------------
  { key: 'ctx-size', aliases: ['c'], type: 'number', default: '4096', description: 'Size of the prompt context window, in tokens. 0 = use the value from the model.' },
  { key: 'batch-size', aliases: ['b'], type: 'number', default: '2048', description: 'Logical maximum batch size for prompt processing.' },
  { key: 'ubatch-size', aliases: ['ub'], type: 'number', default: '512', description: 'Physical maximum micro-batch size; lower values reduce peak memory use.' },
  { key: 'keep', type: 'number', default: '0', description: 'Number of tokens from the initial prompt to keep when the context is shifted.' },
  { key: 'context-shift', type: 'bool', default: 'false', description: 'Automatically shift the context window on infinite text generation instead of stopping.' },
  { key: 'swa-full', type: 'bool', default: 'false', description: 'Use a full-size sliding-window-attention cache instead of the reduced SWA cache.' },
  { key: 'ctx-checkpoints', aliases: ['ctxcp', 'swa-checkpoints'], type: 'number', default: '0', description: 'Maximum number of context checkpoints kept per slot, used to speed up reprocessing.' },
  { key: 'cache-ram', aliases: ['cram'], type: 'number', default: '-1', description: 'Maximum RAM (MiB) used for the prompt cache; -1 = unlimited.' },
  { key: 'cache-idle-slots', type: 'bool', default: 'true', description: 'Save idle slot state to the prompt cache when a new task arrives.' },
  { key: 'kv-unified', aliases: ['kvu'], type: 'bool', description: 'Use a single unified KV cache buffer shared across slots instead of one per slot.' },
  { key: 'cache-type-k', aliases: ['ctk'], type: 'enum', enumValues: ['f32', 'f16', 'bf16', 'q8_0', 'q4_0', 'q4_1', 'iq4_nl', 'q5_0', 'q5_1'], default: 'f16', description: 'Data type used for the KV cache "K" tensor; quantized types trade quality for memory.' },
  { key: 'cache-type-v', aliases: ['ctv'], type: 'enum', enumValues: ['f32', 'f16', 'bf16', 'q8_0', 'q4_0', 'q4_1', 'iq4_nl', 'q5_0', 'q5_1'], default: 'f16', description: 'Data type used for the KV cache "V" tensor; quantized types trade quality for memory.' },
  { key: 'mlock', type: 'bool', default: 'false', description: 'Force the model to stay resident in RAM and never be swapped out.' },
  { key: 'mmap', type: 'bool', default: 'true', description: 'Memory-map the model file instead of reading it fully into RAM.' },
  { key: 'direct-io', aliases: ['dio'], type: 'bool', description: 'Use direct (unbuffered) I/O when reading the model file, if supported.' },

  // --- Server / networking ------------------------------------------------
  { key: 'host', type: 'string', default: '127.0.0.1', description: 'IP address llama-server listens on.' },
  { key: 'port', type: 'number', default: '8080', description: 'TCP port llama-server listens on.' },
  { key: 'api-key', type: 'string', description: 'Bearer API key required by clients to call the server.' },
  { key: 'api-key-file', type: 'path', description: 'File containing the API key, one per line, instead of passing it inline.' },
  { key: 'timeout', aliases: ['to'], type: 'number', default: '600', description: 'Server read/write timeout, in seconds.' },
  { key: 'parallel', aliases: ['np'], type: 'number', default: '1', description: 'Number of parallel decoding slots the server can serve at once.' },
  { key: 'cont-batching', aliases: ['cb'], type: 'bool', default: 'true', description: 'Enable continuous batching so new requests can join in-flight batches.' },
  { key: 'no-webui', aliases: ['ui', 'webui'], type: 'bool', default: 'false', description: 'Disable the built-in web UI served at "/".' },
  { key: 'metrics', type: 'bool', default: 'false', description: 'Expose a Prometheus-compatible /metrics endpoint.' },
  { key: 'slots', type: 'bool', default: 'true', description: 'Expose the /slots endpoint for inspecting active decoding slots.' },
  { key: 'props', type: 'bool', default: 'true', description: 'Expose the /props endpoint with server configuration metadata.' },
  { key: 'slot-save-path', type: 'path', description: 'Directory used to persist and restore slot/prompt-cache state across restarts.' },
  { key: 'threads-http', type: 'number', description: 'Number of threads used for the HTTP server itself, separate from inference threads.' },
  { key: 'no-slots', type: 'bool', description: 'Disable the /slots endpoint.' },

  // --- Embeddings / reranking ------------------------------------------------
  { key: 'embedding', aliases: ['embeddings'], type: 'bool', default: 'false', description: 'Restrict the server to embeddings-only mode (no text generation).' },
  { key: 'reranking', aliases: ['rerank'], type: 'bool', default: 'false', description: 'Enable the /rerank endpoint for cross-encoder style reranking.' },
  { key: 'pooling', type: 'enum', enumValues: ['none', 'mean', 'cls', 'last', 'rank'], description: 'Pooling strategy applied to token embeddings; defaults to the model\'s configured pooling.' },
  { key: 'attention', type: 'enum', enumValues: ['causal', 'non-causal'], description: 'Attention type to use when computing embeddings.' },

  // --- Chat / templating ------------------------------------------------
  { key: 'chat-template', type: 'string', description: 'Name of a built-in chat template, or a custom Jinja template string, to override the model default.' },
  { key: 'chat-template-file', type: 'path', description: 'File containing a Jinja chat template to use instead of the model default.' },
  { key: 'jinja', type: 'bool', description: 'Use the Jinja2 chat-template engine instead of the built-in minimal templater.' },
  { key: 'reasoning-format', type: 'enum', enumValues: ['none', 'deepseek', 'deepseek-legacy'], description: 'How to surface model "thinking"/reasoning content in API responses.' },
  { key: 'tools', type: 'string', description: 'Comma-separated list of built-in tools to enable for agentic tool-calling.' },

  // --- Sampling defaults -------------------------------------------------
  { key: 'seed', aliases: ['s'], type: 'number', default: '-1', description: 'RNG seed used for sampling; -1 picks a random seed each run.' },
  { key: 'temp', aliases: ['temperature'], type: 'number', default: '0.8', description: 'Sampling temperature; higher values increase randomness.' },
  { key: 'top-k', type: 'number', default: '40', description: 'Restrict sampling to the top K most likely tokens.' },
  { key: 'top-p', type: 'number', default: '0.9', description: 'Nucleus sampling: restrict to the smallest token set with cumulative probability >= top-p.' },
  { key: 'min-p', type: 'number', default: '0.05', description: 'Minimum probability (relative to the most likely token) for a token to be considered.' },
  { key: 'repeat-last-n', type: 'number', default: '64', description: 'Number of recent tokens considered for the repetition penalty.' },
  { key: 'repeat-penalty', type: 'number', default: '1.0', description: 'Penalty applied to tokens that already appeared in the recent context.' },
  { key: 'presence-penalty', type: 'number', default: '0.0', description: 'Flat penalty applied to any token that has already appeared at all.' },
  { key: 'frequency-penalty', type: 'number', default: '0.0', description: 'Penalty that scales with how often a token has already appeared.' },
  { key: 'dry-multiplier', type: 'number', default: '0.0', description: 'Strength of DRY (Don\'t Repeat Yourself) repetition sampling; 0 disables it.' },
  { key: 'dry-base', type: 'number', default: '1.75', description: 'Base value for DRY repetition penalty growth.' },
  { key: 'dry-allowed-length', type: 'number', default: '2', description: 'Longest repeated sequence allowed before DRY penalizes it.' },
  { key: 'dry-penalty-last-n', type: 'number', default: '-1', description: 'Number of tokens DRY looks back over; -1 uses the full context.' },
  { key: 'dry-sequence-breaker', type: 'string', description: 'Token/string that resets DRY\'s repetition tracking (repeatable).' },
  { key: 'mirostat', type: 'number', default: '0', enumValues: ['0', '1', '2'], description: 'Mirostat sampling mode: 0=disabled, 1=Mirostat, 2=Mirostat 2.0.' },
  { key: 'mirostat-lr', type: 'number', default: '0.1', description: 'Mirostat learning rate (eta).' },
  { key: 'mirostat-ent', type: 'number', default: '5.0', description: 'Mirostat target entropy (tau).' },
  { key: 'dynatemp-range', type: 'number', default: '0.0', description: 'Range for dynamic temperature sampling; 0 disables it.' },
  { key: 'dynatemp-exp', type: 'number', default: '1.0', description: 'Exponent used to scale dynamic temperature.' },
  { key: 'samplers', type: 'string', description: 'Ordered, semicolon-separated list of samplers to apply.' },
  { key: 'grammar', type: 'string', description: 'GBNF grammar string used to constrain generated output.' },
  { key: 'grammar-file', type: 'path', description: 'File containing a GBNF grammar used to constrain generated output.' },
  { key: 'json-schema', aliases: ['j'], type: 'string', description: 'JSON schema used to constrain generated output to valid JSON.' },
  { key: 'json-schema-file', aliases: ['jf'], type: 'path', description: 'File containing a JSON schema used to constrain generated output.' },
  { key: 'ignore-eos', type: 'bool', default: 'false', description: 'Ignore the end-of-sequence token and keep generating until n-predict/context limit.' },

  // --- RoPE / context extension -------------------------------------------
  { key: 'rope-scaling', type: 'enum', enumValues: ['none', 'linear', 'yarn'], description: 'RoPE frequency scaling method used to extend context length.' },
  { key: 'rope-scale', type: 'number', description: 'RoPE context scaling factor (inverse of rope-freq-scale).' },
  { key: 'rope-freq-base', type: 'number', description: 'RoPE base frequency; overrides the model default.' },
  { key: 'rope-freq-scale', type: 'number', description: 'RoPE frequency scaling factor; overrides the model default.' },
  { key: 'yarn-orig-ctx', type: 'number', default: '0', description: 'Original context size of the model, used by YaRN scaling. 0 = model default.' },
  { key: 'yarn-ext-factor', type: 'number', default: '-1.0', description: 'YaRN extrapolation mix factor; negative uses the YaRN default.' },
  { key: 'yarn-attn-factor', type: 'number', default: '1.0', description: 'Scaling factor applied to YaRN attention magnitude.' },
  { key: 'yarn-beta-slow', type: 'number', default: '1.0', description: 'YaRN high correction dimension/alpha.' },
  { key: 'yarn-beta-fast', type: 'number', default: '32.0', description: 'YaRN low correction dimension/beta.' },
  { key: 'grp-attn-n', aliases: ['gan'], type: 'number', default: '1', description: 'Group-attention factor used for self-extend context length.' },
  { key: 'grp-attn-w', aliases: ['gaw'], type: 'number', default: '512', description: 'Group-attention width used for self-extend context length.' },

  // --- Speculative decoding ------------------------------------------------
  { key: 'model-draft', aliases: ['md'], type: 'path', description: 'Path to a smaller draft model used for speculative decoding.' },
  { key: 'ctx-size-draft', type: 'number', description: 'Context size for the draft model; defaults to the main model\'s context size.' },
  { key: 'gpu-layers-draft', aliases: ['ngld'], type: 'number', description: 'Number of draft-model layers to offload to the GPU.' },
  { key: 'device-draft', type: 'string', description: 'Device(s) used to run the draft model.' },
  { key: 'draft-max', aliases: ['draft', 'draft-n'], type: 'number', default: '16', description: 'Maximum number of tokens the draft model speculates per round.' },
  { key: 'draft-min', aliases: ['draft-n-min'], type: 'number', default: '5', description: 'Minimum number of draft tokens required before they are validated.' },
  { key: 'draft-p-min', type: 'number', default: '0.75', description: 'Minimum probability a draft token must have to be kept.' },

  // --- Misc / process behavior --------------------------------------------
  { key: 'warmup', type: 'bool', default: 'true', description: 'Run a warmup inference pass on startup before accepting requests.' },
  { key: 'no-perf', type: 'bool', description: 'Disable internal libllama performance timing output.' },
  { key: 'verbose', aliases: ['v'], type: 'bool', default: 'false', description: 'Enable verbose (debug-level) logging.' },
  { key: 'log-disable', type: 'bool', description: 'Disable logging entirely.' },
  { key: 'log-file', type: 'path', description: 'File to write server logs to, in addition to stdout.' },
  { key: 'log-colors', type: 'bool', description: 'Colorize log output.' },
  { key: 'log-format', type: 'enum', enumValues: ['text', 'json'], default: 'text', description: 'Output format for server log lines.' }
];

function buildAliasIndex(params: LlamaServerParam[]): Map<string, LlamaServerParam> {
  const index = new Map<string, LlamaServerParam>();
  for (const param of params) {
    index.set(param.key.toLowerCase(), param);
    for (const alias of param.aliases ?? []) index.set(alias.toLowerCase(), param);
  }
  return index;
}

const aliasIndex = buildAliasIndex(llamaServerParams);

/** Looks up a llama-server param by its INI key, ignoring leading dashes and case. */
export function findLlamaServerParam(rawKey: string): LlamaServerParam | undefined {
  const key = rawKey.trim().replace(/^-+/, '').toLowerCase();
  if (!key) return undefined;
  return aliasIndex.get(key);
}

/**
 * Merges params parsed live from the llama-server binary with the static
 * catalog, preferring the live entry for any key the binary reports (since
 * it reflects the actually-installed version) and falling back to the
 * static catalog for everything else.
 */
export function mergeLlamaServerParams(dynamicInfo: LlamaServerParamInfo[]): LlamaServerParam[] {
  if (!dynamicInfo.length) return llamaServerParams;
  const dynamic = dynamicInfo.map(fromLlamaServerParamInfo);
  const dynamicKeysAndAliases = new Set<string>();
  for (const param of dynamic) {
    dynamicKeysAndAliases.add(param.key.toLowerCase());
    for (const alias of param.aliases ?? []) dynamicKeysAndAliases.add(alias.toLowerCase());
  }
  return [
    ...dynamic,
    ...llamaServerParams.filter(
      (param) =>
        !dynamicKeysAndAliases.has(param.key.toLowerCase()) &&
        !param.aliases?.some((alias) => dynamicKeysAndAliases.has(alias.toLowerCase()))
    )
  ];
}

/** Builds a lookup function over an arbitrary (e.g. merged) param list. */
export function createLlamaServerParamLookup(params: LlamaServerParam[]) {
  const index = buildAliasIndex(params);
  return (rawKey: string): LlamaServerParam | undefined => {
    const key = rawKey.trim().replace(/^-+/, '').toLowerCase();
    if (!key) return undefined;
    return index.get(key);
  };
}
