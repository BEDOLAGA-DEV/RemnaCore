import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet } from "../client.js";
import type {
  User,
  Subscription,
  Invoice,
  Tenant,
  PaginationParams,
} from "../../types/index.js";

export function useAdminUsers(params?: PaginationParams) {
  const limit = params?.limit ?? 50;
  const offset = params?.offset ?? 0;

  return useQuery({
    queryKey: QUERY_KEYS.admin.users.all(limit, offset),
    queryFn: () =>
      apiGet<User[]>(ENDPOINTS.admin.users.list, { limit, offset }),
  });
}

export function useAdminUser(userId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.admin.users.detail(userId),
    queryFn: () => apiGet<User>(ENDPOINTS.admin.users.detail(userId)),
    enabled: !!userId,
  });
}

export function useAdminSubscriptions(params?: PaginationParams) {
  const limit = params?.limit ?? 50;
  const offset = params?.offset ?? 0;

  return useQuery({
    queryKey: QUERY_KEYS.admin.subscriptions.all(limit, offset),
    queryFn: () =>
      apiGet<Subscription[]>(ENDPOINTS.admin.subscriptions.list, {
        limit,
        offset,
      }),
  });
}

export function useAdminInvoices(params?: PaginationParams) {
  const limit = params?.limit ?? 50;
  const offset = params?.offset ?? 0;

  return useQuery({
    queryKey: QUERY_KEYS.admin.invoices.all(limit, offset),
    queryFn: () =>
      apiGet<Invoice[]>(ENDPOINTS.admin.invoices.list, { limit, offset }),
  });
}

export function useAdminTenants(params?: PaginationParams) {
  const limit = params?.limit ?? 50;
  const offset = params?.offset ?? 0;

  return useQuery({
    queryKey: QUERY_KEYS.admin.tenants.all(limit, offset),
    queryFn: () =>
      apiGet<Tenant[]>(ENDPOINTS.admin.tenants.list, { limit, offset }),
  });
}

export function useAdminTenant(tenantId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.admin.tenants.detail(tenantId),
    queryFn: () => apiGet<Tenant>(ENDPOINTS.admin.tenants.detail(tenantId)),
    enabled: !!tenantId,
  });
}

export type DashboardStats = {
  total_users: number;
  active_sessions: number;
  active_subs: number;
  cancelled_subs: number;
  paused_subs: number;
  pending_subs: number;
  total_subs: number;
  total_revenue: number;
};

const STATS_STALE_TIME_MS = 30_000;

export function useAdminStats() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.stats,
    queryFn: () => apiGet<DashboardStats>(ENDPOINTS.admin.stats),
    staleTime: STATS_STALE_TIME_MS,
    refetchInterval: STATS_STALE_TIME_MS,
  });
}
