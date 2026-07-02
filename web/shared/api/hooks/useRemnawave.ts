import { useQuery } from "@tanstack/react-query";
import type {
  RemnawaveNodeRow,
  RemnawaveOverview,
  RemnawaveRealtime,
  TrafficByNode,
} from "../../types/index.js";
import { apiGet } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

const REMNAWAVE_STALE_TIME_MS = 30_000;

export function useRemnawaveOverview() {
  return useQuery({
    queryKey: ["remnawave", "overview"] as const,
    queryFn: () => apiGet<RemnawaveOverview>(ENDPOINTS.remnawave.overview),
    staleTime: REMNAWAVE_STALE_TIME_MS,
    refetchInterval: REMNAWAVE_STALE_TIME_MS,
  });
}

export function useRemnawaveRealtime() {
  return useQuery({
    queryKey: ["remnawave", "realtime"] as const,
    queryFn: () => apiGet<RemnawaveRealtime>(ENDPOINTS.remnawave.realtime),
    staleTime: REMNAWAVE_STALE_TIME_MS,
    refetchInterval: REMNAWAVE_STALE_TIME_MS,
  });
}

export function useRemnawaveNodes() {
  return useQuery({
    queryKey: ["remnawave", "nodes"] as const,
    // NOTE: /api/remnawave/nodes returns a top-level array.
    queryFn: () => apiGet<RemnawaveNodeRow[]>(ENDPOINTS.remnawave.nodes),
    staleTime: REMNAWAVE_STALE_TIME_MS,
    refetchInterval: REMNAWAVE_STALE_TIME_MS,
  });
}

export function useRemnawaveTrafficByNode() {
  return useQuery({
    queryKey: ["remnawave", "traffic-by-node"] as const,
    queryFn: () => apiGet<TrafficByNode>(ENDPOINTS.remnawave.trafficByNode),
    staleTime: REMNAWAVE_STALE_TIME_MS,
    refetchInterval: REMNAWAVE_STALE_TIME_MS,
  });
}
