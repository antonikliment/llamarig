<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import Download from '@lucide/svelte/icons/download';
  import Info from '@lucide/svelte/icons/info';
  import Plus from '@lucide/svelte/icons/plus';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Search from '@lucide/svelte/icons/search';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import X from '@lucide/svelte/icons/x';
  import { formatBytes, formatContextLength, formatDate, formatParamCount } from '../../lib/formatting';
  import { mergeLlamaServerParams } from '../../lib/data/llamaServerParams';
  import type { LlamaRigClient } from '../../lib/setup/createLlamaRigClient.svelte';
  import type { CatalogModel, LocalModel, ModelFile } from '../../lib/types';
  import { modelPresetEntries } from '../../lib/presetTemplates';
  import { unknownPresetKeys } from '../../lib/presetValidation';
  import DiffPreview from '../../components/editor/DiffPreview.svelte';
  import type { PresetEntry } from '../../lib/types';
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button, buttonVariants } from '$lib/components/ui/button';
  import * as Card from '$lib/components/ui/card';
  import * as Collapsible from '$lib/components/ui/collapsible';
  import * as Dialog from '$lib/components/ui/dialog';
  import * as Empty from '$lib/components/ui/empty';
  import * as Field from '$lib/components/ui/field';
  import { Input } from '$lib/components/ui/input';
  import * as Item from '$lib/components/ui/item';
  import * as RadioGroup from '$lib/components/ui/radio-group';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import { Separator } from '$lib/components/ui/separator';
  import * as Select from '$lib/components/ui/select';
  import * as Tabs from '$lib/components/ui/tabs';
  import * as Tooltip from '$lib/components/ui/tooltip';
  import {
    bestGPUTotalVRAM,
    downloadStatusClass,
    estimateLocalFit,
    filterLocalModels,
    fitBadge,
    fitMeterColor,
    localModelFilterCounts,
    modelMetadataChips,
    quantFromFilename,
    rankedResourceSummary,
    type LocalModelFilter
  } from './catalogPresentation';
  import { uniquePresetName } from './presetDraft';
  import ParamKeyCombobox from '../presets/ParamKeyCombobox.svelte';

  let { app }: { app: LlamaRigClient } = $props();
  const appState = $derived(app.state);
  let activeTab = $state('mine');
  let localFilter = $state<LocalModelFilter>('all');
  let expandedModel = $state<string | null>(null);
  let createDialogOpen = $state(false);
  let selectedLocalModel = $state<LocalModel | null>(null);
  let draftName = $state('');
  let draftEntries = $state<PresetEntry[]>([]);
  const effectiveLlamaParams = $derived(mergeLlamaServerParams(appState.llamaServerParams));
  const draftInvalidKeys = $derived(unknownPresetKeys(draftEntries, effectiveLlamaParams));
  const sortOptions = [
    { value: 'downloads', label: 'Downloads' },
    { value: 'trending', label: 'Trending' },
    { value: 'modified', label: 'Recently modified' }
  ];
  const fitOptions = [
    { value: 'fits', label: 'Fits' },
    { value: 'marginal', label: 'Fits or marginal' },
    { value: 'all', label: 'All' }
  ];
  const localFilterOptions: { value: LocalModelFilter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'serving', label: 'Serving' },
    { value: 'in_preset', label: 'In preset' },
    { value: 'unused', label: 'Unused' }
  ];

  const filteredLocalModels = $derived(filterLocalModels(appState.localModels, localFilter, appState.activePresetNames));
  const localFilterCounts = $derived(localModelFilterCounts(appState.localModels, appState.activePresetNames));
  const localDiskBytes = $derived(appState.localModels.reduce((sum, model) => sum + (model.size_bytes || 0), 0));
  const capacityLabel = $derived.by(() => {
    const vram = appState.catalogMachine?.vram_bytes || bestGPUTotalVRAM(appState.signals);
    if (vram) return `${formatBytes(vram)} VRAM`;
    const ram = appState.catalogMachine?.total_ram_bytes || appState.signals?.memory?.total_bytes;
    return ram ? `${formatBytes(ram)} RAM` : 'unknown capacity';
  });

  function fileSummary(file: ModelFile | undefined) {
    if (!file) return '';
    return [file.filename, file.quant || 'unknown quant', formatBytes(file.size_bytes), file.fit_level || ''].filter(Boolean).join(' · ');
  }

  function resolutionSpecs() {
    const resolution = appState.modelResolution;
    return [
      formatParamCount(resolution?.params),
      resolution?.is_moe ? 'MoE' : '',
      resolution?.context_length ? `${formatContextLength(resolution.context_length)} ctx` : '',
      resolution?.architecture || ''
    ].filter(Boolean);
  }

  function isServing(model: LocalModel) {
    return (model.used_by_presets || []).some((name) => appState.activePresetNames.includes(name));
  }

  function openCreatePreset(model: LocalModel) {
    selectedLocalModel = model;
    draftName = uniquePresetName(model.filename, appState.presets);
    draftEntries = modelPresetEntries(model.path);
    createDialogOpen = true;
  }

  function toggleDetails(model: LocalModel) {
    expandedModel = expandedModel === model.path ? null : model.path;
  }

  function goCatalog() {
    activeTab = 'catalog';
  }

  async function downloadCatalogModel(model: CatalogModel) {
    app.useCatalogFile(model);
    await app.startDownload();
  }

  async function createLocalPreset(event: SubmitEvent) {
    event.preventDefault();
    await app.createLocalPreset(draftName, draftEntries);
    if (!app.errorMessage) {
      createDialogOpen = false;
      selectedLocalModel = null;
      draftName = '';
      draftEntries = [];
    }
  }

  function usedByPresets(model: LocalModel) {
    return model.used_by_presets || [];
  }

  function cascadePresets(model: LocalModel) {
    return model.model_path_presets || [];
  }

</script>

{#snippet fitMeter(usedPct: number, needPct: number, level: string | undefined)}
  <div class="relative h-1.5 overflow-hidden rounded-full bg-muted">
    <div class="h-full bg-muted-foreground/40" style={`width:${usedPct}%`}></div>
    <div class={`absolute top-0 h-full rounded-r-full ${fitMeterColor(level)}`} style={`left:${usedPct}%;width:${needPct}%`}></div>
  </div>
{/snippet}

{#snippet fitInfoTooltip()}
  <Tooltip.Provider>
    <Tooltip.Root>
      <Tooltip.Trigger class="text-muted-foreground"><Info class="size-3.5" /></Tooltip.Trigger>
      <Tooltip.Content class="max-w-64">
        Estimates only — models over free VRAM can still run by offloading layers to CPU and system RAM (slower).
      </Tooltip.Content>
    </Tooltip.Root>
  </Tooltip.Provider>
{/snippet}

<Tabs.Root bind:value={activeTab} class="space-y-4">
  <Tabs.List>
    <Tabs.Trigger value="mine">My models <Badge variant="secondary">{appState.localModels.length}</Badge></Tabs.Trigger>
    <Tabs.Trigger value="catalog">Catalog</Tabs.Trigger>
  </Tabs.List>

  <Tabs.Content value="mine">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 class="text-2xl font-bold tracking-tight">My models</h2>
          <p class="mt-1 flex items-center gap-1.5 text-sm text-muted-foreground">
            {appState.localModels.length} downloaded · {formatBytes(localDiskBytes)} on disk · fit estimated against <strong class="font-semibold text-foreground">{capacityLabel}</strong>
            {@render fitInfoTooltip()}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <Button size="sm" variant="outline" onclick={app.loadLocalModels} disabled={appState.localModelsLoading}><RefreshCw class={appState.localModelsLoading ? 'animate-spin' : ''} /> Refresh</Button>
          <Button size="sm" onclick={goCatalog}><Plus /> Pull model</Button>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        {#each localFilterOptions as option}
          <Button
            type="button"
            size="sm"
            variant={localFilter === option.value ? 'secondary' : 'ghost'}
            class={`rounded-full ${localFilter === option.value ? 'text-primary' : 'text-muted-foreground'}`}
            onclick={() => (localFilter = option.value)}
          >
            {option.label} · {localFilterCounts[option.value]}
          </Button>
        {/each}
      </div>

      <ScrollArea class="panel-scroll pr-3">
        <div class="space-y-3">
          {#each filteredLocalModels as model (model.path)}
            {@const presets = usedByPresets(model)}
            {@const cascades = cascadePresets(model)}
            {@const fit = estimateLocalFit(model, appState.signals, appState.catalogMachine)}
            {@const serving = isServing(model)}
            <div class="rounded-xl border p-4 transition-colors hover:border-primary/40">
              <div class="grid grid-cols-1 items-center gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,14rem)_minmax(0,14rem)]">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-semibold">{model.filename}</span>
                    {#if quantFromFilename(model.filename)}<Badge variant="secondary" class="font-mono text-[10px]">{quantFromFilename(model.filename)}</Badge>{/if}
                    {#if serving}<Badge variant="outline" class="border-success/30 bg-success/15 text-success">Serving</Badge>{/if}
                  </div>
                  <div class="mt-1 truncate text-xs text-muted-foreground">{model.path}</div>
                </div>
                <div class="min-w-0">
                  {#if fit}
                    <div class="flex justify-between text-xs text-muted-foreground">
                      <span>{formatBytes(model.size_bytes)}</span>
                      <span class="font-semibold" class:text-success={fit.level === 'fits'} class:text-warning={fit.level === 'marginal'} class:text-destructive={fit.level === 'too_large'}>
                        {fitBadge(fit.level)?.label}
                      </span>
                    </div>
                    {@render fitMeter(fit.usedPct, fit.needPct, fit.level)}
                    <div class="mt-1 text-[11px] text-muted-foreground">{formatBytes(fit.usedBytes)} in use + ~{formatBytes(fit.needBytes)}</div>
                  {:else}
                    <div class="text-xs text-muted-foreground">{formatBytes(model.size_bytes)}</div>
                  {/if}
                </div>
                <div class="flex min-w-0 flex-wrap justify-end gap-2">
                  <Button size="sm" variant="outline" onclick={() => toggleDetails(model)}>
                    Details <ChevronDown class={`size-3.5 transition-transform ${expandedModel === model.path ? 'rotate-180' : ''}`} />
                  </Button>
                  {#if !presets.length}
                    <Button size="sm" onclick={() => openCreatePreset(model)}><Plus /> Create preset</Button>
                  {:else}
                    <Badge variant="secondary" class="max-w-40 truncate" title={`in: ${presets.join(', ')}`}>in: {presets.join(', ')}</Badge>
                  {/if}
                </div>
              </div>
              {#if expandedModel === model.path}
                <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t pt-3 text-sm">
                  <dl class="grid grid-cols-2 gap-x-6 gap-y-1 text-xs sm:grid-cols-3">
                    <div><dt class="text-muted-foreground">Path</dt><dd class="truncate">{model.path}</dd></div>
                    <div><dt class="text-muted-foreground">Modified</dt><dd>{formatDate(model.modified_at)}</dd></div>
                    <div><dt class="text-muted-foreground">Size</dt><dd>{formatBytes(model.size_bytes)}</dd></div>
                  </dl>
                  <AlertDialog.Root>
                    <AlertDialog.Trigger class={buttonVariants({ variant: 'destructive', size: 'sm' })}><Trash2 /> Delete</AlertDialog.Trigger>
                    <AlertDialog.Content>
                      <AlertDialog.Header><AlertDialog.Title>Delete {model.filename}?</AlertDialog.Title><AlertDialog.Description>{#if cascades.length}This also deletes presets {cascades.join(', ')} and clears their default/autostart references. Active presets block deletion.{:else if presets.length}This file is discovered through {presets.join(', ')}; those directory presets remain.{:else}This deletes the local model file ({formatBytes(model.size_bytes)}) and cannot be undone.{/if}</AlertDialog.Description></AlertDialog.Header>
                      <AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={() => app.deleteLocalModel(model)}>Delete model</AlertDialog.Action></AlertDialog.Footer>
                    </AlertDialog.Content>
                  </AlertDialog.Root>
                </div>
              {/if}
            </div>
          {:else}
            <Empty.Root><Empty.Header><Empty.Title>No models yet</Empty.Title><Empty.Description>Pull a model from the catalog to get started.</Empty.Description></Empty.Header></Empty.Root>
          {/each}
        </div>
      </ScrollArea>
    </div>
  </Tabs.Content>

  <Tabs.Content value="catalog">
    <div class="grid gap-4 lg:grid-cols-[1fr_minmax(20rem,24rem)]">
      <Card.Root>
        <Card.Header>
          <Card.Title>Recommended models</Card.Title>
          <Card.Description class="flex items-center gap-1.5">
            Ranked for {rankedResourceSummary(appState.catalogMachine, appState.signals)}
            {appState.catalogCache?.hit ? ` · cache ${appState.catalogCache.stale ? 'refreshing' : 'fresh'}` : ''}
            {@render fitInfoTooltip()}
          </Card.Description>
          <Card.Action><Button size="sm" variant="outline" onclick={app.refreshResourcesAndCatalog} disabled={appState.catalogLoading}><RefreshCw class={appState.catalogLoading ? 'animate-spin' : ''} /> Refresh</Button></Card.Action>
        </Card.Header>
        <Card.Content class="space-y-4">
          {#if appState.catalogErrors.length}
            <div role="alert" class="rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm text-warning-foreground dark:text-warning">
              <p class="flex items-center gap-2 font-medium"><TriangleAlert class="size-4" /> {appState.catalogErrors.length} catalog model{appState.catalogErrors.length === 1 ? '' : 's'} could not be loaded.</p>
              <Collapsible.Root class="mt-2"><Collapsible.Trigger class={buttonVariants({ variant: 'ghost', size: 'sm' })}>Show details</Collapsible.Trigger><Collapsible.Content><ul class="mt-1 list-disc space-y-1 pl-5">{#each appState.catalogErrors as error}<li>{error}</li>{/each}</ul></Collapsible.Content></Collapsible.Root>
            </div>
          {/if}
          <div class="grid gap-3 md:grid-cols-[minmax(10rem,1fr)_11rem_11rem_auto]">
            <Field.Field><Field.Label for="catalog-search">Search</Field.Label><Input id="catalog-search" bind:value={appState.catalogQuery.search} placeholder="qwen coder" /></Field.Field>
            <Field.Field><Field.Label for="catalog-sort">Sort</Field.Label><Select.Root type="single" bind:value={appState.catalogQuery.sort}><Select.Trigger id="catalog-sort">{sortOptions.find((option) => option.value === appState.catalogQuery.sort)?.label}</Select.Trigger><Select.Content>{#each sortOptions as option}<Select.Item value={option.value}>{option.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
            <Field.Field><Field.Label for="catalog-fit">Fit</Field.Label><Select.Root type="single" bind:value={appState.catalogQuery.min_fit}><Select.Trigger id="catalog-fit">{fitOptions.find((option) => option.value === appState.catalogQuery.min_fit)?.label}</Select.Trigger><Select.Content>{#each fitOptions as option}<Select.Item value={option.value}>{option.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
            <div class="flex items-end"><Button onclick={app.loadModelCatalog} disabled={appState.catalogLoading}><Search /> Apply</Button></div>
          </div>

          <ScrollArea class="panel-scroll pr-3">
            <div class="space-y-3">
              {#each appState.catalogModels as model}
                {@const fit = fitBadge(model.fit?.level)}
                {@const metadata = modelMetadataChips(model)}
                {@const capacityBytes = appState.catalogMachine?.vram_bytes || bestGPUTotalVRAM(appState.signals) || appState.catalogMachine?.total_ram_bytes || 0}
                <div class="rounded-xl border p-4 transition-colors hover:border-primary/40">
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <a class="font-semibold hover:underline" href={model.url} target="_blank" rel="noreferrer">{model.owner}/{model.repo}</a>
                      <div class="mt-0.5 text-xs text-muted-foreground">{fileSummary(model.best_file)}</div>
                    </div>
                    {#if fit}<Badge variant="outline" class={`shrink-0 ${fit.class}`}>{fit.label}</Badge>{/if}
                  </div>
                  {#if model.best_file?.estimated_vram_bytes || model.best_file?.estimated_ram_bytes}
                    {@const needBytes = model.best_file?.estimated_vram_bytes || model.best_file?.estimated_ram_bytes || 0}
                    {@const needPct = capacityBytes ? Math.min(100, (needBytes / capacityBytes) * 100) : 0}
                    <div class="mt-3">
                      <div class="flex justify-between text-xs text-muted-foreground">
                        <span>est. footprint{capacityBytes ? ` vs ${formatBytes(capacityBytes)}` : ''}</span>
                        <span class="font-semibold">~{formatBytes(needBytes)}</span>
                      </div>
                      {@render fitMeter(0, needPct, model.fit?.level)}
                    </div>
                  {/if}
                  <div class="mt-3 flex flex-wrap items-center gap-2">
                    {#each metadata.primary as chip}<Badge variant="secondary" class="text-[11px]">{chip}</Badge>{/each}
                    {#each metadata.capability as chip}<Badge variant="outline" class="text-[11px]">{chip}</Badge>{/each}
                    {#if model.best_file?.exists}<Badge variant="outline" class="border-success/30 bg-success/15 text-success">downloaded</Badge>{/if}
                    <div class="ml-auto flex gap-2">
                      <Button size="sm" variant="outline" onclick={() => app.useCatalogFile(model)} disabled={!model.best_file}>Choose file</Button>
                      <Button size="sm" onclick={() => downloadCatalogModel(model)} disabled={appState.busy || !model.best_file || model.best_file?.exists}><Download /> Download</Button>
                    </div>
                  </div>
                </div>
              {:else}
                <Empty.Root><Empty.Header><Empty.Title>No catalog models</Empty.Title><Empty.Description>Adjust filters or refresh machine resources.</Empty.Description></Empty.Header></Empty.Root>
              {/each}
            </div>
          </ScrollArea>
        </Card.Content>
      </Card.Root>

      <Card.Root class="self-start">
        <Card.Header><Card.Title>Manual Hugging Face model</Card.Title><Card.Description>Resolve a repository URL, choose a GGUF file, then download and apply it.</Card.Description></Card.Header>
        <Card.Content class="space-y-4">
          <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">1 · Resolve repository</p>
          <Field.Field><Field.Label for="model-url">Hugging Face model URL</Field.Label><div class="flex gap-2"><Input id="model-url" bind:value={appState.modelUrl} placeholder="https://huggingface.co/owner/repo" /><Button onclick={app.resolveModel} disabled={appState.busy}>Validate</Button></div></Field.Field>
          <dl class="grid gap-2 text-sm sm:grid-cols-2">
            <div><dt class="text-muted-foreground">Source</dt><dd>{#if appState.modelResolution?.source}<a class="text-primary hover:underline" href={appState.modelResolution.source.url} target="_blank" rel="noreferrer">{appState.modelResolution.source.owner}/{appState.modelResolution.source.repo}</a>{:else}-{/if}</dd></div>
            <div><dt class="text-muted-foreground">llama.cpp</dt><dd>{appState.modelResolution?.llama_cpp?.compatible ? `compatible (${appState.modelResolution.llama_cpp.hf_ref})` : '-'}</dd></div>
          </dl>
          {#if appState.modelResolution?.description}
            <p class="text-sm text-muted-foreground">{appState.modelResolution.description}</p>
          {/if}

          <Separator />
          <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">2 · Choose GGUF file</p>
          {#if resolutionSpecs().length}
            <div class="flex flex-wrap gap-2">
              {#each resolutionSpecs() as spec}<Badge variant="secondary">{spec}</Badge>{/each}
            </div>
          {/if}
          <RadioGroup.Root bind:value={appState.selectedModelFile} aria-label="Model file">
            {#each appState.modelResolution?.files || [] as file}
              {@const fileFit = fitBadge(file.fit_level)}
              <Field.Field orientation="horizontal" class="rounded-md border p-3">
                <RadioGroup.Item id={`model-${file.filename}`} value={file.filename} />
                <Field.Content><Field.Label for={`model-${file.filename}`}>{file.filename}</Field.Label><Field.Description>{[file.quant || 'unknown quant', formatBytes(file.size_bytes), file.exists ? 'downloaded' : ''].filter(Boolean).join(' · ')}</Field.Description></Field.Content>
                {#if fileFit}<Badge variant="outline" class={fileFit.class}>{fileFit.label}</Badge>{/if}
              </Field.Field>
            {:else}
              <Empty.Root><Empty.Header><Empty.Title>No model resolved</Empty.Title><Empty.Description>Validate a Hugging Face model URL first.</Empty.Description></Empty.Header></Empty.Root>
            {/each}
          </RadioGroup.Root>

          <Separator />
          <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">3 · Download &amp; apply</p>
          <div class="flex flex-wrap items-center gap-3">
            <Button onclick={app.startDownload} disabled={appState.busy || !appState.selectedModelFile}><Download /> Download</Button>
            <Button variant="outline" onclick={app.previewApplyToPreset} disabled={appState.busy || !appState.selectedPresetName || !app.canApplyDownload(app.activeDownload()?.state)}>Preview apply</Button>
            <Button variant="outline" onclick={app.applyToPreset} disabled={appState.busy || !appState.selectedPresetName || !app.canApplyDownload(app.activeDownload()?.state)}>Use in selected preset</Button>
          </div>
          {#if appState.modelApplyPreview}<DiffPreview original={appState.modelApplyPreview.original || ''} current={appState.modelApplyPreview.updated || ''} />{/if}
          {#if app.activeDownload()}
            {@const download = app.activeDownload()}
            <div class="space-y-2">
              <div class="relative h-2 overflow-hidden rounded-full bg-muted"><div class="h-full bg-primary" style={`width:${download?.percent || 0}%`}></div></div>
              <Item.Root variant="outline"><Item.Content><Item.Title>{download?.filename}</Item.Title><Item.Description>{formatBytes(download?.received_bytes)} / {formatBytes(download?.total_bytes)} · {(download?.percent || 0).toFixed(1)}%</Item.Description></Item.Content><Item.Actions>{#if download?.state === 'queued' || download?.state === 'running'}<Button size="sm" variant="outline" onclick={app.cancelDownload} disabled={appState.busy}><X /> Cancel</Button>{/if}<Badge variant="outline" class={downloadStatusClass(download?.state, download?.error)}>{download?.state}</Badge></Item.Actions></Item.Root>
              {#if download?.error}<p class="text-sm text-destructive">{download?.error}</p>{/if}
            </div>
          {/if}
        </Card.Content>
      </Card.Root>
    </div>
  </Tabs.Content>
</Tabs.Root>

<Dialog.Root bind:open={createDialogOpen}>
  <Dialog.Content class="sm:max-w-lg">
    <Dialog.Header>
      <Dialog.Title>Create model preset</Dialog.Title>
      <Dialog.Description>{selectedLocalModel?.path || 'Select an unconfigured model first.'}</Dialog.Description>
    </Dialog.Header>
    {#if selectedLocalModel}
      <form class="space-y-4" onsubmit={createLocalPreset}>
        <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">1 · Name preset</p>
        <Field.Field><Field.Label for="local-preset-name">Preset name</Field.Label><Input id="local-preset-name" bind:value={draftName} autocomplete="off" /></Field.Field>
        <p class="text-xs font-medium uppercase tracking-wider text-muted-foreground">2 · Edit entries</p>
        <div class="space-y-2">
          {#each draftEntries as entry, i (entry)}
            {@const invalid = draftInvalidKeys.includes(entry.key.trim())}
            <div class="flex gap-2 items-center">
              <div class={invalid ? 'rounded-md ring-1 ring-destructive' : ''}>
                <ParamKeyCombobox bind:value={entry.key} params={effectiveLlamaParams} disabled={appState.busy} />
              </div>
              <Input class="flex-1 font-mono" bind:value={entry.value} placeholder="value" />
              {#if invalid}
                <Tooltip.Provider>
                  <Tooltip.Root>
                    <Tooltip.Trigger><TriangleAlert class="size-4 shrink-0 text-destructive" /></Tooltip.Trigger>
                    <Tooltip.Content class="max-w-64">Not a recognized llama-server flag or router key. This will make the router fail to start.</Tooltip.Content>
                  </Tooltip.Root>
                </Tooltip.Provider>
              {/if}
              <Button type="button" variant="ghost" size="icon-sm" aria-label="Remove entry" class="text-muted-foreground hover:text-destructive" onclick={() => { draftEntries = draftEntries.filter((_, j) => j !== i); }}>×</Button>
            </div>
          {/each}
          <Button type="button" variant="ghost" size="sm" class="text-muted-foreground" onclick={() => { draftEntries = [...draftEntries, { key: '', value: '' }]; }}>+ Add entry</Button>
        </div>
        {#if draftInvalidKeys.length}
          <p class="flex items-center gap-1.5 text-sm text-destructive"><TriangleAlert class="size-4 shrink-0" /> Unrecognized key{draftInvalidKeys.length > 1 ? 's' : ''}: {draftInvalidKeys.join(', ')}. Fix before creating.</p>
        {/if}
        <Dialog.Footer><Button type="button" variant="outline" onclick={() => (createDialogOpen = false)}>Cancel</Button><Button type="submit" disabled={appState.busy || !draftName.trim() || draftInvalidKeys.length > 0}>Create preset</Button></Dialog.Footer>
      </form>
    {/if}
  </Dialog.Content>
</Dialog.Root>
