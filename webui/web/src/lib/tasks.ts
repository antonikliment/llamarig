export function isTerminalDownloadState(state: string | undefined) {
  return state === 'completed' || state === 'failed' || state === 'cancelled' || state === 'already_downloaded';
}

export function canApplyDownload(state: string | undefined) {
  return state === 'completed' || state === 'already_downloaded';
}

export function choosePresetSelection(
  presets: { name: string }[],
  preferred: string,
  activeNames: string[],
  defaultPreset: string,
  selectedName: string
) {
  const names = presets.map((preset) => preset.name);
  if (preferred && names.includes(preferred)) return preferred;
  const active = activeNames.find((name) => names.includes(name));
  if (active) return active;
  if (defaultPreset && names.includes(defaultPreset)) return defaultPreset;
  if (selectedName && names.includes(selectedName)) return selectedName;
  return names[0] || '';
}
