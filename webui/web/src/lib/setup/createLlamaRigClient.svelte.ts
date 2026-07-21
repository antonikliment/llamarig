import { createApiClient } from '../api';
import { loadSession, saveApiBase, saveToken } from '../session';
import { canApplyDownload, choosePresetSelection, isTerminalDownloadState } from '../tasks';
import type { CatalogModel, LocalModel, ModelDownload, ModelFile, ModelPreset, PresetEntry, SignalsSnapshot } from '../types';
import { createLlamaRigState } from '../state/createLlamaRigState.svelte';
import { isPresetActive as selectIsPresetActive } from '../state/selectors';
import { templateEntries } from '../presetTemplates';
import { appendRuntimeSample } from '../runtimeHistory';
import { toast } from 'svelte-sonner';

export function createLlamaRigClient() {
  const state = createLlamaRigState();
  const api = createApiClient(() => ({ apiBase: state.apiBase, token: state.token }));
  let errorMessage = $state('');
  let pollTimer: number | null = null;
  let refreshTimer: number | null = null;
  let presetCheckingTimer: number | null = null;
  let catalogEvents: AbortController | null = null;

  const client = {
    state,
    api,
    sections: [
      { id: 'runtime', label: 'Dashboard' },
      { id: 'presets', label: 'Presets' },
      { id: 'models', label: 'Models' },
      { id: 'logs', label: 'Logs' }
    ],
    get errorMessage() {
      return errorMessage;
    },
    set errorMessage(value: string) {
      errorMessage = value;
    },
    mount,
    destroy,
    saveApiBase: () => saveApiBase(sessionStorage, state.apiBase),
    saveToken: () => saveToken(sessionStorage, state.token),
    hasDirtyEditors,
    beforeUnload,
    log,
    showError,
    runTask,
    testConnection,
    refreshRuntimeStatus,
    refreshSignals,
    refreshEvents,
    refreshLogs,
    resumeLogs,
    loadLogArchives,
    selectLogArchive,
    deleteLogArchive,
    clearLogArchives,
    loadLlamaServerParams,
    startSelectedPreset,
    startPreset,
    restartSelectedPreset,
    restartPreset,
    stopRuntime,
    stopPreset,
    loadPresets,
    selectPreset,
    populatePreset,
    createPreset,
    createLocalPreset,
    duplicatePreset,
    deletePreset,
    cleanupPreset,
    toggleAutostart,
    deleteLocalModel,
    reloadSelectedPreset,
    savePreset,
    afterPresetMutation,
    resolveModel,
    loadModelCatalog,
    loadLocalModels,
    refreshResourcesAndCatalog,
    connectCatalogEvents,
    closeCatalogEvents,
    useCatalogFile,
    startDownload,
    cancelDownload,
    previewApplyToPreset,
    applyToPreset,
    startPolling,
    stopPolling,
    pollDownload,
    requireSelectedPreset,
    requireCurrentPreset,
    requireDownload,
    runtimeHint,
    isPresetActive,
    activeDownload,
    canApplyDownload,
    clearLogs: () => {
      state.logEntries = [];
    }
  };

  async function mount() {
    const session = loadSession(sessionStorage);
    state.apiBase = session.apiBase;
    state.token = session.token;
    window.addEventListener('beforeunload', beforeUnload);
    await runTask('initial load', async () => {
      await refreshServerInfo();
      await refreshRuntimeStatus();
      await refreshSignals();
      await loadPresets({ force: true });
      await loadLocalModels();
      await loadModelCatalog();
      await refreshEvents();
      await loadLlamaServerParams();
    });
    connectCatalogEvents();
    refreshTimer = window.setInterval(() => {
      refreshRuntimeStatus().catch(showError);
      refreshSignals().catch(() => undefined);
      refreshEvents().catch(() => undefined);
      if (state.activeSection === 'logs' && !state.logPaused) refreshLogs().catch(() => undefined);
    }, 5000);
  }

  function destroy() {
    window.removeEventListener('beforeunload', beforeUnload);
    if (refreshTimer) window.clearInterval(refreshTimer);
    if (presetCheckingTimer) window.clearTimeout(presetCheckingTimer);
    closeCatalogEvents();
    stopPolling();
  }

  function beforeUnload(event: BeforeUnloadEvent) {
    if (!hasDirtyEditors()) return;
    event.preventDefault();
    event.returnValue = '';
  }

  function hasDirtyEditors() {
    return state.dirty.entries;
  }

  function log(message: string) {
    state.logEntries = [`${new Date().toLocaleTimeString()} ${message}`, ...state.logEntries].slice(0, 200);
  }

  function showError(err: unknown) {
    errorMessage = err instanceof Error ? err.message : String(err);
  }

  async function runTask(label: string, task: () => Promise<void>) {
    if (state.busy) return;
    state.busy = true;
    errorMessage = '';
    try {
      await task();
    } catch (err) {
      showError(err);
      log(`${label} failed: ${err instanceof Error ? err.message : err}`);
      toast.error(`${label} failed`, { description: err instanceof Error ? err.message : String(err) });
    } finally {
      state.busy = false;
    }
  }

  async function testConnection() {
    await runTask('test connection', async () => {
      const info = await refreshServerInfo();
		log(`Connection ok: ${info.router?.status || 'unknown'}`);
      await refreshRuntimeStatus();
    });
  }

  async function refreshServerInfo() {
    const info = await api.getInfo();
	state.defaultPreset = info.default_preset || '';
    return info;
  }

  async function refreshRuntimeStatus() {
    const status = await api.getRuntimeStatus();
	const active = (status.presets || [])
		.filter((preset) => preset.name && preset.state !== 'stopped' && preset.state !== 'failed')
		.map((preset) => preset.name as string);
    state.activePresetNames = active;
    state.runtimeStatus.status = status.state || 'stopped';
    state.runtimeStatus.detail = status.detail || '-';
    state.runtimeStatus.checkedAt = status.checked_at || '';
    errorMessage = '';
  }

  async function refreshSignals() {
    try {
      const data = await api.getSignals();
      state.signals = data.signals || null;
      appendRuntimeHistory(state.signals);
      state.signalsLastError = '';
    } catch (err) {
      state.signalsLastError = err instanceof Error ? err.message : String(err);
    }
  }

  async function loadLlamaServerParams() {
    try {
      const data = await api.getLlamaServerParams();
      state.llamaServerParams = data.ok ? data.params || [] : [];
    } catch {
      state.llamaServerParams = [];
    }
  }

  function appendRuntimeHistory(signals: SignalsSnapshot | null) {
    state.runtimeHistory = appendRuntimeSample(state.runtimeHistory, signals);
  }

  async function refreshEvents() {
    const data = await api.getEvents();
    state.serverEvents = data.events || [];
  }

  const activeLogRequests = new Set<string>();
  async function refreshLogs() {
    const source = state.logSource;
    if (activeLogRequests.has(source)) return;
    activeLogRequests.add(source);
    try {
      const data = await api.getLogs(source, state.logLines);
      if (source === 'control') state.controlLogText = data.text || '';
      else state.gatewayLogText = data.text || '';
    } finally {
      activeLogRequests.delete(source);
    }
  }

  async function resumeLogs() {
    state.logPaused = false;
    try {
      await refreshLogs();
    } catch (err) {
      showError(err);
    }
  }

  async function loadLogArchives() {
    const data = await api.listLogArchives();
    state.logArchives = data.archives || [];
  }

  async function selectLogArchive(id: string) {
    try {
      const data = await api.getLogArchive(id, state.logLines);
      state.selectedLogArchiveId = id;
      state.logArchiveText = data.text || '';
    } catch (err) {
      showError(err);
    }
  }

  async function deleteLogArchive(id: string) {
    await runTask('delete log archive', async () => {
      await api.deleteLogArchive(id);
      if (state.selectedLogArchiveId === id) {
        state.selectedLogArchiveId = '';
        state.logArchiveText = '';
      }
      await loadLogArchives();
    });
  }

  async function clearLogArchives() {
    await runTask('clear log archives', async () => {
      await api.clearLogArchives();
      state.selectedLogArchiveId = '';
      state.logArchiveText = '';
      await loadLogArchives();
    });
  }

  async function startSelectedPreset() {
    await startPreset(requireSelectedPreset());
  }

  async function startPreset(name: string) {
    await runTask('start preset', async () => {
      const result = await api.startRuntime(name);
      state.lastOperation = result.result || null;
      log(`Started ${name}.`);
      toast.success(`Started ${name}`);
      await refreshRuntimeStatus();
      await loadPresets({ select: name, force: true });
    });
  }

  async function restartSelectedPreset() {
    await restartPreset(requireSelectedPreset());
  }

  async function restartPreset(name: string) {
    await runTask('restart preset', async () => {
      const result = await api.restartRuntime(name);
      state.lastOperation = result.result || null;
      log(`Restarted ${name}.`);
      toast.success(`Restarted ${name}`);
      await refreshRuntimeStatus();
      await loadPresets({ select: name, force: true });
    });
  }

  async function stopRuntime() {
    await runTask('stop runtime', async () => {
      const result = await api.stopRuntime();
      state.lastOperation = result.result || null;
      log('Stopped active presets.');
      toast.success('Stopped active presets');
      await refreshRuntimeStatus();
      await loadPresets({ force: true });
    });
  }

  async function stopPreset(name: string) {
    await runTask('stop preset', async () => {
      const result = await api.stopRuntime(name);
      state.lastOperation = result.result || null;
      log(`Stopped ${name}.`);
      toast.success(`Stopped ${name}`);
      await refreshRuntimeStatus();
      await loadPresets({ select: name, force: true });
    });
  }

  async function loadPresets(options: { select?: string; force?: boolean; preserveDirty?: boolean } = {}) {
    const task = async () => {
      const data = await api.listPresets();
      state.presets = data.presets || [];
      state.modelsMax = data.models_max ?? 0;
      const target = choosePresetSelection(
        state.presets,
        options.select || '',
        state.activePresetNames,
		state.defaultPreset,
        state.selectedPresetName
      );
      const preserveCurrent = options.preserveDirty && state.dirty.entries && target === state.selectedPresetName;
      if (target && preserveCurrent && state.currentPreset) {
        const fresh = state.presets.find((preset) => preset.name === target);
        if (fresh) state.currentPreset = { ...state.currentPreset, source_status: fresh.source_status, source_error: fresh.source_error };
      } else if (target) {
        await selectPreset(target, { force: true, skipDirtyCheck: options.force });
      }
      if (presetCheckingTimer) window.clearTimeout(presetCheckingTimer);
      presetCheckingTimer = null;
      if (state.presets.some((preset) => preset.source_status === 'checking')) {
        presetCheckingTimer = window.setTimeout(() => {
          presetCheckingTimer = null;
          void loadPresets({ force: true, preserveDirty: true });
        }, 1000);
      }
      log('Presets loaded.');
    };
    if (options.force || state.busy) return task();
    return runTask('load presets', task);
  }

  async function selectPreset(name: string, options: { force?: boolean; skipDirtyCheck?: boolean } = {}) {
    const load = async () => {
      const data = await api.getPreset(name);
      populatePreset(data.preset || null);
      state.selectedPresetName = name;
      state.modelApplyPreview = null;
    };
    if (options.force || state.busy) return load();
    return runTask('select preset', load);
  }

  function populatePreset(preset: ModelPreset | null) {
    state.currentPreset = preset;
    state.originals.entries = preset?.entries ? [...preset.entries] : [];
    state.draftEntries = preset?.entries ? preset.entries.map((e) => ({ ...e })) : [];
    state.dirty.entries = false;
  }

  async function createPreset(presetName: string, presetTemplate: string) {
    await runTask('create preset', async () => {
      const name = presetName.trim();
      if (!name) throw new Error('preset name is required');
      await api.createPreset({ name, entries: templateEntries(presetTemplate) });
      log(`Created preset ${name}.`);
      toast.success(`Created ${name}`);
      await loadPresets({ select: name, force: true });
    });
  }

  async function createLocalPreset(presetName: string, entries: PresetEntry[]) {
    await runTask('create model preset', async () => {
      const name = presetName.trim();
      if (!name) throw new Error('preset name is required');
      await api.createPreset({ name, entries });
      log(`Created preset ${name}.`);
      toast.success(`Created ${name}`);
      await loadPresets({ select: name, force: true });
      await loadLocalModels();
    });
  }

  async function duplicatePreset(presetName: string) {
    await runTask('duplicate preset', async () => {
      const current = requireCurrentPreset();
      const name = presetName.trim();
      if (!name) throw new Error('preset name is required');
      await api.createPreset({ name, entries: current.entries?.map((e) => ({ ...e })) || [] });
      log(`Duplicated ${current.name} to ${name}.`);
      toast.success(`Duplicated ${current.name}`);
      await loadPresets({ select: name, force: true });
    });
  }

  async function deletePreset() {
    await runTask('delete preset', async () => {
      const current = requireCurrentPreset();
      await api.deletePreset(current.name);
      log(`Deleted preset ${current.name}.`);
      toast.success(`Deleted ${current.name}`);
      state.selectedPresetName = '';
      populatePreset(null);
      await loadPresets({ force: true });
    });
  }

  async function toggleAutostart(name: string, enabled: boolean) {
    await runTask('set autostart', async () => {
      await api.setPresetAutostart(name, enabled);
      log(`Autostart ${enabled ? 'enabled' : 'disabled'} for ${name}.`);
      toast.success(`Autostart ${enabled ? 'enabled' : 'disabled'} for ${name}`);
      await loadPresets({ force: true, preserveDirty: true });
    });
  }

  async function cleanupPreset() {
    await runTask('cleanup preset', async () => {
      const current = requireCurrentPreset();
      await api.cleanupPreset(current.name);
      log(`Cleaned up unavailable preset ${current.name}.`);
      toast.success(`Cleaned up ${current.name}`);
      state.selectedPresetName = '';
      populatePreset(null);
      await refreshServerInfo();
      await loadPresets({ force: true });
      await refreshRuntimeStatus();
    });
  }

  async function deleteLocalModel(model: LocalModel) {
    await runTask('delete local model', async () => {
      await api.deleteLocalModel(model.path, true);
      log(`Deleted local model ${model.filename}.`);
      toast.success(`Deleted ${model.filename}`);
      await loadLocalModels();
      await loadPresets({ force: true });
      await refreshServerInfo();
    });
  }

  async function reloadSelectedPreset() {
    await selectPreset(requireSelectedPreset(), { force: true, skipDirtyCheck: true });
  }

  async function savePreset() {
    await runTask('save preset', async () => {
      const name = requireSelectedPreset();
      await api.replacePreset(name, { entries: state.draftEntries });
      log('Saved preset and refreshed Router sources.');
      toast.success('Saved preset');
      await afterPresetMutation(name);
    });
  }

  async function afterPresetMutation(name: string) {
    await loadPresets({ select: name, force: true });
    await refreshRuntimeStatus();
  }

  async function resolveModel() {
    await runTask('validate model', async () => {
      const url = state.modelUrl.trim();
      if (!url) throw new Error('model URL is required');
      const resolution = await api.resolveModel(url);
      state.modelResolution = resolution;
      state.selectedModelFile = resolution.files?.[0]?.filename || '';
      log(`Resolved ${resolution.source.owner}/${resolution.source.repo}.`);
    });
  }

  async function loadModelCatalog() {
    state.catalogLoading = true;
    try {
      const data = await api.listModelCatalog(state.catalogQuery);
      state.catalogModels = data.models || [];
      state.catalogMachine = data.machine || null;
      state.catalogCache = data.cache || null;
      state.catalogErrors = data.errors || [];
      if (!state.selectedCatalogModelId && state.catalogModels.length) state.selectedCatalogModelId = state.catalogModels[0].id;
      if (data.cache?.refreshing) connectCatalogEvents();
    } catch (error) {
      showError(error);
    } finally {
      state.catalogLoading = false;
    }
  }

  async function loadLocalModels() {
    state.localModelsLoading = true;
    try {
      const data = await api.listLocalModels();
      state.localModels = data.models || [];
    } catch (error) {
      showError(error);
    } finally {
      state.localModelsLoading = false;
    }
  }

  async function refreshResourcesAndCatalog() {
    await refreshSignals();
    await loadModelCatalog();
  }

  function connectCatalogEvents() {
    if (catalogEvents) return;
    const controller = new AbortController();
    catalogEvents = controller;
    void watchCatalogEvents(controller);
  }

  async function watchCatalogEvents(controller: AbortController) {
    try {
      for await (const event of api.watchModelCatalog(controller.signal)) {
        if (event.type !== 'catalog_refresh') continue;
        if (!event.ok) {
          if (event.error) showError(new Error(event.error));
          continue;
        }
        await loadModelCatalog();
      }
    } catch (error) {
      if (!controller.signal.aborted) showError(error);
    } finally {
      if (catalogEvents === controller) catalogEvents = null;
    }
  }

  function closeCatalogEvents() {
    catalogEvents?.abort();
    catalogEvents = null;
  }

  function useCatalogFile(model: CatalogModel, file: ModelFile | undefined = model.best_file) {
    if (!file) return;
    state.selectedCatalogModelId = model.id;
    state.modelUrl = model.url;
    state.modelResolution = {
      source: {
        url: model.url,
        owner: model.owner,
        repo: model.repo
      },
      llama_cpp: {
        compatible: true,
        hf_ref: model.id
      },
      files: model.files || []
    };
    state.selectedModelFile = file.filename;
    log(`Selected ${model.id} ${file.filename}.`);
  }

  async function startDownload() {
    await runTask('download model', async () => {
      if (!state.modelResolution) throw new Error('validate a model URL first');
      if (!state.selectedModelFile) throw new Error('select a GGUF file first');
      const data = await api.startModelDownload({ url: state.modelResolution.source.url, filename: state.selectedModelFile });
      const job = requireDownload(data.download);
      state.downloads[job.id] = job;
      state.activeModelDownloadId = job.id;
      log(`Started model download ${job.filename}.`);
      startPolling(job.id);
    });
  }

  async function cancelDownload() {
    await runTask('cancel model download', async () => {
      if (!state.activeModelDownloadId) throw new Error('download a model first');
      const data = await api.cancelModelDownload(state.activeModelDownloadId);
      const job = requireDownload(data.download);
      state.downloads[job.id] = job;
      stopPolling();
      log(`Cancelled model download ${job.filename}.`);
    });
  }

  async function applyToPreset() {
    await runTask('apply model', async () => {
      if (!state.activeModelDownloadId) throw new Error('download a model first');
      const name = requireSelectedPreset();
      await api.applyModelToPreset(state.activeModelDownloadId, {
        preset: name
      });
      log(`Applied model to ${name}.`);
      await loadPresets({ select: name, force: true });
      state.modelApplyPreview = null;
    });
  }

  async function previewApplyToPreset() {
    await runTask('preview model apply', async () => {
      if (!state.activeModelDownloadId) throw new Error('download a model first');
      const name = requireSelectedPreset();
      const data = await api.applyModelToPreset(state.activeModelDownloadId, {
        preset: name,
        preview: true
      });
      state.modelApplyPreview = data.preview as { original?: string; updated?: string };
      log(`Previewed model apply to ${name}.`);
    });
  }

  function startPolling(id: string) {
    stopPolling();
    pollTimer = window.setInterval(() => pollDownload(id).catch(showError), 1000);
    pollDownload(id).catch(showError);
  }

  function stopPolling() {
    if (pollTimer) window.clearInterval(pollTimer);
    pollTimer = null;
  }

  async function pollDownload(id: string) {
    const data = await api.getModelDownload(id);
    const job = requireDownload(data.download);
    state.downloads[id] = job;
    if (isTerminalDownloadState(job.state)) stopPolling();
  }

  function requireSelectedPreset() {
    if (!state.selectedPresetName) throw new Error('select a preset first');
    return state.selectedPresetName;
  }

  function requireCurrentPreset() {
    if (!state.currentPreset) throw new Error('select a preset first');
    return state.currentPreset;
  }

  function requireDownload(job: ModelDownload | undefined) {
    if (!job) throw new Error('download response missing job');
    return job;
  }

  function runtimeHint() {
    const selected = state.selectedPresetName;
    if (!selected || state.activePresetNames.length === 0) return '';
    if (!state.activePresetNames.includes(selected)) {
      return `Restart will start ${selected}; active: ${state.activePresetNames.join(', ')}.`;
    }
    return `Restart will reload ${selected}.`;
  }

  function isPresetActive(name: string) {
    return selectIsPresetActive(state.activePresetNames, name);
  }

  function activeDownload() {
    return state.downloads[state.activeModelDownloadId] || null;
  }

  return client;
}

export type LlamaRigClient = ReturnType<typeof createLlamaRigClient>;
