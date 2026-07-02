import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import type {
  ActivityListResponse,
  AdminMetrics,
  Invoice,
  MetricsHistoryResponse,
  PaginationParams,
  Subscription,
  Tenant,
  User,
} from "../../types/index.js";
import { apiGet } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

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

export type ActiveSession = {
  id: string;
  user_id: string;
  user_email: string;
  ip_address: string;
  user_agent: string;
  expires_at: string;
  created_at: string;
};

const SESSIONS_REFETCH_INTERVAL_MS = 30_000;

type SessionListResponse = {
  sessions: ActiveSession[];
  total: number;
};

export function useAdminSessions() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.sessions,
    queryFn: async () => {
      const resp = await apiGet<SessionListResponse>(ENDPOINTS.admin.sessions, {
        limit: 50,
        offset: 0,
      });
      return resp.sessions;
    },
    refetchInterval: SESSIONS_REFETCH_INTERVAL_MS,
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

const DEFAULT_ACTIVITY_LIMIT = 40;

export function useAdminMetrics() {
  return useQuery({
    queryKey: ["admin", "metrics"] as const,
    queryFn: () => apiGet<AdminMetrics>(ENDPOINTS.admin.metrics),
    staleTime: STATS_STALE_TIME_MS,
    refetchInterval: STATS_STALE_TIME_MS,
  });
}

export function useAdminActivity(limit = DEFAULT_ACTIVITY_LIMIT) {
  return useQuery({
    queryKey: ["admin", "activity", limit] as const,
    queryFn: () =>
      apiGet<ActivityListResponse>(ENDPOINTS.admin.activity, { limit }),
    staleTime: STATS_STALE_TIME_MS,
    refetchInterval: STATS_STALE_TIME_MS,
  });
}

const DEFAULT_HISTORY_HOURS = 24;

export function useMetricsHistory(hours = DEFAULT_HISTORY_HOURS) {
  return useQuery({
    queryKey: ["admin", "metrics", "history", hours] as const,
    queryFn: () =>
      apiGet<MetricsHistoryResponse>(ENDPOINTS.admin.history, { hours }),
    staleTime: STATS_STALE_TIME_MS,
    refetchInterval: STATS_STALE_TIME_MS,
  });
}
