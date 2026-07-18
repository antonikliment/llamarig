import { describe, expect, it, vi } from 'vitest';
import { apiUrl, createApiClient } from './api';

describe('apiUrl', () => {
  it('uses same-origin paths for empty base', () => {
    expect(apiUrl('/api/info', '')).toBe('/api/info');
  });

  it('normalizes hosts without protocol', () => {
    expect(apiUrl('/api/info', '127.0.0.1:7000', 'http:')).toBe('http://127.0.0.1:7000/api/info');
  });
});

describe('createApiClient', () => {
  it('adds bearer token only when present', async () => {
    let captured: RequestInit | undefined;
    const fetcher: typeof fetch = async (_input, init) => {
      captured = init;
      return new Response('{}');
    };
    const api = createApiClient(() => ({ apiBase: '', token: ' secret ' }), fetcher as unknown as typeof fetch);

    await api.getInfo();

    const headers = captured?.headers as Headers;
    expect(headers.get('Authorization')).toBe('Bearer secret');
  });

  it('includes server error kind and message', async () => {
    const fetcher = vi.fn(async () => new Response(JSON.stringify({ error: { kind: 'bad', message: 'nope' } }), { status: 400 }));
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await expect(api.getInfo()).rejects.toThrow('bad: nope');
  });

  it('builds catalog query URLs', async () => {
    let captured = '';
    const fetcher: typeof fetch = async (input) => {
      captured = String(input);
      return new Response('{}');
    };
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await api.listModelCatalog({ limit: 25, sort: 'trending', search: 'qwen coder', min_fit: 'marginal' });

    expect(captured).toBe('/api/models/catalog?limit=25&sort=trending&search=qwen+coder&min_fit=marginal');
  });

  it('calls signals endpoint', async () => {
    let captured = '';
    const fetcher: typeof fetch = async (input) => {
      captured = String(input);
      return new Response('{}');
    };
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await api.getSignals();

    expect(captured).toBe('/api/signals');
  });

  it('builds bounded log and archive requests', async () => {
    const calls: Array<{ url: string; method?: string }> = [];
    const fetcher: typeof fetch = async (input, init) => {
      calls.push({ url: String(input), method: init?.method });
      return new Response('{}');
    };
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await api.getLogs('gateway', 2000);
    await api.getLogArchive('gateway-archive.log', 200);
    await api.clearLogArchives();

    expect(calls).toEqual([
      { url: '/api/logs?source=gateway&lines=2000', method: undefined },
      { url: '/api/logs/archives/gateway-archive.log?lines=200', method: undefined },
      { url: '/api/logs/archives', method: 'DELETE' }
    ]);
  });
});
