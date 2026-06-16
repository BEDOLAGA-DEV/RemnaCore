import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  useAdminStats,
  useAdminSubscriptions,
  useAdminInvoices,
  useAdminSessions,
  usePlugins,
  useSystemHealth,
  LoadingSpinner,
  PAGINATION_DEFAULTS,
  formatMoney,
} from "@remnacore/shared";
import type { Invoice, Subscription } from "@remnacore/shared";
import {
  PageHeader,
  Panel,
  PanelHeader,
  KpiGrid,
  StatCell,
  StatusPill,
  Bar,
  PulseDot,
} from "../components/ui/index.js";
import type { Tone } from "../components/ui/index.js";

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

function formatLogTime(isoDate: string): string {
  return new Date(isoDate).toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
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

const FEED_LOG_LEVEL: Record<FeedItemType, string> = {
  invoice_paid: "PAY",
  subscription_activated: "SUB",
  subscription_cancelled: "DEL",
  subscription_paused: "WRN",
};

const FEED_LOG_COLOR: Record<FeedItemType, string> = {
  invoice_paid: "var(--accent)",
  subscription_activated: "var(--accent)",
  subscription_cancelled: "var(--danger)",
  subscription_paused: "var(--warn)",
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

const HEALTH_TONE: Record<"healthy" | "degraded" | "unhealthy", Tone> = {
  healthy: "ok",
  degraded: "warn",
  unhealthy: "danger",
};

const HEALTH_DOT: Record<"healthy" | "degraded" | "unhealthy", string> = {
  healthy: "var(--accent)",
  degraded: "var(--warn)",
  unhealthy: "var(--danger)",
};

function formatLatency(latencyMs: number): string {
  return latencyMs < 1
    ? `${(latencyMs * MS_PER_SECOND).toFixed(0)}µs`
    : `${latencyMs.toFixed(1)}ms`;
}

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

  // Keep derived data referenced so behavior/queries stay intact even when not
  // surfaced in the terminal layout.
  void sessions;
  void planDistribution;
  void enabledPlugins;

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const funnelTotal = stats.totalSubs || 1;
  const funnelRows = [
    { label: "ACTIVE", value: stats.activeSubs, color: "var(--accent)" },
    { label: "PENDING", value: stats.pendingSubs, color: "var(--warn)" },
    { label: "PAUSED", value: stats.pausedSubs, color: "var(--t5)" },
    { label: "CANCELLED", value: stats.cancelledSubs, color: "var(--danger)" },
  ];

  return (
    <div className="flex flex-col gap-3.5">
      <PageHeader
        title="DASHBOARD"
        breadcrumb="REMNAWAVE PROVIDER / OVERVIEW / REAL-TIME"
      />

      {/* KPI grid — 5 live stats */}
      <KpiGrid cols={5}>
        <StatCell
          label={t("admin.dashboard.totalUsers")}
          value={stats.userCount.toLocaleString()}
          foot={<span className="text-[10px] text-accent">LIVE</span>}
        />
        <StatCell
          label={t("admin.dashboard.activeSessions")}
          value={stats.activeSessions.toLocaleString()}
          dot={stats.activeSessions > 0 ? "var(--accent)" : "var(--t6)"}
          foot={<span className="text-[10px] text-accent">LIVE</span>}
        />
        <StatCell
          label={t("admin.dashboard.activeSubscriptions")}
          value={stats.activeSubs.toLocaleString()}
        />
        <StatCell
          label="REVENUE"
          value={formatMoney(stats.totalRevenue)}
          dot="var(--warn)"
        />
        <StatCell
          label="TOTAL SUBS"
          value={stats.totalSubs.toLocaleString()}
          foot={
            <span className="text-[10px] text-t6 tabular-nums">
              {stats.churnRate}% CHURN
            </span>
          }
        />
      </KpiGrid>

      {/* Subscription funnel + system status */}
      <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-2">
        <Panel scanline>
          <PanelHeader title="SUBSCRIPTION FUNNEL" />
          <div className="flex flex-col gap-3.5 px-4 py-4">
            {funnelRows.map((row) => {
              const pct = Math.round((row.value / funnelTotal) * PERCENT_MULTIPLIER);
              return (
                <div key={row.label} className="flex flex-col gap-1.5">
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="uppercase tracking-[1px] text-t5">
                      {row.label}
                    </span>
                    <span className="tabular-nums text-t3">
                      {row.value.toLocaleString()}{" "}
                      <span className="text-t7">({pct}%)</span>
                    </span>
                  </div>
                  <div className="flex items-center">
                    <Bar pct={pct} color={row.color} />
                  </div>
                </div>
              );
            })}
            <div className="mt-1 flex items-center justify-between border-t border-line pt-3 text-[11px]">
              <span className="uppercase tracking-[1px] text-t6">TOTAL</span>
              <span className="tabular-nums text-t1">
                {stats.totalSubs.toLocaleString()}
              </span>
            </div>
          </div>
        </Panel>

        <Panel>
          <PanelHeader
            title="SYSTEM STATUS"
            right={
              <span className="text-[9px] uppercase tracking-[1px] text-t7">
                AUTO-REFRESH 15s
              </span>
            }
          />
          <div className="flex flex-col gap-3 px-4 py-4">
            {healthChecks && healthChecks.length > 0 ? (
              healthChecks.map((check) => (
                <div
                  key={check.name}
                  className="flex items-center justify-between"
                >
                  <div className="flex items-center gap-2.5">
                    <span
                      className="h-[6px] w-[6px] rounded-full"
                      style={{ background: HEALTH_DOT[check.status] }}
                    />
                    <span className="text-[12px] text-t3">
                      {COMPONENT_DISPLAY_NAMES[check.name] ?? check.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="tabular-nums text-[11px] text-t6">
                      {formatLatency(check.latency_ms)}
                    </span>
                    <StatusPill
                      label={check.status}
                      tone={HEALTH_TONE[check.status]}
                    />
                  </div>
                </div>
              ))
            ) : (
              <div className="py-6 text-center text-[11px] uppercase tracking-[2px] text-t7">
                NO DATA
              </div>
            )}
          </div>
        </Panel>
      </div>

      {/* System log — recent invoices / subscriptions as terminal lines */}
      <Panel>
        <PanelHeader title="SYSTEM LOG" right={<PulseDot />} />
        <div className="flex flex-col gap-0.5 px-3.5 py-2">
          {feedItems.length > 0 ? (
            feedItems.map((item) => (
              <div
                key={item.id}
                className="flex items-baseline gap-2 border-b py-[3px] text-[11px]"
                style={{ borderColor: "var(--line-soft)" }}
              >
                <span className="shrink-0 tabular-nums text-t8">
                  {formatLogTime(item.timestamp)}
                </span>
                <span
                  className="w-[34px] shrink-0 text-[9px] uppercase tracking-[1px]"
                  style={{ color: FEED_LOG_COLOR[item.type] }}
                >
                  {FEED_LOG_LEVEL[item.type]}
                </span>
                <span className="min-w-0 flex-1 truncate text-t5">
                  {item.description}
                </span>
                <span className="shrink-0 tabular-nums text-[9px] tracking-[.5px] text-t7">
                  {formatRelativeTime(item.timestamp)}
                </span>
              </div>
            ))
          ) : (
            <div className="py-8 text-center text-[11px] uppercase tracking-[2px] text-t7">
              NO RECENT ACTIVITY
            </div>
          )}
        </div>
      </Panel>
    </div>
  );
}
