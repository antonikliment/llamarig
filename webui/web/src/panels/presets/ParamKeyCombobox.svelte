<script lang="ts">
  import { Combobox } from 'bits-ui';
  import Check from '@lucide/svelte/icons/check';
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down';
  import type { LlamaServerParam } from '../../lib/data/llamaServerParams';

  let {
    value = $bindable(''),
    params,
    disabled = false,
    oninput
  }: {
    value: string;
    params: LlamaServerParam[];
    disabled?: boolean;
    oninput?: () => void;
  } = $props();

  let inputValue = $state(value);

  $effect(() => {
    inputValue = value;
  });

  const filtered = $derived(
    inputValue.trim()
      ? params.filter(
          (p) =>
            p.key.includes(inputValue.toLowerCase()) ||
            p.aliases?.some((a) => a.includes(inputValue.toLowerCase()))
        )
      : params
  );

  function onValueChange(v: string) {
    value = v;
    inputValue = v;
    oninput?.();
  }

  function onInputChange(e: Event) {
    inputValue = (e.target as HTMLInputElement).value;
    value = inputValue;
    oninput?.();
  }
</script>

<Combobox.Root
  type="single"
  value={value}
  inputValue={inputValue}
  onValueChange={onValueChange}
  {disabled}
>
  <div class="relative w-40">
    <Combobox.Input
      class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 pr-8 font-mono text-sm shadow-xs transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
      placeholder="key"
      oninput={onInputChange}
      aria-label="Parameter key"
    />
    <Combobox.Trigger class="absolute inset-y-0 right-0 flex items-center pr-2 text-muted-foreground">
      <ChevronsUpDown class="size-3.5" />
    </Combobox.Trigger>
  </div>
  <Combobox.Portal>
    <Combobox.Content class="z-50 min-w-64 max-w-80 rounded-md border bg-popover p-1 shadow-md">
      <Combobox.Viewport class="max-h-60 overflow-y-auto">
        {#each filtered as param (param.key)}
          <Combobox.Item
            value={param.key}
            label={param.key}
            class="flex cursor-default select-none items-start gap-2 rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
          >
            {#snippet children({ selected })}
              <Check class="mt-0.5 size-4 shrink-0 {selected ? 'opacity-100' : 'opacity-0'}" />
              <div class="min-w-0">
                <p class="font-mono leading-snug">{param.key}</p>
                <p class="text-xs text-muted-foreground leading-snug truncate">{param.description}</p>
              </div>
            {/snippet}
          </Combobox.Item>
        {:else}
          <p class="px-2 py-1.5 text-sm text-muted-foreground">No matching params.</p>
        {/each}
      </Combobox.Viewport>
    </Combobox.Content>
  </Combobox.Portal>
</Combobox.Root>
