import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  useAdminStats,
  useAdminMetrics,
  useAdminActivity,
  useAdminSubscriptions,
  useRemnawaveOverview,
  useRemnawaveNodes,
  useRemnawaveRealtime,
  usePlugins,
  useSystemHealth,
  LoadingSpinner,
  PAGINATION_DEFAULTS,
  formatMoney,
} from "@remnacore/shared";
import type {
  ActivityEntry,
  ActivityLevel,
  RemnawaveNodeRow,
  RealtimeNodeMetrics,
} from "@remnacore/shared";
import {
  PageHeader,
  Panel,
  PanelHeader,
  KpiGrid,
  StatCell,
  StatusPill,
  Bar,
  PulseDot,
  DataTable,
} from "../components/ui/index.js";
import type { Tone, Column } from "../components/ui/index.js";

// ─── Constants ────────────────────────────────────────────────────────────────

const PERCENT_MULTIPLIER = 100;
const REMNAWAVE_PLUGIN_SLUG = "remnawave-provider";
const BYTES_PER_GB = 1024 ** 3;
const SPEED_BAR_MAX_BYTES = 100 * 1024 * 1024; // 100 MB/s = full bar

// ─── Helpers ────────────────────────────────────────────────────────────────

function formatLogTime(isoDate: string): string {
  return new Date(isoDate).toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

/** Compact byte formatter for the terminal node table. */
function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / 1024 ** i;
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i] ?? "TB"}`;
}

// ─── Activity Log ─────────────────────────────────────────────────────────────

const ACTIVITY_LEVEL_LABEL: Record<ActivityLevel, string> = {
  ok: "OK",
  warn: "WRN",
  info: "INF",
};

const ACTIVITY_LEVEL_COLOR: Record<ActivityLevel, string> = {
  ok: "var(--accent)",
  warn: "var(--warn)",
  info: "var(--t5)",
};

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

const MS_PER_SECOND = 1000;

function formatLatency(latencyMs: number): string {
  return latencyMs < 1
    ? `${(latencyMs * MS_PER_SECOND).toFixed(0)}µs`
    : `${latencyMs.toFixed(1)}ms`;
}

// ─── Dashboard Page ─────────────────────────────────────────────────────────

export function AdminDashboardPage() {
  const { t } = useTranslation();

  const { data: serverStats, isLoading: statsLoading } = useAdminStats();
  const { data: metrics, isLoading: metricsLoading } = useAdminMetrics();
  const { data: activity } = useAdminActivity();
  const { data: subs, isLoading: subsLoading } = useAdminSubscriptions({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: plugins, isLoading: pluginsLoading } = usePlugins();
  const { data: healthChecks } = useSystemHealth();

  // Remnawave plugin gating — Plugin type uses `status`, not `enabled`.
  const rwEnabled =
    plugins?.some(
      (p) => p.slug === REMNAWAVE_PLUGIN_SLUG && p.status === "enabled",
    ) ?? false;

  // Hooks are always called (rules-of-hooks); the RENDER is gated on rwEnabled.
  const { data: rwOverview } = useRemnawaveOverview();
  const { data: rwNodes } = useRemnawaveNodes();
  const { data: rwRealtime } = useRemnawaveRealtime();

  const isLoading =
    statsLoading || metricsLoading || subsLoading || pluginsLoading;

  const planDistribution = useMemo(() => {
    if (!subs) return [];

    const activeSubs = subs.filter((s) => s.status === "active");
    const planCounts = new Map<string, number>();

    for (const sub of activeSubs) {
      const current = planCounts.get(sub.plan_id) ?? 0;
      planCounts.set(sub.plan_id, current + 1);
    }

    const total = activeSubs.length || 1;

    return Array.from(planCounts.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([planId, count]) => ({
        planId: planId.slice(0, 8),
        count,
        percentage: Math.round((count / total) * PERCENT_MULTIPLIER),
      }));
  }, [subs]);

  const enabledPlugins = useMemo(
    () => plugins?.filter((p) => p.status === "enabled") ?? [],
    [plugins],
  );

  // Join realtime node metrics by uuid for the node table.
  const realtimeByUuid = useMemo(() => {
    const map = new Map<string, RealtimeNodeMetrics>();
    for (const panel of rwRealtime?.panels ?? []) {
      for (const m of panel.metrics ?? []) {
        map.set(m.uuid, m);
      }
    }
    return map;
  }, [rwRealtime]);

  // Aggregate throughput across all panel node metrics → GB/s.
  const throughputGbps = useMemo(() => {
    let totalSpeed = 0;
    for (const panel of rwRealtime?.panels ?? []) {
      for (const m of panel.metrics ?? []) {
        totalSpeed += m.uploadSpeedBytes + m.downloadSpeedBytes;
      }
    }
    return totalSpeed / BYTES_PER_GB;
  }, [rwRealtime]);

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const churnPct = metrics ? (metrics.churn_30d * PERCENT_MULTIPLIER).toFixed(1) : "0.0";

  const funnelTotal = metrics?.total_subs || 1;
  const funnelRows = [
    { label: "ACTIVE", value: metrics?.active_subs ?? 0, color: "var(--accent)" },
    { label: "PENDING", value: metrics?.pending_subs ?? 0, color: "var(--warn)" },
    { label: "PAUSED", value: metrics?.paused_subs ?? 0, color: "var(--t5)" },
    {
      label: "CANCELLED",
      value: metrics?.cancelled_subs ?? 0,
      color: "var(--danger)",
    },
  ];

  // ─── Remnawave node table columns ───────────────────────────────────────
  const nodeColumns: Column<RemnawaveNodeRow>[] = [
    {
      key: "node",
      header: "NODE",
      render: (row) => (
        <span className="truncate text-t3">{row.node.name}</span>
      ),
    },
    {
      key: "status",
      header: "STATUS",
      render: (row) => (
        <StatusPill
          label={row.node.isConnected ? "CONNECTED" : "OFFLINE"}
          tone={row.node.isConnected ? "ok" : "danger"}
        />
      ),
    },
    {
      key: "users",
      header: "USERS",
      align: "right",
      render: (row) => {
        const m = realtimeByUuid.get(row.node.uuid);
        return (
          <span className="tabular-nums text-t4">
            {m ? m.activeConnections.toLocaleString() : "—"}
          </span>
        );
      },
    },
    {
      key: "load",
      header: "LOAD",
      render: (row) => {
        const m = realtimeByUuid.get(row.node.uuid);
        if (!m) {
          return <span className="text-t8">—</span>;
        }
        const speed = m.uploadSpeedBytes + m.downloadSpeedBytes;
        const pct = Math.round((speed / SPEED_BAR_MAX_BYTES) * PERCENT_MULTIPLIER);
        return (
          <span className="flex items-center">
            <Bar pct={pct} />
          </span>
        );
      },
    },
    {
      key: "traffic",
      header: "TRAFFIC",
      align: "right",
      render: (row) => (
        <span className="tabular-nums text-t5">
          {formatBytes(row.node.trafficUsedBytes)}
        </span>
      ),
    },
  ];

  const showRemnawaveCluster = rwEnabled && !!rwOverview;

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
          value={(serverStats?.total_users ?? 0).toLocaleString()}
          foot={<span className="text-[10px] text-accent">LIVE</span>}
        />
        <StatCell
          label="MRR"
          value={formatMoney(metrics?.mrr_cents ?? 0)}
          dot="var(--accent)"
        />
        <StatCell
          label="ARPU"
          value={formatMoney(Math.round(metrics?.arpu_cents ?? 0))}
          dot="var(--accent)"
        />
        <StatCell
          label="CHURN 30D"
          value={`${churnPct}%`}
          dot="var(--warn)"
        />
        <StatCell
          label="SUBS TODAY"
          value={(metrics?.subs_today ?? 0).toLocaleString()}
        />
      </KpiGrid>

      {/* Remnawave widget cluster — gated on plugin enabled + data present */}
      {showRemnawaveCluster && (
        <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-2">
          <Panel>
            <PanelHeader
              title="NODE STATUS"
              right={
                <span className="text-[9px] uppercase tracking-[1px] text-t7">
                  {rwOverview.nodes_healthy}/{rwOverview.nodes_total} HEALTHY
                </span>
              }
            />
            <div className="px-4 py-4">
              <DataTable<RemnawaveNodeRow>
                columns={nodeColumns}
                rows={rwNodes ?? []}
                cols="1.4fr 1.1fr .7fr 1fr .9fr"
                rowKey={(r) => r.node.uuid}
                empty="NO NODES"
              />
            </div>
          </Panel>

          <Panel scanline>
            <PanelHeader
              title="REMNAWAVE THROUGHPUT"
              right={<PulseDot />}
            />
            <div className="flex flex-col gap-4 px-4 py-4">
              <div className="flex flex-col gap-1">
                <span className="text-[10px] uppercase tracking-[1.5px] text-t6">
                  AGGREGATE THROUGHPUT
                </span>
                <span className="text-[34px] font-bold leading-none tracking-[.5px] tabular-nums text-accent">
                  {throughputGbps.toFixed(2)}{" "}
                  <span className="text-[14px] text-t6">GB/S</span>
                </span>
              </div>
              <div className="grid grid-cols-3 gap-3 border-t border-line pt-3 text-[11px]">
                <div className="flex flex-col gap-1">
                  <span className="uppercase tracking-[1px] text-t7">
                    PANELS
                  </span>
                  <span className="tabular-nums text-t3">
                    {rwOverview.panels_up}/{rwOverview.panels_total}
                  </span>
                </div>
                <div className="flex flex-col gap-1">
                  <span className="uppercase tracking-[1px] text-t7">
                    ACTIVE USERS
                  </span>
                  <span className="tabular-nums text-t3">
                    {rwOverview.active_users.toLocaleString()}
                  </span>
                </div>
                <div className="flex flex-col gap-1">
                  <span className="uppercase tracking-[1px] text-t7">
                    BW TODAY
                  </span>
                  <span className="tabular-nums text-t3">
                    {formatBytes(rwOverview.bandwidth.total)}
                  </span>
                </div>
              </div>
            </div>
          </Panel>
        </div>
      )}

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
                {(metrics?.total_subs ?? 0).toLocaleString()}
              </span>
            </div>
          </div>
        </Panel>

        <Panel>
          <PanelHeader
            title="SYSTEM STATUS"
            right={
              <span className="text-[9px] uppercase tracking-[1px] text-t7">
                {enabledPlugins.length} PLUGINS · AUTO-REFRESH 30s
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
            {!rwEnabled && (
              <div className="flex items-center justify-between border-t border-line pt-3">
                <div className="flex items-center gap-2.5">
                  <span
                    className="h-[6px] w-[6px] rounded-full"
                    style={{ background: "var(--t6)" }}
                  />
                  <span className="text-[12px] text-t5">Remnawave</span>
                </div>
                <span className="text-[9px] uppercase tracking-[1.5px] text-t7">
                  OFFLINE
                </span>
              </div>
            )}
          </div>
        </Panel>
      </div>

      {/* Subscriptions by plan + system log */}
      <div className="grid grid-cols-1 gap-3.5 lg:grid-cols-[320px_1fr]">
        <Panel>
          <PanelHeader title="SUBSCRIPTIONS BY PLAN" />
          <div className="flex flex-col gap-3 px-4 py-4">
            {planDistribution.length > 0 ? (
              planDistribution.map((p) => (
                <div key={p.planId} className="flex flex-col gap-1.5">
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="tracking-[1px] text-t3">{p.planId}</span>
                    <span className="tabular-nums text-t6">
                      {p.count} ({p.percentage}%)
                    </span>
                  </div>
                  <div className="flex items-center">
                    <Bar pct={p.percentage} />
                  </div>
                </div>
              ))
            ) : (
              <div className="py-6 text-center text-[11px] uppercase tracking-[2px] text-t7">
                NO ACTIVE SUBSCRIPTIONS
              </div>
            )}
          </div>
        </Panel>

        <Panel>
          <PanelHeader title="SYSTEM LOG" right={<PulseDot />} />
          <div className="flex flex-col gap-0.5 px-3.5 py-2">
            {activity?.activity && activity.activity.length > 0 ? (
              activity.activity.map((entry: ActivityEntry) => (
                <div
                  key={entry.id}
                  className="flex items-baseline gap-2 border-b py-[3px] text-[11px]"
                  style={{ borderColor: "var(--line-soft)" }}
                >
                  <span className="shrink-0 tabular-nums text-t8">
                    {formatLogTime(entry.timestamp)}
                  </span>
                  <span
                    className="w-[34px] shrink-0 text-[9px] uppercase tracking-[1px]"
                    style={{ color: ACTIVITY_LEVEL_COLOR[entry.level] }}
                  >
                    {ACTIVITY_LEVEL_LABEL[entry.level]}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-t5">
                    {entry.message}
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
    </div>
  );
}
