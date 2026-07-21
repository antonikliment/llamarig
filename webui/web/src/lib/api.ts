import { createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { ControlService } from './gen/v1/control_pb';
import type {
  CatalogQuery,
  EventsResponse,
  InfoResponse,
  ModelCatalogResponse,
  LocalModelsResponse,
  LlamaServerParamsResponse,
  LogsResponse,
  LogArchivesResponse,
  ModelDownloadResponse,
  ModelResolution,
  OperationResponse,
  PresetEntry,
  PresetResponse,
  PresetsResponse,
  SignalsResponse,
  RuntimeStatusResponse
} from './types';
import type { SessionState } from './session';

export type LlamaRigApi = ReturnType<typeof createApiClient>;

export function apiUrl(path: string, apiBase: string, locationProtocol = window.location.protocol) {
  const base = apiBase.trim().replace(/\/$/, '');
  if (!base) return path;
  const normalized = /^https?:\/\//i.test(base) ? base : `${locationProtocol}//${base}`;
  return new URL(path, normalized).toString();
}

export function createApiClient(getSession: () => SessionState, fetcher: typeof fetch = fetch) {
  const auth: Interceptor = (next) => (request) => {
    const token = getSession().token.trim();
    if (token) request.header.set('Authorization', `Bearer ${token}`);
    return next(request);
  };
  const control = () => createClient(ControlService, createConnectTransport({
    baseUrl: apiUrl('/', getSession().apiBase).replace(/\/$/, '') || '/',
    fetch: fetcher,
    interceptors: [auth]
  }));

  async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers || {});
    const session = getSession();
    if (session.token.trim()) headers.set('Authorization', `Bearer ${session.token.trim()}`);
    const response = await fetcher(apiUrl(path, session.apiBase), { ...options, headers });
    const text = await response.text();
    let data: unknown = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = { ok: false, error: { message: text || 'Invalid JSON response' } };
    }
    if (!response.ok) throw apiError(response, data);
    return data as T;
  }

  return {
    getInfo: async () => {
      const response = await control().getInfo({});
      return legacy<InfoResponse>({ ...response.info, build: response.build });
    },
    getRuntimeStatus: async () => legacy<RuntimeStatusResponse>((await control().getRuntimeStatus({})).status),
    getLlamaServerParams: async () => legacy<LlamaServerParamsResponse>(await control().getLlamaServerParams({})),
    getSignals: async () => legacy<SignalsResponse>(await control().getSignals({})),
    getEvents: async () => legacy<EventsResponse>(await control().listEvents({})),
    getLogs: (source = 'control', lines = 500) => request<LogsResponse>(`/api/logs?source=${encodeURIComponent(source)}&lines=${lines}`),
    listLogArchives: () => request<LogArchivesResponse>('/api/logs/archives'),
    getLogArchive: (id: string, lines = 500) => request<LogsResponse>(`/api/logs/archives/${encodeURIComponent(id)}?lines=${lines}`),
    deleteLogArchive: (id: string) => request<{ ok: boolean; deleted: number }>(`/api/logs/archives/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    clearLogArchives: () => request<{ ok: boolean; deleted: number }>('/api/logs/archives', { method: 'DELETE' }),
    startRuntime: async (target: string) => legacy<OperationResponse>(await control().startRuntime({ target })),
    stopRuntime: async (target = '') => legacy<OperationResponse>(await control().stopRuntime({ target })),
    restartRuntime: async (target: string) => legacy<OperationResponse>(await control().restartRuntime({ target })),
    listPresets: async () => legacy<PresetsResponse>(await control().listPresets({})),
    getPreset: async (name: string) => legacy<PresetResponse>(await control().getPreset({ name })),
    createPreset: async (body: { name: string; entries: PresetEntry[] }) =>
      legacy<PresetResponse>(await control().putPreset({ preset: body, createOnly: true })),
    replacePreset: async (name: string, body: { entries: PresetEntry[] }) =>
      legacy<PresetResponse>(await control().putPreset({ preset: { name, entries: body.entries } })),
    deletePreset: async (name: string) => legacy<{ ok: boolean }>(await control().deletePreset({ name })),
    cleanupPreset: async (name: string) => legacy<{ ok: boolean }>(await control().cleanupPreset({ name })),
    setPresetAutostart: async (name: string, enabled: boolean) =>
      legacy<{ ok: boolean }>(await control().setPresetAutostart({ name, enabled })),
    listLocalModels: async () => legacy<LocalModelsResponse>(await control().listLocalModels({})),
    deleteLocalModel: async (path: string, cascadePresets = false) =>
      legacy(await control().deleteLocalModel({ path, cascadePresets })),
    listModelCatalog: async (params: CatalogQuery = {}) => legacy<ModelCatalogResponse>(await control().listModelCatalog({
      limit: params.limit,
      sort: params.sort,
      search: params.search,
      minFit: params.min_fit
    })),
    watchModelCatalog: (signal: AbortSignal) => control().watchModelCatalog({}, { signal }),
    resolveModel: async (url: string) => legacy<ModelResolution>((await control().resolveModel({ url })).resolution),
    startModelDownload: async (body: { url: string; filename: string }) =>
      legacy<ModelDownloadResponse>(await control().startModelDownload(body)),
    getModelDownload: async (id: string) => legacy<ModelDownloadResponse>(await control().getModelDownload({ id })),
    cancelModelDownload: async (id: string) => legacy<ModelDownloadResponse>(await control().cancelModelDownload({ id })),
    applyModelToPreset: async (id: string, body: { preset: string; preview?: boolean }) => {
      const response = await control().applyModelDownloadToPreset({ id, preset: body.preset, preview: body.preview });
      return legacy<OperationResponse & { preview?: unknown }>({
        ...response,
        preview: response.previewDiff
      });
    }
  };
}

function legacy<T>(value: unknown): T {
  if (typeof value === 'bigint') return Number(value) as T;
  if (Array.isArray(value)) return value.map(legacy) as T;
  if (value && typeof value === 'object') {
    return Object.entries(value).reduce<Record<string, unknown>>((result, [key, child]) => {
      if (!key.startsWith('$')) result[key.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`)] = legacy(child);
      return result;
    }, {}) as T;
  }
  return value as T;
}

function apiError(response: Response, data: unknown) {
  const payload = data as { error?: { kind?: string; message?: string } } | null;
  const message = payload?.error?.message || `HTTP ${response.status}`;
  const kind = payload?.error?.kind || 'http_error';
  return new Error(`${kind}: ${message}`);
}
