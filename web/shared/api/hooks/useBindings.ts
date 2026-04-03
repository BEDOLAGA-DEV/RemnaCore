import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet } from "../client.js";
import type { Binding } from "../../types/index.js";

export function useBindings() {
  return useQuery({
    queryKey: QUERY_KEYS.bindings.all,
    queryFn: () => apiGet<Binding[]>(ENDPOINTS.bindings.list),
  });
}

export function useSubscriptionBindings(subId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.subscriptions.bindings(subId),
    queryFn: () =>
      apiGet<Binding[]>(ENDPOINTS.subscriptions.bindings(subId)),
    enabled: !!subId,
  });
}
