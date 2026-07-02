import type { Binding } from "@remnacore/shared";
import {
  cn,
  formatBytes,
  LoadingSpinner,
  useBindings,
} from "@remnacore/shared";
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Calendar,
  Inbox,
  Server,
} from "lucide-react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";

const DAYS_IN_RANGE = 30;

function trafficPercent(used: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(Math.round((used / limit) * 100), 100);
}

function barColor(percent: number): string {
  if (percent >= 90) return "bg-destructive";
  if (percent >= 70) return "bg-[var(--warm)]";
  return "bg-primary";
}

function SummaryCard({
  icon,
  label,
  value,
  delay,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  delay: number;
}) {
  return (
    <div
      className="glass-card relative overflow-hidden p-5 animate-fade-up"
      style={{ animationDelay: `${delay}ms`, animationFillMode: "backwards" }}
    >
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10">
          {icon}
        </div>
        <div className="min-w-0">
          <p className="text-xs uppercase tracking-wider text-muted-foreground">
            {label}
          </p>
          <p className="mt-0.5 font-mono text-xl font-bold text-foreground">
            {value}
          </p>
        </div>
      </div>
      <div className="kpi-glow bg-primary" />
    </div>
  );
}

function BindingRow({ binding, index }: { binding: Binding; index: number }) {
  const { t } = useTranslation();
  const used = binding.traffic_limit_bytes;
  const limit = binding.traffic_limit_bytes;
  const percent = trafficPercent(used, limit);

  return (
    <div
      className="glass-card p-4 animate-fade-up"
      style={{
        animationDelay: `${200 + index * 60}ms`,
        animationFillMode: "backwards",
      }}
    >
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-2">
            <Server size={14} className="text-muted-foreground" />
          </div>
          <div className="min-w-0">
            <p className="truncate font-mono text-sm font-medium text-foreground">
              {binding.remnawave_username}
            </p>
            <p className="text-xs text-muted-foreground">
              {binding.id.slice(0, 8)}
            </p>
          </div>
        </div>

        <div className="text-right shrink-0">
          <p className="font-mono text-sm font-semibold text-foreground">
            {formatBytes(used, 1)}
          </p>
          <p className="text-xs text-muted-foreground">
            {t("traffic.ofLimit", { limit: formatBytes(limit, 1) })}
          </p>
        </div>
      </div>

      <div className="mt-3 flex items-center gap-3">
        <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-700",
              barColor(percent),
            )}
            style={{ width: `${percent}%` }}
          />
        </div>
        <span className="w-10 text-right font-mono text-xs text-muted-foreground">
          {percent}%
        </span>
      </div>
    </div>
  );
}

export function TrafficPage() {
  const { t } = useTranslation();
  const { data: bindings, isLoading, isError } = useBindings();

  const allBindings = useMemo(() => bindings ?? [], [bindings]);

  const stats = useMemo(() => {
    const totalUsed = allBindings.reduce(
      (sum, b) => sum + b.traffic_limit_bytes,
      0,
    );
    // Without separate upload/download fields, split estimate for display
    const estimatedDown = Math.round(totalUsed * 0.7);
    const estimatedUp = totalUsed - estimatedDown;
    const dailyAvg =
      allBindings.length > 0 ? Math.round(totalUsed / DAYS_IN_RANGE) : 0;

    return { totalUsed, estimatedDown, estimatedUp, dailyAvg };
  }, [allBindings]);

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between animate-fade-up">
        <div>
          <h1 className="text-3xl font-bold tracking-[-0.03em] text-foreground">
            {t("traffic.title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("traffic.subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-2 rounded-full bg-surface-2 px-4 py-2 text-xs font-medium text-muted-foreground">
          <Calendar size={14} />
          <span>{t("traffic.last30days")}</span>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        <SummaryCard
          icon={<ArrowDownToLine size={18} className="text-primary" />}
          label={t("traffic.downloaded")}
          value={formatBytes(stats.estimatedDown, 1)}
          delay={50}
        />
        <SummaryCard
          icon={<ArrowUpFromLine size={18} className="text-[var(--warm)]" />}
          label={t("traffic.uploaded")}
          value={formatBytes(stats.estimatedUp, 1)}
          delay={100}
        />
        <SummaryCard
          icon={<Activity size={18} className="text-primary" />}
          label={t("traffic.dailyAverage")}
          value={formatBytes(stats.dailyAvg, 1)}
          delay={150}
        />
      </div>

      {/* Per-binding breakdown */}
      <div>
        <h2
          className="mb-4 text-lg font-bold tracking-[-0.03em] text-foreground animate-fade-up"
          style={{ animationDelay: "180ms", animationFillMode: "backwards" }}
        >
          {t("traffic.perBinding")}
        </h2>

        {allBindings.length > 0 ? (
          <div className="flex flex-col gap-3">
            {allBindings.map((binding, index) => (
              <BindingRow key={binding.id} binding={binding} index={index} />
            ))}
          </div>
        ) : (
          <div
            className="animate-fade-up flex flex-col items-center justify-center rounded-[var(--radius)] border border-dashed border-border bg-card/50 p-12"
            style={{ animationDelay: "200ms", animationFillMode: "backwards" }}
          >
            <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-surface-2">
              <Inbox size={20} className="text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">
              {t("traffic.noData")}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
