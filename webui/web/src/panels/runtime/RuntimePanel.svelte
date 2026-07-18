<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import Box from '@lucide/svelte/icons/box';
  import Radio from '@lucide/svelte/icons/radio';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Server from '@lucide/svelte/icons/server';
  import { formatBytes, formatDate } from '../../lib/formatting';
  import { gpuKey } from '../../lib/runtimeHistory';
  import type { LlamaRigClient } from '../../lib/setup/createLlamaRigClient.svelte';
  import ResourceMeter from '../../components/metrics/ResourceMeter.svelte';
  import TrendChart from '../../components/metrics/TrendChart.svelte';
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button, buttonVariants } from '$lib/components/ui/button';
  import * as Card from '$lib/components/ui/card';
  import * as Item from '$lib/components/ui/item';

  let { app }: { app: LlamaRigClient } = $props();
  const appState = $derived(app.state);

  function statusDot(status: string) {
    if (status === 'running') return 'bg-success';
    if (status === 'failed') return 'bg-destructive';
    return 'bg-warning';
  }

  function percent(used: number | undefined, total: number | undefined) {
    return total ? ((used || 0) / total) * 100 : 0;
  }

  function operationStatusClass(status: string | undefined) {
    if (status === 'failed') return 'border-destructive/30 bg-destructive/10 text-destructive';
    if (status === 'skipped') return 'border-warning/40 bg-warning/10 text-warning-foreground dark:text-warning';
    return 'border-success/30 bg-success/10 text-success';
  }

  const memory = $derived(appState.signals?.memory);
  const memoryPercent = $derived(
    memory?.used_percent ?? percent((memory?.total_bytes || 0) - (memory?.available_bytes || 0), memory?.total_bytes)
  );
  const capturedAt = $derived(appState.signals?.captured_at || '');
  const stale = $derived(!!appState.signalsLastError);

  async function refreshDashboard() {
    await app.runTask('refresh dashboard', async () => {
      await Promise.all([app.refreshRuntimeStatus(), app.refreshSignals(), app.refreshEvents()]);
    });
  }
</script>

<div class="space-y-5">
  <div class="flex flex-wrap items-end justify-between gap-3 border-b pb-4">
    <div class="space-y-1"><h2 class="text-xl font-semibold tracking-tight">System overview</h2><p class="text-sm text-muted-foreground">Live status and five-minute resource trends.</p></div>
    <Button size="sm" variant="outline" class="bg-background/60" onclick={refreshDashboard} disabled={appState.busy}><RefreshCw class={appState.busy ? 'animate-spin' : ''} /> Refresh</Button>
  </div>

  <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="Dashboard summary">
    {#snippet statIcon(Icon: typeof Activity)}
      <span class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"><Icon class="size-4" /></span>
    {/snippet}
    <Card.Root size="sm" class="bg-card/70 shadow-sm">
      <Card.Content class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1.5">
          <p class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Runtime</p>
          <p class="flex items-center gap-2 text-2xl font-semibold capitalize"><span class={`size-2.5 shrink-0 rounded-full ${statusDot(appState.runtimeStatus.status)}`}></span>{appState.runtimeStatus.status}</p>
          <p class="truncate text-sm text-muted-foreground" title={appState.runtimeStatus.detail}>{appState.runtimeStatus.detail}</p>
        </div>
        {@render statIcon(Activity)}
      </Card.Content>
    </Card.Root>
    <Card.Root size="sm" class="bg-card/70 shadow-sm">
      <Card.Content class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1.5">
          <p class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Active presets</p>
          <p class="text-2xl font-semibold tabular-nums">{appState.activePresetNames.length}{appState.modelsMax > 0 ? ` / ${appState.modelsMax}` : ''}</p>
          <p class="truncate text-sm text-muted-foreground">{appState.activePresetNames.join(', ') || 'None running'}</p>
        </div>
        {@render statIcon(Server)}
      </Card.Content>
    </Card.Root>
    <Card.Root size="sm" class="bg-card/70 shadow-sm">
      <Card.Content class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1.5">
          <p class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Local models</p>
          <p class="text-2xl font-semibold tabular-nums">{appState.localModels.length}</p>
          <Button size="sm" variant="link" class="h-auto p-0" onclick={() => (appState.activeSection = 'models')}>Manage models</Button>
        </div>
        {@render statIcon(Box)}
      </Card.Content>
    </Card.Root>
    <Card.Root size="sm" class={`bg-card/70 shadow-sm ${stale ? 'ring-warning/50' : ''}`}>
      <Card.Content class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1.5">
          <p class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Telemetry</p>
          {#if stale}
            <p class="flex items-center gap-2 text-2xl font-semibold text-warning-foreground dark:text-warning"><span class="size-2.5 shrink-0 rounded-full bg-warning"></span>Stale</p>
          {:else}
            <p class="flex items-center gap-2 text-2xl font-semibold text-success"><span class="relative flex size-2.5 shrink-0"><span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-60"></span><span class="relative inline-flex size-2.5 rounded-full bg-success"></span></span>Live</p>
          {/if}
          <p class="truncate text-sm text-muted-foreground">{capturedAt ? formatDate(capturedAt) : 'Awaiting first sample'}</p>
        </div>
        {@render statIcon(Radio)}
      </Card.Content>
    </Card.Root>
  </section>

  <div class="grid gap-3 lg:grid-cols-2">
    <Card.Root class="overflow-hidden border-primary/15 shadow-sm">
      <Card.Header>
        <Card.Title>Active presets</Card.Title>
        <Card.Description>Stop or restart a running preset. Active requests may be interrupted.</Card.Description>
        <Card.Action><Button size="sm" variant="outline" onclick={() => (appState.activeSection = 'presets')}>Manage presets</Button></Card.Action>
      </Card.Header>
      <Card.Content>
        {#if appState.activePresetNames.length}
          <Item.Group>
            {#each appState.activePresetNames as name}
              <Item.Root variant="outline" class="bg-muted/20">
                <Item.Content><Item.Title class="flex items-center gap-2"><span class="size-2 rounded-full bg-success shadow-[0_0_0_3px_color-mix(in_oklab,var(--color-success)_18%,transparent)]"></span>{name}</Item.Title><Item.Description>Running</Item.Description></Item.Content>
                <Item.Actions>
                  <AlertDialog.Root>
                    <AlertDialog.Trigger class={buttonVariants({ variant: 'outline', size: 'sm' })} disabled={appState.busy}>Restart</AlertDialog.Trigger>
                    <AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>Restart {name}?</AlertDialog.Title><AlertDialog.Description>Restarting this preset may interrupt active requests.</AlertDialog.Description></AlertDialog.Header><AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={() => app.restartPreset(name)}>Restart preset</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content>
                  </AlertDialog.Root>
                  <AlertDialog.Root>
                    <AlertDialog.Trigger class={buttonVariants({ variant: 'destructive', size: 'sm' })} disabled={appState.busy}>Stop</AlertDialog.Trigger>
                    <AlertDialog.Content><AlertDialog.Header><AlertDialog.Title>Stop {name}?</AlertDialog.Title><AlertDialog.Description>Stopping this preset may interrupt active requests.</AlertDialog.Description></AlertDialog.Header><AlertDialog.Footer><AlertDialog.Cancel>Cancel</AlertDialog.Cancel><AlertDialog.Action onclick={() => app.stopPreset(name)}>Stop preset</AlertDialog.Action></AlertDialog.Footer></AlertDialog.Content>
                  </AlertDialog.Root>
                </Item.Actions>
              </Item.Root>
            {/each}
          </Item.Group>
        {:else}
          <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-dashed p-4"><p class="text-sm text-muted-foreground">No active presets.</p><Button size="sm" onclick={() => (appState.activeSection = 'presets')}>Choose a preset</Button></div>
        {/if}
      </Card.Content>
    </Card.Root>

    <Card.Root>
      <Card.Header><Card.Title>Operational details</Card.Title><Card.Description>Processes, warnings, and most recent action.</Card.Description></Card.Header>
      <Card.Content class="space-y-4">
        {#if appState.signalsLastError}<p class="rounded-md border border-warning/40 bg-warning/10 p-3 text-sm text-warning-foreground dark:text-warning">Telemetry refresh failed: {appState.signalsLastError}</p>{/if}
        {#each appState.signals?.warnings || [] as warning}<p class="rounded-md border border-warning/30 p-3 text-sm text-warning-foreground dark:text-warning">{warning}</p>{/each}
        {#if (appState.signals?.runtime || []).length}
          <Item.Group>{#each appState.signals?.runtime || [] as proc}<Item.Root variant="outline"><Item.Content><Item.Title>{String(proc.name || proc.pid)}</Item.Title><Item.Description>pid {proc.pid} · RSS {formatBytes(proc.rss_bytes)} · CPU {(proc.cpu_percent || 0).toFixed(1)}%</Item.Description></Item.Content></Item.Root>{/each}</Item.Group>
        {:else}<p class="text-sm text-muted-foreground">No runtime processes reported.</p>{/if}
        {#if appState.lastOperation}
          {@const op = appState.lastOperation}
          <div class={`rounded-lg border p-3 ${operationStatusClass(op.status)}`}><div class="flex items-start justify-between gap-3"><div class="space-y-1"><Badge variant="outline" class={operationStatusClass(op.status)}>{op.status || 'succeeded'}</Badge><p class="font-medium text-foreground">{[op.action, op.target].filter(Boolean).join(' · ') || 'Last operation'}</p>{#if op.message}<p class="text-sm text-muted-foreground">{op.message}</p>{/if}</div>{#if op.duration_ms != null}<span class="font-mono text-sm text-muted-foreground">{(op.duration_ms / 1000).toFixed(2)}s</span>{/if}</div></div>
        {/if}
      </Card.Content>
    </Card.Root>
  </div>

  <section class="space-y-3" aria-labelledby="resources-title">
    <div><p class="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">Telemetry</p><h2 id="resources-title" class="text-lg font-semibold">Resources</h2><p class="text-sm text-muted-foreground">Current machine load.</p></div>
    <div class="grid gap-3 md:grid-cols-2">
      <Card.Root>
        <Card.Header><Card.Title>System</Card.Title><Card.Description>CPU and memory</Card.Description></Card.Header>
        <Card.Content class="space-y-4">
          <ResourceMeter label="CPU" percent={appState.signals?.cpu?.used_percent} detail={`${appState.signals?.cpu?.logical_cores || '-'} cores`} />
          <ResourceMeter label="Memory" percent={memoryPercent} detail={`${formatBytes(memory?.available_bytes)} free`} />
        </Card.Content>
      </Card.Root>
      {#each appState.signals?.gpu || [] as gpu, index}
        <Card.Root>
          <Card.Header><Card.Title class="truncate" title={gpu.name}>{gpu.name || `GPU ${index + 1}`}</Card.Title><Card.Description>{gpu.backend || 'GPU'}</Card.Description></Card.Header>
          <Card.Content class="space-y-4">
            <ResourceMeter label="Utilisation" percent={gpu.utilization_percent} />
            <ResourceMeter label="VRAM" percent={percent(gpu.used_vram_bytes, gpu.total_vram_bytes)} detail={`${formatBytes(gpu.used_vram_bytes)} / ${formatBytes(gpu.total_vram_bytes)}`} />
            <div class="flex items-baseline justify-between text-sm"><span class="font-medium">Temperature</span><span class="tabular-nums text-muted-foreground">{gpu.temperature_celsius == null ? 'Unavailable' : `${gpu.temperature_celsius.toFixed(0)}°C`}</span></div>
          </Card.Content>
        </Card.Root>
      {/each}
    </div>
    {#if !(appState.signals?.gpu || []).length}<p class="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">GPU telemetry unavailable.</p>{/if}
  </section>

  <Card.Root>
    <Card.Header><Card.Title>Live trends</Card.Title><Card.Description>Browser-local rolling window; history resets when the page reloads.</Card.Description></Card.Header>
    <Card.Content class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      <TrendChart label="CPU" points={appState.runtimeHistory.map((sample) => ({ capturedAt: sample.capturedAt, value: sample.cpu }))} />
      <TrendChart label="Memory" points={appState.runtimeHistory.map((sample) => ({ capturedAt: sample.capturedAt, value: sample.memory }))} />
      {#each appState.signals?.gpu || [] as gpu, index}
        {@const key = gpuKey(gpu.backend, gpu.name, index)}
        <TrendChart label={`${gpu.name || `GPU ${index + 1}`} utilisation`} points={appState.runtimeHistory.map((sample) => ({ capturedAt: sample.capturedAt, value: sample.gpu.find((item) => item.key === key)?.utilization ?? null }))} />
        <TrendChart label={`${gpu.name || `GPU ${index + 1}`} VRAM`} points={appState.runtimeHistory.map((sample) => ({ capturedAt: sample.capturedAt, value: sample.gpu.find((item) => item.key === key)?.vram ?? null }))} />
        <TrendChart label={`${gpu.name || `GPU ${index + 1}`} temperature`} unit="°C" points={appState.runtimeHistory.map((sample) => ({ capturedAt: sample.capturedAt, value: sample.gpu.find((item) => item.key === key)?.temperature ?? null }))} />
      {/each}
    </Card.Content>
  </Card.Root>

</div>
