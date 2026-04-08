import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  useAdminStats,
  useAdminSubscriptions,
  useAdminInvoices,
  useAdminSessions,
  usePlugins,
  useSystemHealth,
  LoadingSpinner,
  PAGINATION_DEFAULTS,
  cn,
  formatMoney,
} from "@remnacore/shared";
import type { ActiveSession, Invoice, Subscription } from "@remnacore/shared";
import { StatusDot } from "../components/StatusDot.js";

// ─── Helpers ────────────────────────────────────────────────────────────────

const FEED_ITEM_LIMIT = 8;
const MS_PER_SECOND = 1000;
const MS_PER_MINUTE = 60_000;
const MS_PER_HOUR = 3_600_000;
const MS_PER_DAY = 86_400_000;
const PERCENT_MULTIPLIER = 100;

function formatRelativeTime(isoDate: string): string {
  const diff = Date.now() - new Date(isoDate).getTime();

  if (diff < MS_PER_MINUTE) {
    return `${Math.max(1, Math.floor(diff / MS_PER_SECOND))}s ago`;
  }
  if (diff < MS_PER_HOUR) {
    return `${Math.floor(diff / MS_PER_MINUTE)}m ago`;
  }
  if (diff < MS_PER_DAY) {
    return `${Math.floor(diff / MS_PER_HOUR)}h ago`;
  }
  return `${Math.floor(diff / MS_PER_DAY)}d ago`;
}

const USER_AGENT_TRUNCATE_LENGTH = 20;

function shortenUserAgent(ua: string): string {
  if (ua.includes("Chrome")) return "Chrome";
  if (ua.includes("Firefox")) return "Firefox";
  if (ua.includes("Safari")) return "Safari";
  if (ua.includes("Edge")) return "Edge";
  if (ua.includes("Opera") || ua.includes("OPR")) return "Opera";
  return ua.slice(0, USER_AGENT_TRUNCATE_LENGTH);
}

// ─── Activity Feed Item Types ───────────────────────────────────────────────

type FeedItemType =
  | "invoice_paid"
  | "subscription_activated"
  | "subscription_cancelled"
  | "subscription_paused";

type FeedItem = {
  id: string;
  type: FeedItemType;
  description: string;
  timestamp: string;
};

const FEED_DOT_COLORS: Record<FeedItemType, string> = {
  invoice_paid: "bg-primary shadow-[0_0_6px_hsl(var(--primary)/0.6)]",
  subscription_activated:
    "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]",
  subscription_cancelled: "bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)]",
  subscription_paused: "bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.6)]",
};

function buildFeedItems(
  invoices: Invoice[] | undefined,
  subscriptions: Subscription[] | undefined,
): FeedItem[] {
  const items: FeedItem[] = [];

  if (invoices) {
    for (const inv of invoices) {
      if (inv.status === "paid" && inv.paid_at) {
        items.push({
          id: `inv-${inv.id}`,
          type: "invoice_paid",
          description: `Invoice ${inv.id.slice(0, 8)} paid`,
          timestamp: inv.paid_at,
        });
      }
    }
  }

  if (subscriptions) {
    for (const sub of subscriptions) {
      if (sub.status === "active") {
        items.push({
          id: `sub-act-${sub.id}`,
          type: "subscription_activated",
          description: `Subscription ${sub.id.slice(0, 8)} activated`,
          timestamp: sub.updated_at,
        });
      } else if (sub.status === "cancelled" && sub.cancelled_at) {
        items.push({
          id: `sub-can-${sub.id}`,
          type: "subscription_cancelled",
          description: `Subscription ${sub.id.slice(0, 8)} cancelled`,
          timestamp: sub.cancelled_at,
        });
      } else if (sub.status === "paused" && sub.paused_at) {
        items.push({
          id: `sub-pau-${sub.id}`,
          type: "subscription_paused",
          description: `Subscription ${sub.id.slice(0, 8)} paused`,
          timestamp: sub.paused_at,
        });
      }
    }
  }

  items.sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
  );

  return items.slice(0, FEED_ITEM_LIMIT);
}

// ─── System Status ──────────────────────────────────────────────────────────

const COMPONENT_DISPLAY_NAMES: Record<string, string> = {
  postgres: "PostgreSQL",
  valkey: "Valkey",
  nats: "NATS JetStream",
  outbox: "Outbox Relay",
};

// ─── Dashboard Page ─────────────────────────────────────────────────────────

export function AdminDashboardPage() {
  const { t } = useTranslation();

  const { data: serverStats, isLoading: statsLoading } = useAdminStats();
  const { data: subs, isLoading: subsLoading } = useAdminSubscriptions({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: invoices, isLoading: invoicesLoading } = useAdminInvoices({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: plugins, isLoading: pluginsLoading } = usePlugins();
  const { data: sessions, isLoading: sessionsLoading } = useAdminSessions();
  const { data: healthChecks } = useSystemHealth();

  const isLoading =
    statsLoading || subsLoading || invoicesLoading || pluginsLoading || sessionsLoading;

  const stats = useMemo(() => {
    const s = serverStats;
    const userCount = s?.total_users ?? 0;
    const activeSessions = s?.active_sessions ?? 0;
    const activeSubs = s?.active_subs ?? 0;
    const cancelledSubs = s?.cancelled_subs ?? 0;
    const pausedSubs = s?.paused_subs ?? 0;
    const pendingSubs = s?.pending_subs ?? 0;
    const totalSubs = s?.total_subs ?? 0;
    const totalRevenue = s?.total_revenue ?? 0;

    const churnRate =
      totalSubs > 0
        ? ((cancelledSubs / totalSubs) * PERCENT_MULTIPLIER).toFixed(1)
        : "0.0";

    return { userCount, activeSessions, activeSubs, totalSubs, totalRevenue, churnRate, cancelledSubs, pausedSubs, pendingSubs };
  }, [serverStats]);

  const planDistribution = useMemo(() => {
    if (!subs) return [];

    const activeSubs = subs.filter((s) => s.status === "active");
    const planCounts = new Map<string, number>();

    for (const sub of activeSubs) {
      const current = planCounts.get(sub.plan_id) ?? 0;
      planCounts.set(sub.plan_id, current + 1);
    }

    const total = activeSubs.length || 1;
    const colors = [
      "bg-primary",
      "bg-amber-500",
      "bg-violet-400",
      "bg-emerald-500",
      "bg-rose-500",
    ];

    return Array.from(planCounts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([planId, count], index) => ({
        planId: planId.slice(0, 8),
        count,
        percentage: Math.round((count / total) * PERCENT_MULTIPLIER),
        color: colors[index % colors.length] ?? "bg-muted",
      }));
  }, [subs]);

  const feedItems = useMemo(
    () => buildFeedItems(invoices, subs),
    [invoices, subs],
  );

  const enabledPlugins = useMemo(
    () => plugins?.filter((p) => p.status === "enabled") ?? [],
    [plugins],
  );

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight text-foreground">
          {t("admin.dashboard.title")}
        </h1>
        <span className="font-mono text-[11px] text-muted-foreground">
          {new Date().toLocaleDateString("en-US", {
            weekday: "short",
            month: "short",
            day: "numeric",
          })}
        </span>
      </div>

      {/* Row 1: KPI Cards */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <KpiCard
          label={t("admin.dashboard.totalUsers")}
          value={stats.userCount.toLocaleString()}
        />
        <KpiCard
          label="Sessions"
          value={stats.activeSessions.toLocaleString()}
          highlight={stats.activeSessions > 0}
        />
        <KpiCard
          label={t("admin.dashboard.activeSubscriptions")}
          value={stats.activeSubs.toLocaleString()}
        />
        <KpiCard
          label="MRR"
          value={formatMoney(stats.totalRevenue)}
        />
        <KpiCard
          label="Churn"
          value={`${stats.churnRate}%`}
        />
      </div>

      {/* Row 1.5: Active Sessions */}
      <ActiveSessionsCard sessions={sessions} />

      {/* Row 2: Subscription Funnel + Plan Distribution + System Status */}
      <div className="grid gap-3 lg:grid-cols-[1fr_1fr_1fr]">
        {/* Subscription Funnel */}
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="mb-4 text-[11px] uppercase tracking-wider text-muted-foreground">
            Subscription Funnel
          </p>
          <div className="space-y-3">
            <FunnelRow label="Active" value={stats.activeSubs} total={stats.totalSubs} color="bg-primary" />
            <FunnelRow label="Pending" value={stats.pendingSubs} total={stats.totalSubs} color="bg-amber-500" />
            <FunnelRow label="Paused" value={stats.pausedSubs} total={stats.totalSubs} color="bg-violet-400" />
            <FunnelRow label="Cancelled" value={stats.cancelledSubs} total={stats.totalSubs} color="bg-red-500" />
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-muted-foreground">Total</span>
              <span className="font-mono text-sm font-semibold text-foreground">
                {stats.totalSubs}
              </span>
            </div>
          </div>
        </div>

        {/* Plan Distribution */}
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="mb-4 text-[11px] uppercase tracking-wider text-muted-foreground">
            Plan Distribution
          </p>
          <div className="space-y-3">
            {planDistribution.length > 0 ? (
              planDistribution.map((plan) => (
                <div key={plan.planId}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="font-mono text-xs text-foreground">
                      {plan.planId}
                    </span>
                    <span className="font-mono text-[11px] text-muted-foreground">
                      {plan.count} ({plan.percentage}%)
                    </span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
                    <div
                      className={cn("h-full rounded-full", plan.color)}
                      style={{ width: `${plan.percentage}%` }}
                    />
                  </div>
                </div>
              ))
            ) : (
              <p className="py-4 text-center font-mono text-xs text-muted-foreground">
                No active plans
              </p>
            )}
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-muted-foreground">
                Total active
              </span>
              <span className="font-mono text-sm font-semibold text-foreground">
                {stats.activeSubs}
              </span>
            </div>
          </div>
        </div>

        {/* System Status */}
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="mb-4 text-[11px] uppercase tracking-wider text-muted-foreground">
            System Status
          </p>
          <div className="space-y-3">
            {healthChecks && healthChecks.length > 0 ? (
              healthChecks.map((check) => (
                <div
                  key={check.name}
                  className="flex items-center justify-between"
                >
                  <div className="flex items-center gap-2">
                    <StatusDot status={check.status} />
                    <span className="text-xs text-foreground">
                      {COMPONENT_DISPLAY_NAMES[check.name] ?? check.name}
                    </span>
                  </div>
                  <span className="font-mono text-[11px] text-muted-foreground">
                    {check.latency_ms < 1
                      ? `${(check.latency_ms * 1000).toFixed(0)}µs`
                      : `${check.latency_ms.toFixed(1)}ms`}
                  </span>
                </div>
              ))
            ) : (
              <p className="py-2 text-center font-mono text-xs text-muted-foreground">
                Loading...
              </p>
            )}
          </div>
        </div>
      </div>

      {/* Row 3: Activity Feed + Active Plugins */}
      <div className="grid gap-3 lg:grid-cols-[3fr_2fr]">
        {/* Activity Feed */}
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="mb-4 flex items-center justify-between">
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
              Activity Feed
            </p>
            <div className="flex items-center gap-1.5">
              <span className="inline-block size-1.5 animate-pulse rounded-full bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]" />
              <span className="font-mono text-[10px] text-emerald-500">
                streaming
              </span>
            </div>
          </div>
          <div className="space-y-2.5">
            {feedItems.length > 0 ? (
              feedItems.map((item) => (
                <div key={item.id} className="flex items-center gap-2.5">
                  <span
                    className={cn(
                      "inline-block size-1.5 shrink-0 rounded-full",
                      FEED_DOT_COLORS[item.type],
                    )}
                  />
                  <span className="flex-1 truncate font-mono text-xs text-foreground">
                    {item.description}
                  </span>
                  <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                    {formatRelativeTime(item.timestamp)}
                  </span>
                </div>
              ))
            ) : (
              <p className="py-4 text-center font-mono text-xs text-muted-foreground">
                No recent activity
              </p>
            )}
          </div>
        </div>

        {/* Active Plugins */}
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="mb-4 flex items-center gap-2">
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
              Active Plugins
            </p>
            <span className="rounded bg-amber-500/10 px-1.5 py-0.5 font-mono text-[10px] font-medium text-amber-500">
              {enabledPlugins.length}
            </span>
          </div>
          <div className="space-y-2">
            {enabledPlugins.length > 0 ? (
              enabledPlugins.map((plugin) => (
                <div
                  key={plugin.id}
                  className="flex items-center justify-between rounded-lg bg-secondary p-2.5"
                >
                  <div className="flex items-center gap-2">
                    <StatusDot
                      status={
                        plugin.status === "enabled" ? "healthy" : "degraded"
                      }
                    />
                    <span className="text-xs font-medium text-foreground">
                      {plugin.name}
                    </span>
                  </div>
                  <span className="font-mono text-[10px] text-muted-foreground">
                    v{plugin.version}
                  </span>
                </div>
              ))
            ) : (
              <p className="py-4 text-center font-mono text-xs text-muted-foreground">
                No active plugins
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── KPI Card ───────────────────────────────────────────────────────────────

type KpiCardProps = {
  label: string;
  value: string;
  highlight?: boolean;
};

function KpiCard({ label, value, highlight }: KpiCardProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p
        className={cn(
          "mt-2 font-mono text-[28px] font-bold leading-none tracking-tight",
          highlight ? "text-primary" : "text-foreground",
        )}
      >
        {value}
      </p>
    </div>
  );
}

// ─── Funnel Row ─────────────────────────────────────────────────────────────

type FunnelRowProps = {
  label: string;
  value: number;
  total: number;
  color: string;
};

function FunnelRow({ label, value, total, color }: FunnelRowProps) {
  const percentage = total > 0 ? Math.round((value / total) * PERCENT_MULTIPLIER) : 0;

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-xs text-muted-foreground">{label}</span>
        <span className="font-mono text-[11px] text-foreground">
          {value}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
        <div
          className={cn("h-full rounded-full", color)}
          style={{ width: `${Math.max(percentage, percentage > 0 ? 2 : 0)}%` }}
        />
      </div>
    </div>
  );
}

// ─── Active Sessions Card ────────────────────────────────────────────────────

type ActiveSessionsCardProps = {
  sessions: ActiveSession[] | undefined;
};

function ActiveSessionsCard({ sessions }: ActiveSessionsCardProps) {
  const { t } = useTranslation();
  const sessionCount = sessions?.length ?? 0;

  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <div className="mb-4 flex items-center gap-2">
        <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
          {t("admin.dashboard.activeSessions")}
        </p>
        <span className="rounded bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] font-medium text-emerald-500">
          {sessionCount} {t("admin.dashboard.online")}
        </span>
      </div>
      {sessionCount > 0 ? (
        <div className="divide-y divide-border/30">
          {sessions?.map((session) => (
            <div
              key={session.id}
              className="flex items-center gap-4 py-2 hover:bg-secondary/50"
            >
              <Link
                to="/users/$id"
                params={{ id: session.user_id }}
                className="shrink-0 font-mono text-xs text-primary hover:underline"
              >
                {session.user_email}
              </Link>
              <span className="shrink-0 font-mono text-xs text-foreground">
                {session.ip_address}
              </span>
              <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                {shortenUserAgent(session.user_agent)}
              </span>
              <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                {formatRelativeTime(session.created_at)}
              </span>
            </div>
          ))}
        </div>
      ) : (
        <p className="py-4 text-center font-mono text-xs text-muted-foreground">
          {t("admin.dashboard.noActiveSessions")}
        </p>
      )}
    </div>
  );
}
