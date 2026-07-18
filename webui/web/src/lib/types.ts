export type RuntimeInfo = {
  status?: string;
  detail?: string;
  checked_at?: string;
};

export type InfoResponse = {
  router?: RuntimeInfo;
  default_preset?: string;
};

export type RuntimePreset = {
  name?: string;
  state?: string;
  ready?: boolean;
};

export type RuntimeStatusResponse = {
  state?: string;
  detail?: string;
  checked_at?: string;
  presets?: RuntimePreset[];
};

export type OperationResult = {
  target?: string;
  action?: string;
  status?: string;
  message?: string;
  duration_ms?: number;
};

export type OperationResponse = {
  result?: OperationResult;
};

export type ModelFile = {
  filename: string;
  size_bytes?: number;
  quant?: string;
  exists?: boolean;
  estimated_ram_bytes?: number;
  estimated_vram_bytes?: number;
  fit_level?: string;
  fit_reason?: string;
};

export type LocalModel = {
  path: string;
  filename: string;
  size_bytes?: number;
  modified_at?: string;
	used_by_presets?: string[];
  model_path_presets?: string[];
  models_dir_presets?: string[];
};

export type LocalModelsResponse = {
  ok?: boolean;
  models?: LocalModel[];
};

export type ModelResolution = {
  source: {
    url: string;
    owner: string;
    repo: string;
  };
  llama_cpp?: {
    compatible?: boolean;
    hf_ref?: string;
  };
  description?: string;
  params?: number;
  architecture?: string;
  context_length?: number;
  is_moe?: boolean;
  files?: ModelFile[];
};

export type CatalogQuery = {
  limit?: number;
  sort?: string;
  search?: string;
  min_fit?: string;
};

export type MachineProfile = {
  total_ram_bytes?: number;
  available_ram_bytes?: number;
  gpu_name?: string;
  vram_bytes?: number;
  has_gpu?: boolean;
};

export type MachineFit = {
  level: string;
  reason?: string;
  required_ram_bytes?: number;
  available_ram_bytes?: number;
};

export type CatalogModel = {
  id: string;
  owner: string;
  repo: string;
  url: string;
  downloads?: number;
  likes?: number;
  last_modified?: string;
  tags?: string[];
  license?: string;
  params?: number;
  architecture?: string;
  context_length?: number;
  is_moe?: boolean;
  files?: ModelFile[];
  best_file?: ModelFile;
  fit?: MachineFit;
  score?: number;
};

export type CatalogCacheState = {
  hit?: boolean;
  stale?: boolean;
  refreshing?: boolean;
  updated_at?: string;
  ttl_seconds?: number;
};

export type ModelCatalogResponse = {
  ok?: boolean;
  machine?: MachineProfile;
  models?: CatalogModel[];
  cache?: CatalogCacheState;
  errors?: string[];
};

export type MemoryStats = {
  total_bytes?: number;
  available_bytes?: number;
  used_bytes?: number;
  used_percent?: number;
};

export type CPUStats = {
  logical_cores?: number;
  used_percent?: number;
};

export type GPUStats = {
  name?: string;
  backend?: string;
  total_vram_bytes?: number;
  used_vram_bytes?: number;
  utilization_percent?: number;
  temperature_celsius?: number;
  source?: string;
};

export type RuntimeProcessStats = {
  name?: string;
  pid?: number;
  rss_bytes?: number;
  cpu_percent?: number;
  command?: string;
};

export type SignalsSnapshot = {
  captured_at?: string;
  memory?: MemoryStats;
  cpu?: CPUStats;
  gpu?: GPUStats[];
  runtime?: RuntimeProcessStats[];
  warnings?: string[];
};

export type SignalsResponse = {
  signals?: SignalsSnapshot;
};

export type ServerEvent = {
  time: string;
  action: string;
  success: boolean;
  error_kind?: string;
  duration?: string;
};

export type EventsResponse = {
  events?: ServerEvent[];
};

export type LogsResponse = {
  ok: boolean;
  source?: 'control' | 'gateway';
  text: string;
};

export type LogArchive = {
  id: string;
  source: 'control' | 'gateway';
  size_bytes: number;
  archived_at: string;
};

export type LogArchivesResponse = {
  ok: boolean;
  archives?: LogArchive[];
};

export type ModelDownload = {
  id: string;
  state: string;
  filename: string;
  target_path?: string;
  received_bytes?: number;
  total_bytes?: number;
  percent?: number;
  error?: string;
};

export type ModelDownloadResponse = {
  download?: ModelDownload;
};

export type ModelApplyPreview = {
  original?: string;
  updated?: string;
};

export type PresetEntry = {
  key: string;
  value: string;
};

export type ModelPreset = {
  name: string;
  entries: PresetEntry[];
  source_status?: string;
  source_error?: string;
  autostart?: boolean;
};

export type PresetsResponse = {
  ok?: boolean;
  path?: string;
  global?: PresetEntry[];
  presets?: ModelPreset[];
  models_max?: number;
};

export type PresetResponse = {
  ok?: boolean;
  preset?: ModelPreset;
};

export type LlamaServerParamInfo = {
  key: string;
  aliases?: string[];
  value_hint?: string;
  default_value?: string;
  description: string;
};

export type LlamaServerParamsResponse = {
  ok?: boolean;
  params?: LlamaServerParamInfo[];
  source?: string;
  warning?: string;
};

export type ApiErrorPayload = {
  error?: {
    kind?: string;
    message?: string;
  };
};
