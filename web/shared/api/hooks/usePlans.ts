import { useQuery } from "@tanstack/react-query";
import { STALE_TIME_PLANS_MS } from "../../lib/constants.js";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import type { Plan } from "../../types/index.js";
import { apiGet } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

export function usePlans() {
  return useQuery({
    queryKey: QUERY_KEYS.plans.all,
    queryFn: () => apiGet<Plan[]>(ENDPOINTS.plans.list),
    staleTime: STALE_TIME_PLANS_MS,
  });
}

export function usePlan(planId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.plans.detail(planId),
    queryFn: () => apiGet<Plan>(ENDPOINTS.plans.detail(planId)),
    enabled: !!planId,
    staleTime: STALE_TIME_PLANS_MS,
  });
}
