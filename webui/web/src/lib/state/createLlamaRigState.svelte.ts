import type {
  CatalogCacheState,
  CatalogModel,
  MachineProfile,
  LocalModel,
  LlamaServerParamInfo,
  LogArchive,
  ModelDownload,
  ModelApplyPreview,
  ModelPreset,
  OperationResult,
  ModelResolution,
  PresetEntry,
  ServerEvent,
  SignalsSnapshot
} from '../types';
import type { RuntimeHistorySample } from '../runtimeHistory';

export function createLlamaRigState() {
  const state = $state({
    apiBase: '',
    token: '',
    busy: false,
    activeSection: 'runtime',
    presets: [] as ModelPreset[],
    modelsMax: 0,
    selectedPresetName: '',
    activePresetNames: [] as string[],
	defaultPreset: '',
    currentPreset: null as ModelPreset | null,
    modelUrl: '',
    modelResolution: null as ModelResolution | null,
    catalogQuery: {
      limit: 50,
      sort: 'trending',
      search: '',
      min_fit: 'fits'
    },
    catalogModels: [] as CatalogModel[],
    catalogMachine: null as MachineProfile | null,
    catalogCache: null as CatalogCacheState | null,
    catalogErrors: [] as string[],
    signals: null as SignalsSnapshot | null,
    signalsLastError: '',
    runtimeHistory: [] as RuntimeHistorySample[],
    selectedCatalogModelId: '',
    catalogLoading: false,
    localModels: [] as LocalModel[],
    localModelsLoading: false,
    selectedModelFile: '',
    activeModelDownloadId: '',
    modelApplyPreview: null as ModelApplyPreview | null,
    downloads: {} as Record<string, ModelDownload>,
    draftEntries: [] as PresetEntry[],
    llamaServerParams: [] as LlamaServerParamInfo[],
    originals: {
      entries: [] as PresetEntry[]
    },
    dirty: {
      entries: false
    },
    logEntries: [] as string[],
    controlLogText: '',
    gatewayLogText: '',
    logSource: 'control' as 'control' | 'gateway',
    logLines: 500,
    logPaused: false,
    logArchives: [] as LogArchive[],
    selectedLogArchiveId: '',
    logArchiveText: '',
    serverEvents: [] as ServerEvent[],
    lastOperation: null as OperationResult | null,
    runtimeStatus: {
      status: 'unknown',
      detail: '-',
      checkedAt: ''
    }
  });
  return state;
}

export type LlamaRigState = ReturnType<typeof createLlamaRigState>;
