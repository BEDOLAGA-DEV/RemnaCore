import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  useAdminUsers,
  useAdminSubscriptions,
  useAdminInvoices,
  usePlugins,
  useSystemHealth,
  LoadingSpinner,
  PAGINATION_DEFAULTS,
  cn,
  formatMoney,
} from "@remnacore/shared";
import type { Invoice, Subscription } from "@remnacore/shared";
import { Sparkline } from "../components/Sparkline.js";
import { StatusDot } from "../components/StatusDot.js";

// ─── Helpers ────────────────────────────────────────────────────────────────

const MOCK_SPARKLINE_LENGTH = 10;
const FEED_ITEM_LIMIT = 8;
const MS_PER_SECOND = 1000;
const MS_PER_MINUTE = 60_000;
const MS_PER_HOUR = 3_600_000;
const MS_PER_DAY = 86_400_000;
const PERCENT_MULTIPLIER = 100;

function generateSparkline(endValue: number, trend: "up" | "down"): number[] {
  const points: number[] = [];
  const base = trend === "up" ? endValue * 0.6 : endValue * 1.4;
  const step = (endValue - base) / (MOCK_SPARKLINE_LENGTH - 1);

  for (let i = 0; i < MOCK_SPARKLINE_LENGTH; i++) {
    const jitter = (Math.random() - 0.5) * Math.abs(step) * 2;
    points.push(Math.max(0, base + step * i + jitter));
  }

  points[MOCK_SPARKLINE_LENGTH - 1] = endValue;
  return points;
}

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

// ─── Revenue Chart (SVG) ────────────────────────────────────────────────────

const MOCK_REVENUE_MONTHS = ["Oct", "Nov", "Dec", "Jan", "Feb", "Mar"];
const MOCK_REVENUE_VALUES = [9200, 11400, 10800, 14200, 16100, 18400];
const CHART_WIDTH = 420;
const CHART_HEIGHT = 140;
const CHART_PADDING_X = 32;
const CHART_PADDING_TOP = 12;
const CHART_PADDING_BOTTOM = 24;

function RevenueChart() {
  const { points, areaPath, gradientId } = useMemo(() => {
    const min = Math.min(...MOCK_REVENUE_VALUES) * 0.8;
    const max = Math.max(...MOCK_REVENUE_VALUES) * 1.1;
    const range = max - min || 1;
    const usableWidth = CHART_WIDTH - CHART_PADDING_X * 2;
    const usableHeight = CHART_HEIGHT - CHART_PADDING_TOP - CHART_PADDING_BOTTOM;
    const step = usableWidth / (MOCK_REVENUE_VALUES.length - 1);

    const pts = MOCK_REVENUE_VALUES.map((v, i) => ({
      x: CHART_PADDING_X + i * step,
      y:
        CHART_PADDING_TOP +
        usableHeight -
        ((v - min) / range) * usableHeight,
    }));

    const polyline = pts.map((p) => `${p.x},${p.y}`).join(" ");
    const firstX = pts[0]?.x ?? 0;
    const lastX = pts[pts.length - 1]?.x ?? 0;
    const bottomY = CHART_HEIGHT - CHART_PADDING_BOTTOM;
    const area = `M ${firstX},${bottomY} L ${pts.map((p) => `${p.x},${p.y}`).join(" L ")} L ${lastX},${bottomY} Z`;

    const id = "rev-chart-grad";

    return { points: pts, polyline, areaPath: area, gradientId: id };
  }, []);

  return (
    <svg
      viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
      className="w-full"
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.2} />
          <stop
            offset="100%"
            stopColor="hsl(var(--primary))"
            stopOpacity={0}
          />
        </linearGradient>
      </defs>
      <path d={areaPath} fill={`url(#${gradientId})`} />
      <polyline
        points={points.map((p) => `${p.x},${p.y}`).join(" ")}
        fill="none"
        stroke="hsl(var(--primary))"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {points.map((p, i) => (
        <circle
          key={MOCK_REVENUE_MONTHS[i]}
          cx={p.x}
          cy={p.y}
          r={3}
          fill="hsl(var(--primary))"
        />
      ))}
      {MOCK_REVENUE_MONTHS.map((month, i) => {
        const x =
          CHART_PADDING_X +
          (i * (CHART_WIDTH - CHART_PADDING_X * 2)) /
            (MOCK_REVENUE_MONTHS.length - 1);
        return (
          <text
            key={month}
            x={x}
            y={CHART_HEIGHT - 4}
            textAnchor="middle"
            className="fill-muted-foreground text-[10px] font-mono"
          >
            {month}
          </text>
        );
      })}
    </svg>
  );
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

  const { data: users, isLoading: usersLoading } = useAdminUsers({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: subs, isLoading: subsLoading } = useAdminSubscriptions({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: invoices, isLoading: invoicesLoading } = useAdminInvoices({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: plugins, isLoading: pluginsLoading } = usePlugins();
  const { data: healthChecks } = useSystemHealth();

  const isLoading =
    usersLoading || subsLoading || invoicesLoading || pluginsLoading;

  const stats = useMemo(() => {
    const userCount = users?.length ?? 0;
    const totalSubs = subs?.length ?? 0;
    const activeSubs = subs?.filter((s) => s.status === "active").length ?? 0;
    const cancelledSubs =
      subs?.filter((s) => s.status === "cancelled").length ?? 0;

    const paidInvoices = invoices?.filter((i) => i.status === "paid") ?? [];
    const totalRevenue = paidInvoices.reduce(
      (sum, inv) => sum + inv.total_amount,
      0,
    );

    const churnRate =
      totalSubs > 0
        ? ((cancelledSubs / totalSubs) * PERCENT_MULTIPLIER).toFixed(1)
        : "0.0";

    return { userCount, activeSubs, totalSubs, totalRevenue, churnRate, cancelledSubs };
  }, [users, subs, invoices]);

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
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          label={t("admin.dashboard.totalUsers")}
          value={stats.userCount.toLocaleString()}
          change="+12%"
          trend="up"
          sparklineData={generateSparkline(stats.userCount, "up")}
          sparklineColor="hsl(var(--primary))"
        />
        <KpiCard
          label={t("admin.dashboard.activeSubscriptions")}
          value={stats.activeSubs.toLocaleString()}
          change="+8%"
          trend="up"
          sparklineData={generateSparkline(stats.activeSubs, "up")}
          sparklineColor="hsl(var(--primary))"
        />
        <KpiCard
          label="MRR"
          value={formatMoney(stats.totalRevenue)}
          change="+23%"
          trend="up"
          sparklineData={generateSparkline(stats.totalRevenue, "up")}
          sparklineColor="hsl(var(--primary))"
        />
        <KpiCard
          label="Churn"
          value={`${stats.churnRate}%`}
          change="-0.3%"
          trend="down"
          sparklineData={generateSparkline(
            Number.parseFloat(stats.churnRate),
            "down",
          )}
          sparklineColor="hsl(0 84% 60%)"
        />
      </div>

      {/* Row 2: Revenue + Plan Distribution + System Status */}
      <div className="grid gap-3 lg:grid-cols-[2fr_1fr_1fr]">
        {/* Revenue Chart */}
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="mb-4 flex items-baseline justify-between">
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
                Revenue
              </p>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="font-mono text-xl font-bold tracking-tight text-foreground">
                  {formatMoney(stats.totalRevenue)}
                </span>
                <span className="rounded bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[11px] font-medium text-emerald-500">
                  +23%
                </span>
              </div>
            </div>
            <span className="font-mono text-[10px] text-muted-foreground">
              Last 6 months
            </span>
          </div>
          <RevenueChart />
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
                  {check.message ? (
                    <span className="font-mono text-[11px] text-muted-foreground">
                      {check.message}
                    </span>
                  ) : null}
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
  change: string;
  trend: "up" | "down";
  sparklineData: number[];
  sparklineColor: string;
};

function KpiCard({
  label,
  value,
  change,
  trend,
  sparklineData,
  sparklineColor,
}: KpiCardProps) {
  const isPositiveTrend = trend === "up";

  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <div className="mt-2 flex items-end justify-between">
        <div>
          <p className="font-mono text-[28px] font-bold leading-none tracking-tight text-foreground">
            {value}
          </p>
          <span
            className={cn(
              "mt-2 inline-block rounded px-1.5 py-0.5 font-mono text-[11px] font-medium",
              isPositiveTrend
                ? "bg-emerald-500/10 text-emerald-500"
                : "bg-red-500/10 text-red-500",
            )}
          >
            {change}
          </span>
        </div>
        <Sparkline
          data={sparklineData}
          color={sparklineColor}
          width={80}
          height={28}
        />
      </div>
    </div>
  );
}
