import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { apiGet } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

/** Reseller dashboard: the active shop's branding + tenant-scoped aggregates. */
export type ResellerDashboard = {
  tenant_id: string;
  tenant_name: string;
  branding: unknown;
  summary: {
    active_customers: number;
    active_subscriptions: number;
    pending_commission: number;
    currency: string;
  };
};

export type ResellerCommission = {
  id: string;
  sale_id: string;
  amount: number;
  currency: string;
  status: string;
  created_at: string;
  paid_at: string | null;
};

export type ResellerCustomer = {
  user_id: string;
  email: string;
  display_name: string;
  active_subs_count: number;
  created_at: string;
};

// The active shop is carried automatically via the X-Shop-Id header (set from
// useShopStore.activeShopId by the ky client). Queries are keyed by shopId and
// disabled until a shop is selected.

export function useResellerDashboard(shopId: string | null) {
  return useQuery({
    queryKey: QUERY_KEYS.reseller.dashboard(shopId ?? ""),
    queryFn: () => apiGet<ResellerDashboard>(ENDPOINTS.reseller.dashboard),
    enabled: !!shopId,
  });
}

export function useResellerCommissions(shopId: string | null) {
  return useQuery({
    queryKey: QUERY_KEYS.reseller.commissions(shopId ?? ""),
    queryFn: () =>
      apiGet<{ commissions: ResellerCommission[] }>(
        ENDPOINTS.reseller.commissions,
      ).then((r) => r.commissions ?? []),
    enabled: !!shopId,
  });
}

export function useResellerCustomers(shopId: string | null) {
  return useQuery({
    queryKey: QUERY_KEYS.reseller.customers(shopId ?? ""),
    queryFn: () =>
      apiGet<{ customers: ResellerCustomer[] }>(
        ENDPOINTS.reseller.customers,
      ).then((r) => r.customers ?? []),
    enabled: !!shopId,
  });
}
