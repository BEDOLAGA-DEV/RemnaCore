import type { SetupStatusResponse } from "@remnacore/shared";
import { apiGet, ENDPOINTS } from "@remnacore/shared";

// The status only flips once (when the first admin is created), so the result
// is cached for the session to avoid a network call on every navigation.
let cache: Promise<boolean> | null = null;

/** Returns true while the platform still needs first-run admin setup. */
export function checkSetupNeeded(): Promise<boolean> {
  if (!cache) {
    cache = apiGet<SetupStatusResponse>(ENDPOINTS.setup.status)
      .then((r) => r.needs_setup)
      // Fail open: on a status error, don't trap the operator on /setup.
      .catch(() => false);
  }
  return cache;
}

/** Marks setup complete so route guards stop redirecting to /setup. */
export function markSetupComplete(): void {
  cache = Promise.resolve(false);
}
