# Primary theme overrides — implementation notes

Status: implemented and verified on 2026-07-11 (`pnpm run verify:web`, 45 tests, light/dark Dashboard captures inspected).

## Intent

Give light and dark mode independent primary colors while keeping LlamaRig's semantic shadcn-svelte theme. The chosen primary should influence the whole visual theme, not only primary buttons. Users can override both colors from Settings and restore defaults.

## Implementation map

- `webui/web/src/lib/theme.ts` owns defaults, local-storage keys, validation, persistence, CSS-variable application, and automatic black/white primary foreground selection.
- `webui/web/src/main.ts` applies stored colors before mounting Svelte to avoid a color flash.
- `webui/web/src/styles/app.css` derives background tint, cards, secondary/muted/accent surfaces, borders, focus rings, chart color, and sidebar states from `--active-primary`. Light and dark mode select separate user variables.
- `webui/web/src/components/shell/AppShell.svelte` exposes two native color pickers through the existing shadcn Input and a reset action in Settings.
- Storage keys are `llamarig.theme.primary.light` and `llamarig.theme.primary.dark`; values are six-digit hex colors.
- Defaults are light `#08766d` and dark `#6dd8c8`.

## Dashboard layout included in the same change

- Summary cards remain 50px high.
- Active Presets and Resources remain side-by-side on desktop.
- CPU, Memory, and each GPU now stack vertically inside the single Resources panel.
- Live Trends remains full width.

## Verification checklist

- Run `pnpm run verify:web` from `webui/`.
- Capture `pnpm run screenshots -- --out <dir> --section dashboard` and inspect light/dark output.
- In Settings, change each color, switch modes, reload, and confirm persistence; Reset must restore defaults.
- Check focus rings, selected sidebar item, links/buttons, charts, and text contrast in both modes.
