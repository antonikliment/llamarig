<script lang="ts">
  import AppShell from '../components/shell/AppShell.svelte';
  import type { LlamaRigState } from '../lib/state.svelte';

  let { app }: { app: LlamaRigState } = $props();
</script>

<AppShell
  {app}
  sections={[{ id: 'runtime', label: 'Dashboard' }, { id: 'presets', label: 'Presets' }]}
  sectionTitle="Dashboard"
  errorMessage=""
  onSaveApiBase={() => {}}
  onSaveToken={() => {}}
  onTestConnection={() => {}}
>
  <p>Dashboard content</p>
</AppShell>
