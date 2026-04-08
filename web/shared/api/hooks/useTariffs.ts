import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost, apiPut, apiDeleteVoid } from "../client.js";
import type { Tariff, TariffInput } from "../../types/index.js";

export function useAdminTariffs() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.tariffs.all,
    queryFn: () => apiGet<Tariff[]>(ENDPOINTS.admin.tariffs.list),
  });
}

export function useTariffs() {
  return useQuery({
    queryKey: ["tariffs"] as const,
    queryFn: () => apiGet<Tariff[]>(ENDPOINTS.tariffs.list),
  });
}

export function useCreateTariff() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: TariffInput) =>
      apiPost<Tariff>(ENDPOINTS.admin.tariffs.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tariffs.all,
      });
    },
  });
}

export function useUpdateTariff() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: TariffInput }) =>
      apiPut<Tariff>(ENDPOINTS.admin.tariffs.update(id), data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tariffs.all,
      });
    },
  });
}

export function useDeleteTariff() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) =>
      apiDeleteVoid(ENDPOINTS.admin.tariffs.delete(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tariffs.all,
      });
    },
  });
}
