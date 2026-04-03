import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost } from "../client.js";
import type { Invoice } from "../../types/index.js";
import type { StatusResponse } from "../types.js";

export function useInvoices() {
  return useQuery({
    queryKey: QUERY_KEYS.invoices.all,
    queryFn: () => apiGet<Invoice[]>(ENDPOINTS.invoices.list),
  });
}

export function usePayInvoice() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (invoiceId: string) =>
      apiPost<StatusResponse>(ENDPOINTS.invoices.pay(invoiceId)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.invoices.all });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.subscriptions.all,
      });
    },
  });
}
