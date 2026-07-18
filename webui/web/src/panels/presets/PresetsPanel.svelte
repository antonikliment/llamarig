<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy';
  import Plus from '@lucide/svelte/icons/plus';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import Info from '@lucide/svelte/icons/info';
  import X from '@lucide/svelte/icons/x';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import { createLlamaServerParamLookup, mergeLlamaServerParams } from '../../lib/data/llamaServerParams';
  import { unknownPresetKeys } from '../../lib/presetValidation';
  import { presetTarget } from '../../lib/state/selectors';
  import ParamKeyCombobox from './ParamKeyCombobox.svelte';
  import type { LlamaRigClient } from '../../lib/setup/createLlamaRigClient.svelte';
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button, buttonVariants } from '$lib/components/ui/button';
  import * as Card from '$lib/components/ui/card';
  import * as Dialog from '$lib/components/ui/dialog';
  import * as Empty from '$lib/components/ui/empty';
  import * as Field from '$lib/components/ui/field';
  import { Input } from '$lib/components/ui/input';
  import * as Item from '$lib/components/ui/item';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import * as Select from '$lib/components/ui/select';
  import * as Tooltip from '$lib/components/ui/tooltip';

  let { app }: { app: LlamaRigClient } = $props();
  const appState = $derived(app.state);
  let createOpen = $state(false);
  let duplicateOpen = $state(false);
  let deleteOpen = $state(false);
  let cleanupOpen = $state(false);
  let discardOpen = $state(false);
  let pendingPreset = $state('');
  let newName = $state('');
  let newTemplate = $state('single');
  let duplicateName = $state('');

  const effectiveLlamaParams = $derived(mergeLlamaServerParams(appState.llamaServerParams));
  const findLlamaParam = $derived(createLlamaServerParamLookup(effectiveLlamaParams));
  const invalidKeys = $derived(unknownPresetKeys(appState.draftEntries, effectiveLlamaParams));

  const templateOptions = [
    { value: 'single', label: 'Single model' },
    { value: 'directory', label: 'Models directory' },
    { value: 'blank', label: 'Blank' }
  ];

  function requestPreset(name: string) {
    if (!appState.dirty.entries) return app.selectPreset(name);
    pendingPreset = name;
    discardOpen = true;
  }

  async function createPreset(event: SubmitEvent) {
    event.preventDefault();
    await app.createPreset(newName, newTemplate);
    if (!app.errorMessage) {
      createOpen = false;
      newName = '';
    }
  }

  async function duplicatePreset(event: SubmitEvent) {
    event.preventDefault();
    await app.duplicatePreset(duplicateName);
    if (!app.errorMessage) duplicateOpen = false;
  }

  async function deletePreset() {
    await app.deletePreset();
    if (!app.errorMessage) deleteOpen = false;
  }

  function removeEntry(index: number) {
    appState.draftEntries = appState.draftEntries.filter((_, i) => i !== index);
    appState.dirty.entries = true;
  }

  function addEntry() {
    appState.draftEntries = [...appState.draftEntries, { key: '', value: '' }];
    appState.dirty.entries = true;
  }

  function paramHint(key: string) {
    return findLlamaParam(key);
  }

  function valueListId(index: number) {
    return `llama-param-values-${index}`;
  }

  function valueSuggestions(key: string): string[] | undefined {
    const param = findLlamaParam(key);
    if (!param) return undefined;
    if (param.type === 'bool') return ['true', 'false'];
    if (param.enumValues) return param.enumValues;
    if (param.default) return [param.default];
    return undefined;
  }

  function unavailable(preset = appState.currentPreset) {
    return preset?.source_status === 'unavailable';
  }

  function checking(preset = appState.currentPreset) {
    return preset?.source_status === 'checking';
  }
</script>

<div class="grid gap-4 xl:grid-cols-[22rem_minmax(0,1fr)]">
  <Card.Root>
    <Card.Header>
      <Card.Title>Server presets</Card.Title>
      <Card.Description>Select or manage a llama-server preset.</Card.Description>
      <Card.Action><Button size="icon-sm" variant="outline" aria-label="Reload presets" onclick={() => app.loadPresets({ force: true })} disabled={appState.busy}><RefreshCw /></Button></Card.Action>
    </Card.Header>
    <Card.Content class="space-y-4">
      <div class="flex flex-wrap gap-2">
        <Dialog.Root bind:open={createOpen}>
          <Dialog.Trigger class={buttonVariants({ size: 'sm' })}><Plus /> New</Dialog.Trigger>
          <Dialog.Content>
            <Dialog.Header><Dialog.Title>Create preset</Dialog.Title><Dialog.Description>Choose a name and starter configuration.</Dialog.Description></Dialog.Header>
            <form class="space-y-4" onsubmit={createPreset}>
              <Field.Field><Field.Label for="preset-name">Name</Field.Label><Input id="preset-name" bind:value={newName} autocomplete="off" /></Field.Field>
              <Field.Field>
                <Field.Label for="preset-template">Template</Field.Label>
                <Select.Root type="single" bind:value={newTemplate}>
                  <Select.Trigger id="preset-template">{templateOptions.find((option) => option.value === newTemplate)?.label}</Select.Trigger>
                  <Select.Content>{#each templateOptions as option}<Select.Item value={option.value}>{option.label}</Select.Item>{/each}</Select.Content>
                </Select.Root>
              </Field.Field>
              <Dialog.Footer><Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close><Button type="submit" disabled={appState.busy}>Create</Button></Dialog.Footer>
            </form>
          </Dialog.Content>
        </Dialog.Root>

        <Dialog.Root bind:open={duplicateOpen}>
          <Dialog.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={!appState.currentPreset} onclick={() => (duplicateName = `${appState.currentPreset?.name || ''}-copy`)}><Copy /> Duplicate</Dialog.Trigger>
          <Dialog.Content>
            <Dialog.Header><Dialog.Title>Duplicate preset</Dialog.Title><Dialog.Description>Copy {appState.currentPreset?.name} into a new preset.</Dialog.Description></Dialog.Header>
            <form class="space-y-4" onsubmit={duplicatePreset}>
              <Field.Field><Field.Label for="duplicate-name">New name</Field.Label><Input id="duplicate-name" bind:value={duplicateName} autocomplete="off" /></Field.Field>
              <Dialog.Footer><Dialog.Close class={buttonVariants({ variant: 'outline' })}>Cancel</Dialog.Close><Button type="submit" disabled={appState.busy}>Duplicate</Button></Dialog.Footer>
            </form>
          </Dialog.Content>
        </Dialog.Root>

        <AlertDialog.Root bind:open={deleteOpen}>
          <AlertDialog.Trigger class={buttonVariants({ variant: 'destructive', size: 'sm' })} disabled={!appState.currentPreset}><Trash2 /> Delete</AlertDialog.Trigger>
          <AlertDialog.Content>
            <AlertDialog.Header><AlertDialog.Title>Delete {appState.currentPreset?.name}?</AlertDialog.Title><AlertDialog.Description>This removes the preset configuration and cannot be undone.</AlertDialog.Description></AlertDialog.Header>
            <AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={deletePreset}>Delete preset</AlertDialog.Action></AlertDialog.Footer>
          </AlertDialog.Content>
        </AlertDialog.Root>
      </div>

      <ScrollArea class="panel-scroll pr-3">
        <Item.Group>
          {#each appState.presets as preset}
            <Item.Root
              variant={appState.selectedPresetName === preset.name ? 'muted' : 'default'}
              size="sm"
              class={appState.selectedPresetName === preset.name ? 'border-primary/50 bg-primary/10 shadow-sm' : 'hover:bg-muted/50'}
            >
              <Item.Content>
                <Button variant="ghost" class="h-auto w-full cursor-pointer justify-start px-0 text-left hover:bg-transparent" aria-pressed={appState.selectedPresetName === preset.name} onclick={() => requestPreset(preset.name)}>
                  <Item.Title>{preset.name}</Item.Title>
                  <Item.Description>{presetTarget(preset)}</Item.Description>
                </Button>
              </Item.Content>
              {#if appState.activePresetNames.includes(preset.name)}<Item.Actions><Badge class="bg-success/15 text-success border-success/30" variant="outline">active</Badge></Item.Actions>{/if}
              {#if unavailable(preset)}<Item.Actions><Badge class="bg-destructive/15 text-destructive border-destructive/30" variant="outline">unavailable</Badge></Item.Actions>{/if}
              {#if checking(preset)}<Item.Actions><Badge variant="secondary">checking</Badge></Item.Actions>{/if}
            </Item.Root>
          {:else}
            <Empty.Root><Empty.Header><Empty.Title>No presets</Empty.Title><Empty.Description>Create a server preset to begin.</Empty.Description></Empty.Header></Empty.Root>
          {/each}
        </Item.Group>
      </ScrollArea>
    </Card.Content>
  </Card.Root>

  <Card.Root class="min-w-0">
    <Card.Header>
      <Card.Title>{appState.currentPreset?.name || 'No preset selected'}</Card.Title>
      <Card.Description>INI key/value entries for this llama-server preset.</Card.Description>
      <Card.Action>
        {#if checking()}
          <Badge variant="secondary">Checking source</Badge>
        {:else if unavailable()}
          <Badge variant="outline" class="bg-destructive/15 text-destructive border-destructive/30">Unavailable</Badge>
        {:else if appState.dirty.entries}
          <Badge variant="outline" class="bg-destructive/15 text-destructive border-destructive/30">Unsaved changes</Badge>
        {:else}
          <Badge variant="secondary">Saved</Badge>
        {/if}
      </Card.Action>
    </Card.Header>
    <Card.Content class="space-y-4">
      {#if appState.currentPreset}
        {#if unavailable()}
          <div class="rounded-md border border-destructive/40 bg-destructive/10 p-4 space-y-3">
            <div><p class="font-medium text-destructive">Preset source unavailable</p><p class="text-sm text-muted-foreground break-all">{appState.currentPreset.source_error}</p></div>
            <AlertDialog.Root bind:open={cleanupOpen}>
              <AlertDialog.Trigger class={buttonVariants({ variant: 'destructive', size: 'sm' })}><Trash2 /> Cleanup preset</AlertDialog.Trigger>
              <AlertDialog.Content>
                <AlertDialog.Header><AlertDialog.Title>Cleanup {appState.currentPreset.name}?</AlertDialog.Title><AlertDialog.Description>This removes the unavailable preset and clears matching default or autostart references. This cannot be undone.</AlertDialog.Description></AlertDialog.Header>
                <AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={app.cleanupPreset}>Cleanup preset</AlertDialog.Action></AlertDialog.Footer>
              </AlertDialog.Content>
            </AlertDialog.Root>
          </div>
        {/if}
        <div class="space-y-2">
          {#each appState.draftEntries as entry, i (entry)}
            {@const hint = paramHint(entry.key)}
            {@const valueOptions = valueSuggestions(entry.key)}
            {@const invalid = invalidKeys.includes(entry.key.trim())}
            <div class="space-y-1">
              <div class="flex gap-2 items-center">
                <div class={invalid ? 'rounded-md ring-1 ring-destructive' : ''}>
                  <ParamKeyCombobox
                    bind:value={entry.key}
                    params={effectiveLlamaParams}
                    disabled={appState.busy}
                    oninput={() => (appState.dirty.entries = true)}
                  />
                </div>
                <Input
                  class="flex-1 font-mono text-sm"
                  bind:value={entry.value}
                  placeholder="value"
                  list={valueOptions ? valueListId(i) : undefined}
                  oninput={() => (appState.dirty.entries = true)}
                  disabled={appState.busy}
                />
                {#if valueOptions}
                  <datalist id={valueListId(i)}>
                    {#each valueOptions as option}<option value={option}></option>{/each}
                  </datalist>
                {/if}
                {#if invalid}
                  <Tooltip.Provider>
                    <Tooltip.Root>
                      <Tooltip.Trigger>
                        <TriangleAlert class="size-4 text-destructive shrink-0" />
                      </Tooltip.Trigger>
                      <Tooltip.Content class="max-w-64">
                        <p>Not a recognized llama-server flag or router key. This will make the router fail to start.</p>
                      </Tooltip.Content>
                    </Tooltip.Root>
                  </Tooltip.Provider>
                {:else if hint}
                  <Tooltip.Provider>
                    <Tooltip.Root>
                      <Tooltip.Trigger>
                        <Info class="size-4 text-muted-foreground shrink-0" />
                      </Tooltip.Trigger>
                      <Tooltip.Content class="max-w-64">
                        <p>{hint.description}</p>
                        {#if hint.default}<p class="font-mono text-xs mt-1">default: {hint.default}</p>{/if}
                      </Tooltip.Content>
                    </Tooltip.Root>
                  </Tooltip.Provider>
                {:else}
                  <div class="size-4 shrink-0"></div>
                {/if}
                <Button size="icon-sm" variant="ghost" onclick={() => removeEntry(i)} disabled={appState.busy} aria-label="Remove entry"><X /></Button>
              </div>
            </div>
          {/each}
        </div>
        <Button size="sm" variant="outline" onclick={addEntry} disabled={appState.busy}><Plus /> Add entry</Button>
        {#if invalidKeys.length}
          <p class="flex items-center gap-1.5 text-sm text-destructive"><TriangleAlert class="size-4 shrink-0" /> Unrecognized key{invalidKeys.length > 1 ? 's' : ''}: {invalidKeys.join(', ')}. Fix before saving.</p>
        {/if}
        <div class="flex flex-wrap gap-2">
          {#if appState.dirty.entries}
            <AlertDialog.Root>
              <AlertDialog.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={appState.busy}>Reload</AlertDialog.Trigger>
              <AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>Discard unsaved changes?</AlertDialog.Title><AlertDialog.Description>Reloading replaces current entries.</AlertDialog.Description></AlertDialog.Header><AlertDialog.Footer><AlertDialog.Cancel>Keep editing</AlertDialog.Cancel><AlertDialog.Action onclick={app.reloadSelectedPreset}>Discard and reload</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content>
            </AlertDialog.Root>
          {:else}
            <Button size="sm" variant="outline" onclick={app.reloadSelectedPreset} disabled={appState.busy}>Reload</Button>
          {/if}
          <Button size="sm" onclick={app.savePreset} disabled={appState.busy || invalidKeys.length > 0}>Save</Button>
          {#if appState.dirty.entries}
            <AlertDialog.Root>
              <AlertDialog.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={appState.busy || unavailable() || checking() || invalidKeys.length > 0 || app.isPresetActive(appState.selectedPresetName)}>Start</AlertDialog.Trigger>
              <AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>Start without saving?</AlertDialog.Title><AlertDialog.Description>Runtime will use saved preset content, not current editor changes.</AlertDialog.Description></AlertDialog.Header><AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={app.startSelectedPreset}>Start saved preset</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content>
            </AlertDialog.Root>
          {:else}
            <Button size="sm" variant="outline" onclick={app.startSelectedPreset} disabled={appState.busy || unavailable() || checking() || invalidKeys.length > 0 || app.isPresetActive(appState.selectedPresetName)}>Start</Button>
          {/if}
        </div>
      {:else}
        <Empty.Root><Empty.Header><Empty.Title>No preset selected</Empty.Title><Empty.Description>Select a preset from the list to edit its entries.</Empty.Description></Empty.Header></Empty.Root>
      {/if}
    </Card.Content>
  </Card.Root>
</div>

<AlertDialog.Root bind:open={discardOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header><AlertDialog.Title>Discard unsaved changes?</AlertDialog.Title><AlertDialog.Description>Switching presets replaces current entries.</AlertDialog.Description></AlertDialog.Header>
    <AlertDialog.Footer><AlertDialog.Cancel>Keep editing</AlertDialog.Cancel><AlertDialog.Action onclick={() => app.selectPreset(pendingPreset, { skipDirtyCheck: true })}>Discard and switch</AlertDialog.Action></AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
