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
  ModelPreset,
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
    getInfo: () => request<InfoResponse>('/api/info'),
    getRuntimeStatus: () => request<RuntimeStatusResponse>('/api/runtime/status'),
    getLlamaServerParams: () => request<LlamaServerParamsResponse>('/api/runtime/llama-params'),
    getSignals: () => request<SignalsResponse>('/api/signals'),
    getEvents: () => request<EventsResponse>('/api/events'),
    getLogs: (source = 'control', lines = 500) => request<LogsResponse>(`/api/logs?source=${encodeURIComponent(source)}&lines=${lines}`),
    listLogArchives: () => request<LogArchivesResponse>('/api/logs/archives'),
    getLogArchive: (id: string, lines = 500) => request<LogsResponse>(`/api/logs/archives/${encodeURIComponent(id)}?lines=${lines}`),
    deleteLogArchive: (id: string) => request<{ ok: boolean; deleted: number }>(`/api/logs/archives/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    clearLogArchives: () => request<{ ok: boolean; deleted: number }>('/api/logs/archives', { method: 'DELETE' }),
    startRuntime: (name: string) =>
      request<OperationResponse>(`/api/runtime/start?preset=${encodeURIComponent(name)}`, { method: 'POST' }),
    stopRuntime: (name = '') =>
      request<OperationResponse>(`/api/runtime/stop${name ? `?preset=${encodeURIComponent(name)}` : ''}`, { method: 'POST' }),
    restartRuntime: (name: string) =>
      request<OperationResponse>(`/api/runtime/restart?preset=${encodeURIComponent(name)}`, { method: 'POST' }),
    listPresets: () => request<PresetsResponse>('/api/presets'),
    getPreset: (name: string) => request<PresetResponse>(`/api/presets/${encodeURIComponent(name)}`),
    createPreset: (body: { name: string; entries: PresetEntry[] }) =>
      request<PresetResponse>('/api/presets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      }),
    replacePreset: (name: string, body: { entries: PresetEntry[] }) =>
      request<PresetResponse>(`/api/presets/${encodeURIComponent(name)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      }),
    deletePreset: (name: string) => request<{ ok: boolean }>(`/api/presets/${encodeURIComponent(name)}`, { method: 'DELETE' }),
    cleanupPreset: (name: string) => request<{ ok: boolean }>(`/api/presets/${encodeURIComponent(name)}/cleanup`, { method: 'POST' }),
    setPresetAutostart: (name: string, enabled: boolean) =>
      request<{ ok: boolean }>(`/api/presets/${encodeURIComponent(name)}/autostart`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled })
      }),
    listLocalModels: () => request<LocalModelsResponse>('/api/models/local'),
    deleteLocalModel: (path: string, cascadePresets = false) =>
      request(`/api/models/local?path=${encodeURIComponent(path)}&cascade_presets=${cascadePresets}`, { method: 'DELETE' }),
    listModelCatalog: (params: CatalogQuery = {}) => {
      const query = new URLSearchParams();
      if (params.limit) query.set('limit', String(params.limit));
      if (params.sort) query.set('sort', params.sort);
      if (params.search) query.set('search', params.search);
      if (params.min_fit) query.set('min_fit', params.min_fit);
      const suffix = query.toString();
      return request<ModelCatalogResponse>(`/api/models/catalog${suffix ? `?${suffix}` : ''}`);
    },
    modelCatalogEventsUrl: () => apiUrl('/api/models/catalog/events', getSession().apiBase),
    resolveModel: (url: string) =>
      request<ModelResolution>('/api/models/resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url })
      }),
    startModelDownload: (body: { url: string; filename: string }) =>
      request<ModelDownloadResponse>('/api/models/downloads', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      }),
    getModelDownload: (id: string) => request<ModelDownloadResponse>(`/api/models/downloads/${encodeURIComponent(id)}`),
    cancelModelDownload: (id: string) =>
      request<ModelDownloadResponse>(`/api/models/downloads/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    applyModelToPreset: (id: string, body: { preset: string; preview?: boolean }) =>
      request<OperationResponse & { command?: unknown; preview?: unknown }>(`/api/models/downloads/${encodeURIComponent(id)}/apply-to-preset`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      })
  };
}

function apiError(response: Response, data: unknown) {
  const payload = data as { error?: { kind?: string; message?: string } } | null;
  const message = payload?.error?.message || `HTTP ${response.status}`;
  const kind = payload?.error?.kind || 'http_error';
  return new Error(`${kind}: ${message}`);
}
