import ky, { type KyInstance, type NormalizedOptions } from "ky";
import { useAuthStore } from "../stores/authStore.js";
import { ENDPOINTS } from "./endpoints.js";

/**
 * API base URL. Reads from env at runtime (Vite injects import.meta.env).
 * Falls back to the current origin for production builds that serve the
 * SPA from the same host as the backend.
 */
const getBaseUrl = (): string => {
  if (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_URL) {
    return import.meta.env.VITE_API_URL as string;
  }
  return "";
};

let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

/**
 * Attempt to refresh the access token using the stored refresh token.
 * Returns true if refresh succeeded, false otherwise.
 */
async function refreshAccessToken(): Promise<boolean> {
  const { refreshToken, setTokens, logout } = useAuthStore.getState();
  if (!refreshToken) {
    logout();
    return false;
  }

  try {
    const response = await ky
      .post(`${getBaseUrl()}${ENDPOINTS.auth.refresh}`, {
        json: { refresh_token: refreshToken },
      })
      .json<{ access_token: string; refresh_token: string }>();

    setTokens(response.access_token, response.refresh_token);
    return true;
  } catch {
    logout();
    return false;
  }
}

/**
 * Singleton ky client with JWT auth and automatic token refresh.
 *
 * - beforeRequest: attaches Authorization header from Zustand store
 * - afterResponse: on 401, attempts one refresh rotation then retries
 */
export const apiClient: KyInstance = ky.create({
  prefixUrl: "",
  hooks: {
    beforeRequest: [
      (request: Request) => {
        // 1. Set auth header (always, regardless of URL rewrite)
        const { accessToken } = useAuthStore.getState();
        if (accessToken) {
          request.headers.set("Authorization", `Bearer ${accessToken}`);
        }
      },
    ],
    afterResponse: [
      async (
        request: Request,
        _options: NormalizedOptions,
        response: Response,
      ) => {
        if (response.status !== 401) return response;

        // Don't retry refresh or login endpoints
        const url = new URL(request.url);
        if (
          url.pathname.includes("/auth/refresh") ||
          url.pathname.includes("/auth/login")
        ) {
          return response;
        }

        // Deduplicate concurrent refresh attempts
        if (!isRefreshing) {
          isRefreshing = true;
          refreshPromise = refreshAccessToken().finally(() => {
            isRefreshing = false;
            refreshPromise = null;
          });
        }

        const success = await refreshPromise;
        if (!success) return response;

        // Retry the original request with new token
        const { accessToken } = useAuthStore.getState();
        const newRequest = new Request(request);
        if (accessToken) {
          newRequest.headers.set("Authorization", `Bearer ${accessToken}`);
        }
        return ky(newRequest);
      },
    ],
  },
});

/**
 * Typed JSON GET request.
 */
export async function apiGet<T>(
  endpoint: string,
  searchParams?: Record<string, string | number>,
): Promise<T> {
  const baseUrl = getBaseUrl();
  return apiClient
    .get(`${baseUrl}${endpoint}`, {
      searchParams: searchParams as Record<string, string>,
    })
    .json<T>();
}

/**
 * Typed JSON POST request.
 */
export async function apiPost<T>(
  endpoint: string,
  body?: unknown,
): Promise<T> {
  const baseUrl = getBaseUrl();
  return apiClient.post(`${baseUrl}${endpoint}`, { json: body }).json<T>();
}

/**
 * Typed JSON PUT request.
 */
export async function apiPut<T>(
  endpoint: string,
  body?: unknown,
): Promise<T> {
  const baseUrl = getBaseUrl();
  return apiClient.put(`${baseUrl}${endpoint}`, { json: body }).json<T>();
}

/**
 * Typed JSON DELETE request.
 */
export async function apiDelete<T>(
  endpoint: string,
  searchParams?: Record<string, string>,
): Promise<T> {
  const baseUrl = getBaseUrl();
  return apiClient
    .delete(`${baseUrl}${endpoint}`, { searchParams })
    .json<T>();
}
