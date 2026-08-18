import type {
  ApiErrorResponse,
  CalculateRequest,
  CalculateResponse,
  TokenResponse,
} from './types';

/**
 * Client for the calculation API.
 *
 * It owns the token lifecycle: fetch on first use, cache until shortly before
 * expiry, and retry once on a 401 so a server restart (which mints a new signing
 * secret in development) recovers without the user noticing.
 */

/** A failure the server described. */
export class CalculatorApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly position?: number;
  readonly requestId?: string;

  constructor(
    code: string,
    message: string,
    status: number,
    position?: number,
    requestId?: string,
  ) {
    super(message);
    this.name = 'CalculatorApiError';
    this.code = code;
    this.status = status;
    this.position = position;
    this.requestId = requestId;
  }
}

/** The request never reached the server, or never came back. */
export class CalculatorNetworkError extends Error {
  readonly code = 'NETWORK_ERROR';

  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options);
    this.name = 'CalculatorNetworkError';
  }
}

export type CalculatorApiConfig = {
  baseUrl: string;
  clientId: string;
  clientSecret: string;
  /** Abort a request that has not answered in this long. */
  timeoutMs?: number;
  /** Injectable for tests. */
  fetch?: typeof globalThis.fetch;
  now?: () => number;
};

export type CalculatorApi = {
  calculate(request: CalculateRequest): Promise<CalculateResponse>;
};

/** Refresh this far ahead of expiry, so a token never dies mid-flight. */
const EXPIRY_MARGIN_MS = 30_000;

const DEFAULT_TIMEOUT_MS = 10_000;

export function createCalculatorApi(config: CalculatorApiConfig): CalculatorApi {
  const {
    baseUrl,
    clientId,
    clientSecret,
    timeoutMs = DEFAULT_TIMEOUT_MS,
    fetch: fetchImpl = globalThis.fetch.bind(globalThis),
    now = Date.now,
  } = config;

  let cached: { value: string; expiresAt: number } | null = null;
  // Concurrent callers share one token request rather than racing to mint two.
  let pending: Promise<string> | null = null;

  async function send(path: string, init: RequestInit): Promise<Response> {
    try {
      return await fetchImpl(`${baseUrl}${path}`, {
        ...init,
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (cause) {
      const timedOut = cause instanceof DOMException && cause.name === 'TimeoutError';
      throw new CalculatorNetworkError(
        timedOut ? 'The calculator service took too long to answer' : "Can't reach the calculator service",
        { cause },
      );
    }
  }

  /** Reads the error envelope; falls back to the status when the body is not ours. */
  async function toError(response: Response): Promise<CalculatorApiError> {
    const payload = (await response.json().catch(() => null)) as ApiErrorResponse | null;

    if (payload?.error?.code) {
      let { message } = payload.error;
      if (response.status === 429) {
        const retryAfter = response.headers.get('Retry-After');
        if (retryAfter) message += `. Try again in ${retryAfter}s`;
      }
      return new CalculatorApiError(
        payload.error.code,
        message,
        response.status,
        payload.error.position,
        payload.requestId,
      );
    }

    return new CalculatorApiError(
      'UNEXPECTED_RESPONSE',
      `The calculator service answered with ${response.status}`,
      response.status,
    );
  }

  async function mintToken(): Promise<string> {
    const response = await send('/auth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ clientId, clientSecret }),
    });

    if (!response.ok) throw await toError(response);

    const payload = (await response.json()) as TokenResponse;
    cached = {
      value: payload.token,
      expiresAt: now() + payload.expiresIn * 1000 - EXPIRY_MARGIN_MS,
    };
    return payload.token;
  }

  async function token(forceRefresh = false): Promise<string> {
    if (forceRefresh) cached = null;
    if (cached && cached.expiresAt > now()) return cached.value;

    pending ??= mintToken().finally(() => {
      pending = null;
    });
    return pending;
  }

  async function calculate(
    request: CalculateRequest,
    forceRefresh = false,
  ): Promise<CalculateResponse> {
    const bearer = await token(forceRefresh);

    const response = await send('/calculate', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${bearer}`,
      },
      body: JSON.stringify(request),
    });

    // One retry with a fresh token: the cached one may have been signed by a
    // previous instance of the server.
    if (response.status === 401 && !forceRefresh) {
      return calculate(request, true);
    }

    if (!response.ok) throw await toError(response);

    return (await response.json()) as CalculateResponse;
  }

  return {
    calculate: (request) => calculate(request),
  };
}
