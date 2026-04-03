/**
 * Application-wide constants.
 */

export const APP_NAME = "RemnaCore";

export const PAGINATION_DEFAULTS = {
  limit: 50,
  maxLimit: 200,
} as const;

export const TOKEN_STORAGE_KEY = "remnacore-auth" as const;

export const THEME_STORAGE_KEY = "remnacore-theme" as const;

// ─── TanStack Query stale times ──────────────────────────────────────────────

/** 5 minutes — user profile changes infrequently within a session */
export const STALE_TIME_USER_MS = 5 * 60 * 1000;

/** 10 minutes — plans change rarely */
export const STALE_TIME_PLANS_MS = 10 * 60 * 1000;

/** 2 minutes — reasonable default for most queries */
export const STALE_TIME_DEFAULT_MS = 2 * 60 * 1000;

// ─── Numeric unit constants ──────────────────────────────────────────────────

/** Bytes per kilobyte (binary, IEC) */
export const BYTES_PER_KB = 1024;

/** Minor currency units per major unit (e.g., cents per dollar) */
export const CENTS_PER_UNIT = 100;
