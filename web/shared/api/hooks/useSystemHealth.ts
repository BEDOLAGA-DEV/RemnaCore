import { useQuery } from "@tanstack/react-query";
import ky from "ky";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";

type ComponentStatus = "healthy" | "degraded" | "unhealthy";

export type HealthCheck = {
  name: string;
  status: ComponentStatus;
  message?: string;
  latency_ms: number;
};

type ReadyzResponse = {
  status: string;
  checks: HealthCheck[];
};

const HEALTH_POLL_INTERVAL_MS = 15_000;

/**
 * Fetches system health from /readyz.
 * Uses a plain ky instance (no auth required for readiness probes).
 */
async function fetchReadyz(): Promise<HealthCheck[]> {
  const baseUrl =
    typeof import.meta !== "undefined" && import.meta.env?.VITE_API_URL
      ? (import.meta.env.VITE_API_URL as string)
      : "";

  const response = await ky
    .get(`${baseUrl}${ENDPOINTS.infra.readyz}`)
    .json<ReadyzResponse>();

  return response.checks;
}

export function useSystemHealth() {
  return useQuery({
    queryKey: QUERY_KEYS.infra.readyz,
    queryFn: fetchReadyz,
    refetchInterval: HEALTH_POLL_INTERVAL_MS,
    retry: 1,
    staleTime: HEALTH_POLL_INTERVAL_MS,
  });
}
