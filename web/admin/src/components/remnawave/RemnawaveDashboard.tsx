import { apiGet, cn, LoadingSpinner } from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowDownToLine,
  ArrowUpFromLine,
  Globe,
  Server,
  Users,
  Wifi,
} from "lucide-react";
import { StatusDot } from "../StatusDot.js";

// ─── Constants ──────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i] ?? "TB"}`;
}

const OVERVIEW_STALE_MS = 30_000;
const REALTIME_STALE_MS = 10_000;
const REALTIME_REFETCH_MS = 15_000;

// ─── Types ──────────────────────────────────────────────────────────────────

type OverviewBandwidth = {
  upload: number;
  download: number;
  total: number;
};

type OverviewResponse = {
  panels_up: number;
  panels_total: number;
  nodes_healthy: number;
  nodes_total: number;
  active_users: number;
  bandwidth: OverviewBandwidth;
};

type PanelMetrics = {
  panel_id: string;
  slug?: string;
  online?: boolean;
  metrics: Record<string, unknown> | null;
};

type RealtimeResponse = {
  panels: PanelMetrics[];
};

// ─── Query Keys ─────────────────────────────────────────────────────────────

const QUERY_KEY_OVERVIEW = ["remnawave", "dashboard", "overview"] as const;
const QUERY_KEY_REALTIME = ["remnawave", "dashboard", "realtime"] as const;

// ─── Hooks ──────────────────────────────────────────────────────────────────

function useRemnawaveOverview() {
  return useQuery<OverviewResponse>({
    queryKey: [...QUERY_KEY_OVERVIEW],
    queryFn: () => apiGet<OverviewResponse>("/api/remnawave/dashboard/overview"),
    staleTime: OVERVIEW_STALE_MS,
  });
}

function useRemnawaveRealtime() {
  return useQuery<RealtimeResponse>({
    queryKey: [...QUERY_KEY_REALTIME],
    queryFn: () => apiGet<RealtimeResponse>("/api/remnawave/dashboard/realtime"),
    staleTime: REALTIME_STALE_MS,
    refetchInterval: REALTIME_REFETCH_MS,
  });
}

// ─── Dashboard Component ────────────────────────────────────────────────────

export default function RemnawaveDashboard() {
  const {
    data: overview,
    isLoading: overviewLoading,
    isError: overviewError,
    error: overviewErr,
  } = useRemnawaveOverview();

  const {
    data: realtime,
    isLoading: realtimeLoading,
    isError: realtimeError,
  } = useRemnawaveRealtime();

  if (overviewLoading || realtimeLoading) {
    return <LoadingSpinner />;
  }

  if (overviewError) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-16">
        <AlertTriangle size={32} className="text-destructive" />
        <p className="text-[13px] text-destructive">
          Failed to load Remnawave dashboard
        </p>
        <p className="max-w-md text-center font-mono text-[11px] text-muted-foreground">
          {overviewErr instanceof Error
            ? overviewErr.message
            : "An unexpected error occurred"}
        </p>
      </div>
    );
  }

  const stats = overview ?? {
    panels_up: 0,
    panels_total: 0,
    nodes_healthy: 0,
    nodes_total: 0,
    active_users: 0,
    bandwidth: { upload: 0, download: 0, total: 0 },
  };

  const panels = realtime?.panels ?? [];

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight text-foreground">
          Remnawave
        </h1>
        <span className="font-mono text-[11px] text-muted-foreground">
          {new Date().toLocaleDateString("en-US", {
            weekday: "short",
            month: "short",
            day: "numeric",
          })}
        </span>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={Server}
          iconBg="bg-blue-500/10"
          iconColor="text-blue-500"
          label="Panels"
          value={`${stats.panels_up}/${stats.panels_total}`}
          highlight={stats.panels_up < stats.panels_total}
        />
        <StatCard
          icon={Wifi}
          iconBg="bg-emerald-500/10"
          iconColor="text-emerald-500"
          label="Nodes Healthy"
          value={`${stats.nodes_healthy}/${stats.nodes_total}`}
          highlight={stats.nodes_healthy < stats.nodes_total}
        />
        <StatCard
          icon={Users}
          iconBg="bg-violet-500/10"
          iconColor="text-violet-500"
          label="Active Users"
          value={stats.active_users.toLocaleString()}
        />
        <StatCard
          icon={Globe}
          iconBg="bg-amber-500/10"
          iconColor="text-amber-500"
          label="Bandwidth Today"
          value={formatBytes(stats.bandwidth.total)}
        />
      </div>

      {/* Bandwidth Breakdown */}
      <div className="rounded-xl border border-border bg-card p-5">
        <p className="mb-4 text-[11px] uppercase tracking-wider text-muted-foreground">
          Bandwidth Breakdown
        </p>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-500/10">
              <ArrowUpFromLine size={16} className="text-emerald-500" />
            </div>
            <div>
              <p className="text-[11px] text-muted-foreground">Upload</p>
              <p className="font-mono text-[15px] font-semibold text-foreground">
                {formatBytes(stats.bandwidth.upload)}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-500/10">
              <ArrowDownToLine size={16} className="text-blue-500" />
            </div>
            <div>
              <p className="text-[11px] text-muted-foreground">Download</p>
              <p className="font-mono text-[15px] font-semibold text-foreground">
                {formatBytes(stats.bandwidth.download)}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Panel Status */}
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="mb-4 flex items-center gap-2">
          <p className="text-[11px] uppercase tracking-wider text-muted-foreground">
            Panel Status
          </p>
          {!realtimeError && (
            <span className="flex items-center gap-1.5">
              <span className="inline-block size-1.5 animate-pulse rounded-full bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]" />
              <span className="font-mono text-[10px] text-emerald-500">
                live
              </span>
            </span>
          )}
        </div>

        {realtimeError ? (
          <div className="flex items-center gap-2 py-4">
            <AlertTriangle size={14} className="text-amber-500" />
            <p className="text-[12px] text-muted-foreground">
              Unable to load realtime panel data
            </p>
          </div>
        ) : panels.length > 0 ? (
          <div className="space-y-2">
            {panels.map((panel) => (
              <PanelRow key={panel.panel_id} panel={panel} />
            ))}
          </div>
        ) : (
          <p className="py-4 text-center font-mono text-xs text-muted-foreground">
            {stats.panels_total > 0
              ? "Realtime metrics unavailable"
              : "No panels configured"}
          </p>
        )}

        {stats.panels_total > 0 && (
          <div className="mt-4 border-t border-border pt-3">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-muted-foreground">
                Total panels
              </span>
              <span className="font-mono text-sm font-semibold text-foreground">
                {stats.panels_up} / {stats.panels_total} online
              </span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Stat Card ──────────────────────────────────────────────────────────────

type StatCardProps = {
  icon: React.ComponentType<{ size: number; className?: string }>;
  iconBg: string;
  iconColor: string;
  label: string;
  value: string;
  highlight?: boolean;
};

function StatCard({
  icon: Icon,
  iconBg,
  iconColor,
  label,
  value,
  highlight,
}: StatCardProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex h-10 w-10 items-center justify-center rounded-lg",
            iconBg,
          )}
        >
          <Icon size={20} className={iconColor} />
        </div>
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {label}
          </p>
          <p
            className={cn(
              "text-[22px] font-bold text-foreground",
              highlight && "text-amber-500",
            )}
          >
            {value}
          </p>
        </div>
      </div>
    </div>
  );
}

// ─── Panel Row ──────────────────────────────────────────────────────────────

type PanelRowProps = {
  panel: PanelMetrics;
};

function PanelRow({ panel }: PanelRowProps) {
  const isUp = panel.online !== false;
  const label = panel.slug || panel.panel_id;

  return (
    <div className="flex items-center justify-between rounded-lg bg-secondary p-3">
      <div className="flex items-center gap-2.5">
        <StatusDot status={isUp ? "healthy" : "unhealthy"} />
        <span className="font-mono text-xs text-foreground">
          {label}
        </span>
      </div>
      <span
        className={cn(
          "rounded px-1.5 py-0.5 font-mono text-[10px] font-medium",
          isUp
            ? "bg-emerald-500/10 text-emerald-500"
            : "bg-red-500/10 text-red-500",
        )}
      >
        {isUp ? "online" : "offline"}
      </span>
    </div>
  );
}
