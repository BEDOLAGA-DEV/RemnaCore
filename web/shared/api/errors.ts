import { HTTPError } from "ky";

/** True when err is a ky HTTP error with the given status. */
function hasStatus(err: unknown, status: number): boolean {
  return err instanceof HTTPError && err.response.status === status;
}

/** A 403 — the caller is authenticated but lacks permission for the resource. */
export function isForbidden(err: unknown): boolean {
  return hasStatus(err, 403);
}

/** A 401 — the caller is not (or no longer) authenticated. */
export function isUnauthorized(err: unknown): boolean {
  return hasStatus(err, 401);
}

/** Either an auth (401) or authorization (403) failure. */
export function isAuthError(err: unknown): boolean {
  return isUnauthorized(err) || isForbidden(err);
}
