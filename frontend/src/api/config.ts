import { createCalculatorApi, type CalculatorApi } from './client';

/**
 * The configured client.
 *
 * ⚠️ The client secret ships inside the JavaScript bundle and is therefore
 * public — any user can read it. That is inherent to a browser SPA, not an
 * oversight. The token gives the server per-client attribution and keeps
 * non-browser consumers out; the real defence for this surface is the server's
 * rate limiting, CORS allowlist and payload caps.
 *
 * The default base URL is same-origin `/v1`, proxied to the API by Vite in
 * development and by nginx in the container. Nothing cross-origin happens, so
 * CORS never enters the picture.
 */
export const api: CalculatorApi = createCalculatorApi({
  baseUrl: import.meta.env.VITE_API_URL ?? '/v1',
  clientId: import.meta.env.VITE_API_CLIENT_ID ?? 'web',
  clientSecret: import.meta.env.VITE_API_CLIENT_SECRET ?? 'dev-secret',
});
