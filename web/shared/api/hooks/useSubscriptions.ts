import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost, apiDelete } from "../client.js";
import type { Subscription } from "../../types/index.js";
import type {
  CreateSubscriptionRequest,
  StatusResponse,
  CreateSubscriptionResponse,
  AddAddonRequest,
} from "../types.js";

export function useSubscriptions() {
  return useQuery({
    queryKey: QUERY_KEYS.subscriptions.all,
    queryFn: () => apiGet<Subscription[]>(ENDPOINTS.subscriptions.list),
  });
}

export function useSubscription(id: string) {
  return useQuery({
    queryKey: QUERY_KEYS.subscriptions.detail(id),
    queryFn: () => apiGet<Subscription>(ENDPOINTS.subscriptions.detail(id)),
    enabled: !!id,
  });
}

export function useCreateSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateSubscriptionRequest) =>
      apiPost<CreateSubscriptionResponse>(ENDPOINTS.subscriptions.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.all,
      });
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.invoices.all });
    },
  });
}

export function useCancelSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (subId: string) =>
      apiPost<StatusResponse>(ENDPOINTS.subscriptions.cancel(subId)),
    onSuccess: (_data, subId) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.all,
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.detail(subId),
      });
    },
  });
}

export function useAddAddon() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ subId, data }: { subId: string; data: AddAddonRequest }) =>
      apiPost<StatusResponse>(ENDPOINTS.subscriptions.addAddon(subId), data),
    onSuccess: (_data, { subId }) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.detail(subId),
      });
    },
  });
}

export function useRemoveAddon() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ subId, addonId }: { subId: string; addonId: string }) =>
      apiDelete<StatusResponse>(
        ENDPOINTS.subscriptions.removeAddon(subId, addonId),
      ),
    onSuccess: (_data, { subId }) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.detail(subId),
      });
    },
  });
}
