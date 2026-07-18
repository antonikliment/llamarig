import { spawn, spawnSync } from 'node:child_process';
import { mkdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const webuiDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.resolve(webuiDir, argValue('--out') || 'screenshots');
const baseUrl = 'http://127.0.0.1:5180';
const requestedSection = argValue('--section').toLowerCase();
const sections = [
  { label: 'Dashboard', file: 'runtime' },
  { label: 'Presets', file: 'presets' },
  { label: 'Models', file: 'models' },
  { label: 'Logs', file: 'logs' }
].filter((section) => !requestedSection || section.label.toLowerCase() === requestedSection);
const modes = ['light', 'dark'];
const now = new Date().toISOString();
const presets = [
  { name: 'default', entries: [{ key: 'model', value: '/models/qwen2.5-coder-7b/qwen.gguf' }, { key: 'ctx-size', value: '8192' }] },
  { name: 'llama3-8b', entries: [{ key: 'model', value: '/models/llama-3-8b/llama.gguf' }] }
];

await mkdir(outDir, { recursive: true });
const server = spawn('pnpm', ['exec', 'vite', '--port', '5180', '--host', '127.0.0.1'], { cwd: webuiDir, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
let browser;

try {
  await waitForServer(server);
  browser = await chromium.launch();
  const files = [];

  for (const mode of modes) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 });
    await context.route('**/api/**', mockApi);
    await context.addInitScript((value) => localStorage.setItem('mode-watcher-mode', value), mode);
    const page = await context.newPage();
    page.on('pageerror', (err) => console.error(`[pageerror ${mode}] ${err.message}`));
    page.on('console', (msg) => msg.type() === 'error' && console.error(`[console ${mode}] ${msg.text()}`));
    await page.goto(baseUrl, { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle');

    for (const section of sections) {
      await page.getByRole('button', { name: section.label, exact: true }).click();
      await page.waitForLoadState('networkidle');
      await page.locator('main .rounded-xl, main [data-slot="card"]').first().waitFor({ timeout: 5000 }).catch(() => console.error(`[no-content ${mode}] ${section}`));
      // Runtime sparklines need a few signal poll samples (app polls every 5s).
      if (section.label === 'Dashboard') await page.waitForTimeout(11_000);
      const file = path.join(outDir, `${section.file}-${mode}.png`);
      await page.screenshot({ path: file, fullPage: true });
      files.push(file);
    }
    await context.close();
  }

  await browser.close();
  browser = null;
  for (const file of files) {
    const size = (await stat(file)).size;
    console.log(`${path.relative(webuiDir, file)} ${size} bytes`);
  }
} finally {
  if (browser) await browser.close().catch(() => undefined);
  if (!server.killed && server.pid) {
    try {
      if (process.platform === 'win32') {
        spawnSync('taskkill', ['/pid', server.pid.toString(), '/T', '/F'], { stdio: 'ignore' });
      } else {
        process.kill(-server.pid, 'SIGTERM');
      }
    } catch (err) {
      if (err.code !== 'ESRCH') throw err;
    }
  }
}

function argValue(name) {
  const i = process.argv.indexOf(name);
  return i >= 0 ? process.argv[i + 1] : '';
}

function waitForServer(proc) {
  return new Promise((resolve, reject) => {
    let done = false;
    let log = '';
    const timeout = setTimeout(() => finish(reject, new Error('Vite server timed out')), 30_000);
    const finish = (fn, value) => {
      if (done) return;
      done = true;
      clearTimeout(timeout);
      fn(value);
    };
    proc.on('error', (err) => finish(reject, err));
    proc.on('exit', (code) => finish(reject, new Error(`Vite exited early with code ${code}\n${log}`)));
    const onData = (chunk) => {
      log = (log + chunk).slice(-4000);
    };
    proc.stdout.on('data', onData);
    proc.stderr.on('data', onData);
    const poll = async () => {
      try {
        const response = await fetch(baseUrl);
        if (response.ok) return finish(resolve);
      } catch {
        // keep polling
      }
      if (!done) setTimeout(poll, 250);
    };
    poll();
  });
}

async function mockApi(route) {
  const url = new URL(route.request().url());
  const body = fixture(url.pathname);
  if (typeof body === 'string') {
    await route.fulfill({ status: 200, contentType: 'text/plain; charset=utf-8', body });
    return;
  }
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
}

function fixture(pathname) {
  if (pathname === '/api/info') return { router: { status: 'running', detail: 'llama-server ready', checked_at: now }, default_preset: 'default' };
  if (pathname === '/api/runtime/status') return { state: 'running', detail: '1 preset active', checked_at: now, presets: [{ name: 'default', state: 'running', ready: true }] };
  if (pathname === '/api/signals') {
    const jitter = (base, spread) => Math.max(0, Math.min(100, base + (Math.random() - 0.5) * spread));
    const vram = 12000000000 + Math.random() * 4000000000;
    return { signals: { captured_at: new Date().toISOString(), cpu: { logical_cores: 16, used_percent: jitter(42, 30) }, memory: { total_bytes: 34359738368, available_bytes: 18000000000, used_percent: jitter(48, 20) }, gpu: [{ name: 'NVIDIA RTX 4090', backend: 'CUDA', total_vram_bytes: 25769803776, used_vram_bytes: vram, utilization_percent: jitter(67, 40), temperature_celsius: jitter(64, 8) }], runtime: [{ name: 'router', pid: 12345, rss_bytes: 8200000000, cpu_percent: 23.4 }], warnings: [] } };
  }
  if (pathname === '/api/events') return { events: events() };
  if (pathname === '/api/presets') return { ok: true, path: '/home/demo/.llamarig/models.ini', models_max: 2, presets };
  if (pathname === '/api/models/local') return { models: localModels() };
  if (pathname === '/api/models/catalog') return catalog();
  if (pathname === '/api/models/catalog/events') return {};

  const match = pathname.match(/^\/api\/presets\/([^/]+)$/);
	if (match) {
		const name = decodeURIComponent(match[1]);
		return { ok: true, preset: presets.find((preset) => preset.name === name) };
  }
  return {};
}

function events() {
  return ['runtime.start', 'models.download', 'preset.save', 'models.resolve', 'runtime.restart', 'models.download', 'preset.delete', 'runtime.stop'].map((action, i) => ({
    time: new Date(Date.now() - i * 7 * 60_000).toISOString(),
    action,
    success: i !== 3 && i !== 5,
    error_kind: i === 3 ? 'timeout' : i === 5 ? 'network' : undefined,
    duration: ['1.2s', '34s', '220ms', '10s', '3.1s', '45s', '90ms', '800ms'][i]
  }));
}

function localModels() {
  return [
    { path: '/models/qwen2.5-coder-7b/qwen.gguf', filename: 'qwen.gguf', size_bytes: 4680000000, modified_at: now, used_by_presets: ['default'] },
    { path: '/models/llama-3-8b/llama.gguf', filename: 'llama.gguf', size_bytes: 5100000000, modified_at: now, used_by_presets: ['llama3-8b'] },
    { path: '/models/mistral-7b/mistral.gguf', filename: 'mistral.gguf', size_bytes: 4300000000, modified_at: now, used_by_presets: [] },
    { path: '/models/embed/nomic.gguf', filename: 'nomic.gguf', size_bytes: 740000000, modified_at: now }
  ];
}

function catalog() {
  const names = ['Qwen2.5 Coder 7B', 'Llama 3 8B Instruct', 'Mistral 7B Instruct', 'Gemma 2 9B', 'DeepSeek Coder 6.7B'];
  return { ok: true, machine: { total_ram_bytes: 34359738368, available_ram_bytes: 18000000000, gpu_name: 'NVIDIA RTX 4090', vram_bytes: 25769803776, has_gpu: true }, cache: { hit: true, stale: false, updated_at: now, ttl_seconds: 3600 }, models: names.map((repo, i) => ({ id: `model-${i}`, owner: i % 2 ? 'meta-llama' : 'Qwen', repo, url: `https://huggingface.co/${repo.replaceAll(' ', '-')}`, downloads: 90000 - i * 8000, likes: 1200 - i * 90, tags: ['gguf', 'text-generation'], license: i % 2 ? 'llama3' : 'apache-2.0', best_file: { filename: `${repo.toLowerCase().replaceAll(' ', '-')}.Q4_K_M.gguf`, size_bytes: 4200000000 + i * 400000000, quant: 'Q4_K_M', fit_level: i === 3 ? 'marginal' : 'fits', exists: false }, fit: { level: i === 3 ? 'marginal' : 'fits', reason: i === 3 ? 'Close to VRAM headroom' : 'Fits comfortably in VRAM' } })) };
}
