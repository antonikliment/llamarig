import { describe, expect, it, vi } from 'vitest';
import { apiUrl, createApiClient } from './api';

const connectResponse = (body = '{}', status = 200) => new Response(body, {
  status,
  headers: { 'Content-Type': 'application/json' }
});

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
      return connectResponse();
    };
    const api = createApiClient(() => ({ apiBase: '', token: ' secret ' }), fetcher as unknown as typeof fetch);

    await api.getInfo();

    const headers = captured?.headers as Headers;
    expect(headers.get('Authorization')).toBe('Bearer secret');
  });

  it('includes Connect error code and message', async () => {
    const fetcher = vi.fn(async () => connectResponse(JSON.stringify({ code: 'invalid_argument', message: 'nope' }), 400));
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await expect(api.getInfo()).rejects.toThrow('[invalid_argument] nope');
  });

  it('sends typed catalog query fields', async () => {
    let captured = '';
    let body = '';
    const fetcher: typeof fetch = async (input, init) => {
      captured = String(input);
      body = await new Request(new URL(captured, 'http://localhost'), init).text();
      return connectResponse();
    };
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await api.listModelCatalog({ limit: 25, sort: 'trending', search: 'qwen coder', min_fit: 'marginal' });

    expect(captured).toBe('/llamarig.control.v1.ControlService/ListModelCatalog');
    expect(JSON.parse(body)).toEqual({ limit: 25, sort: 'trending', search: 'qwen coder', minFit: 'marginal' });
  });

  it('calls signals endpoint', async () => {
    let captured = '';
    const fetcher: typeof fetch = async (input) => {
      captured = String(input);
      return connectResponse();
    };
    const api = createApiClient(() => ({ apiBase: '', token: '' }), fetcher as unknown as typeof fetch);

    await api.getSignals();

    expect(captured).toBe('/llamarig.control.v1.ControlService/GetSignals');
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
