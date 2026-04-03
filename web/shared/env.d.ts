/**
 * Ambient type declarations for Vite environment variables.
 * Shared package is consumed by Vite apps, so import.meta.env is available
 * at runtime.
 */
interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
