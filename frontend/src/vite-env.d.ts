/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Base URL of the calculation API. Same-origin `/v1` by default. */
  readonly VITE_API_URL?: string;
  readonly VITE_API_CLIENT_ID?: string;
  /** Public by necessity — see src/api/config.ts. */
  readonly VITE_API_CLIENT_SECRET?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
