import { describe, expect, it, vi } from 'vitest';
import { CalculatorApiError, CalculatorNetworkError, createCalculatorApi } from './client';

type Call = { url: string; init: RequestInit };

/** A fetch stub that records calls and replies from a queue of handlers. */
function stubFetch(handlers: ((call: Call) => Response)[]) {
  const calls: Call[] = [];
  let index = 0;

  const fetchImpl = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const call = { url: String(input), init: init ?? {} };
    calls.push(call);

    const handler = handlers[Math.min(index, handlers.length - 1)];
    index += 1;
    return handler(call);
  });

  return { fetchImpl: fetchImpl as unknown as typeof globalThis.fetch, calls };
}

const json = (body: unknown, init?: ResponseInit) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });

const tokenBody = (token = 'token-1', expiresIn = 900) => ({
  token,
  tokenType: 'Bearer',
  expiresIn,
  expiresAt: new Date().toISOString(),
});

const resultBody = { result: 14, formatted: '14', expression: '2 + 3 * 4' };

function newApi(handlers: ((call: Call) => Response)[], now = () => 1_000_000) {
  const { fetchImpl, calls } = stubFetch(handlers);
  const api = createCalculatorApi({
    baseUrl: '/v1',
    clientId: 'web',
    clientSecret: 'dev-secret',
    fetch: fetchImpl,
    now,
  });
  return { api, calls };
}

describe('token handling', () => {
  it('mints a token, then sends it as a bearer credential', async () => {
    const { api, calls } = newApi([
      (call) => (call.url.endsWith('/auth/token') ? json(tokenBody()) : json(resultBody)),
    ]);

    const result = await api.calculate({ expression: '2 + 3 * 4' });
    expect(result.formatted).toBe('14');

    expect(calls).toHaveLength(2);
    expect(calls[0].url).toBe('/v1/auth/token');
    expect(JSON.parse(String(calls[0].init.body))).toEqual({
      clientId: 'web',
      clientSecret: 'dev-secret',
    });

    expect(calls[1].url).toBe('/v1/calculate');
    const headers = calls[1].init.headers as Record<string, string>;
    expect(headers.Authorization).toBe('Bearer token-1');
  });

  it('reuses the cached token across calls', async () => {
    const { api, calls } = newApi([
      (call) => (call.url.endsWith('/auth/token') ? json(tokenBody()) : json(resultBody)),
    ]);

    await api.calculate({ expression: '1+1' });
    await api.calculate({ expression: '2+2' });

    const tokenRequests = calls.filter((call) => call.url.endsWith('/auth/token'));
    expect(tokenRequests).toHaveLength(1);
  });

  it('re-mints once the cached token is close to expiry', async () => {
    let clock = 1_000_000;
    const { api, calls } = newApi(
      [(call) => (call.url.endsWith('/auth/token') ? json(tokenBody()) : json(resultBody))],
      () => clock,
    );

    await api.calculate({ expression: '1+1' });
    // 900s TTL minus the 30s safety margin.
    clock += 871_000;
    await api.calculate({ expression: '2+2' });

    expect(calls.filter((call) => call.url.endsWith('/auth/token'))).toHaveLength(2);
  });

  it('shares one token request between concurrent callers', async () => {
    const { api, calls } = newApi([
      (call) => (call.url.endsWith('/auth/token') ? json(tokenBody()) : json(resultBody)),
    ]);

    await Promise.all([
      api.calculate({ expression: '1+1' }),
      api.calculate({ expression: '2+2' }),
      api.calculate({ expression: '3+3' }),
    ]);

    expect(calls.filter((call) => call.url.endsWith('/auth/token'))).toHaveLength(1);
  });

  // The development server mints a new signing secret on every restart, so a
  // cached token going stale is routine rather than exceptional.
  it('retries once with a fresh token after a 401', async () => {
    let issued = 0;
    let calculated = 0;

    const { api, calls } = newApi([
      (call) => {
        if (call.url.endsWith('/auth/token')) {
          issued += 1;
          return json(tokenBody(`token-${issued}`));
        }
        calculated += 1;
        if (calculated === 1) {
          return json({ error: { code: 'UNAUTHORIZED', message: 'nope' } }, { status: 401 });
        }
        return json(resultBody);
      },
    ]);

    const result = await api.calculate({ expression: '2 + 3 * 4' });
    expect(result.formatted).toBe('14');
    expect(issued).toBe(2);

    const retried = calls.at(-1)?.init.headers as Record<string, string>;
    expect(retried.Authorization).toBe('Bearer token-2');
  });

  it('gives up after one retry', async () => {
    const { api, calls } = newApi([
      (call) =>
        call.url.endsWith('/auth/token')
          ? json(tokenBody())
          : json({ error: { code: 'UNAUTHORIZED', message: 'nope' } }, { status: 401 }),
    ]);

    await expect(api.calculate({ expression: '1+1' })).rejects.toBeInstanceOf(CalculatorApiError);
    // Two token requests, two calculate attempts — not an infinite loop.
    expect(calls).toHaveLength(4);
  });
});

describe('error mapping', () => {
  it('turns the error envelope into a typed failure, keeping the position', async () => {
    const { api } = newApi([
      (call) =>
        call.url.endsWith('/auth/token')
          ? json(tokenBody())
          : json(
              {
                error: { code: 'DIVISION_BY_ZERO', message: "Can't divide by zero", position: 7 },
                requestId: 'req-1',
              },
              { status: 422 },
            ),
    ]);

    await expect(api.calculate({ expression: '100 + 1/0' })).rejects.toMatchObject({
      code: 'DIVISION_BY_ZERO',
      message: "Can't divide by zero",
      status: 422,
      position: 7,
      requestId: 'req-1',
    });
  });

  it('folds Retry-After into the message on a 429', async () => {
    const { api } = newApi([
      (call) =>
        call.url.endsWith('/auth/token')
          ? json(tokenBody())
          : json(
              { error: { code: 'RATE_LIMITED', message: 'Too many requests' } },
              { status: 429, headers: { 'Retry-After': '3' } },
            ),
    ]);

    await expect(api.calculate({ expression: '1+1' })).rejects.toThrow(/Try again in 3s/);
  });

  it('handles a failure that is not our envelope', async () => {
    const { api } = newApi([
      (call) =>
        call.url.endsWith('/auth/token')
          ? json(tokenBody())
          : new Response('<html>502</html>', { status: 502 }),
    ]);

    await expect(api.calculate({ expression: '1+1' })).rejects.toMatchObject({
      code: 'UNEXPECTED_RESPONSE',
      status: 502,
    });
  });

  it('reports an unreachable service', async () => {
    const fetchImpl = vi.fn(async () => {
      throw new TypeError('Failed to fetch');
    }) as unknown as typeof globalThis.fetch;

    const api = createCalculatorApi({
      baseUrl: '/v1',
      clientId: 'web',
      clientSecret: 'dev-secret',
      fetch: fetchImpl,
    });

    await expect(api.calculate({ expression: '1+1' })).rejects.toBeInstanceOf(
      CalculatorNetworkError,
    );
    await expect(api.calculate({ expression: '1+1' })).rejects.toThrow(/Can't reach/);
  });

  it('distinguishes a timeout from an unreachable service', async () => {
    const fetchImpl = vi.fn(async () => {
      throw new DOMException('The operation timed out', 'TimeoutError');
    }) as unknown as typeof globalThis.fetch;

    const api = createCalculatorApi({
      baseUrl: '/v1',
      clientId: 'web',
      clientSecret: 'dev-secret',
      fetch: fetchImpl,
    });

    await expect(api.calculate({ expression: '1+1' })).rejects.toThrow(/took too long/);
  });
});

describe('request shape', () => {
  it('passes the precision through', async () => {
    const { api, calls } = newApi([
      (call) => (call.url.endsWith('/auth/token') ? json(tokenBody()) : json(resultBody)),
    ]);

    await api.calculate({ expression: '1/3', precision: 4 });

    expect(JSON.parse(String(calls[1].init.body))).toEqual({
      expression: '1/3',
      precision: 4,
    });
  });
});
