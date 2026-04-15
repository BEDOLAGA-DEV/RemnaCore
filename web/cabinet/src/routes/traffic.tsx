import { useTranslation } from "react-i18next";
import { useBindings, LoadingSpinner, formatBytes } from "@remnacore/shared";
import { TrafficChart } from "../components/TrafficChart.js";

export function TrafficPage() {
  const { t } = useTranslation();
  const { data: bindings, isLoading, isError } = useBindings();

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  const allBindings = bindings ?? [];
  const totalTraffic = allBindings.reduce(
    (sum, b) => sum + b.traffic_limit_bytes,
    0,
  );

  return (
    <div className="space-y-6">
      <h1
        className="animate-fade-up text-3xl font-bold tracking-tight text-foreground"
      >
        {t("traffic.title")}
      </h1>

      {/* Summary cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        <div
          className="animate-fade-up rounded-lg border border-border bg-card p-5"
          style={{ animationDelay: "50ms", animationFillMode: "backwards" }}
        >
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {t("traffic.total")}
          </p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-foreground">
            {formatBytes(totalTraffic)}
          </p>
        </div>
        <div
          className="animate-fade-up rounded-lg border border-border bg-card p-5"
          style={{ animationDelay: "100ms", animationFillMode: "backwards" }}
        >
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {t("bindings.title")}
          </p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-foreground">
            {allBindings.length}
          </p>
        </div>
        <div
          className="animate-fade-up rounded-lg border border-border bg-card p-5"
          style={{ animationDelay: "150ms", animationFillMode: "backwards" }}
        >
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {t("bindings.status.synced")}
          </p>
          <p className="mt-1.5 font-mono text-2xl font-bold text-foreground">
            {allBindings.filter((b) => b.status === "synced").length}
          </p>
        </div>
      </div>

      {/* Traffic chart */}
      <div
        className="animate-fade-up"
        style={{ animationDelay: "200ms", animationFillMode: "backwards" }}
      >
        <TrafficChart bindings={allBindings} />
      </div>
    </div>
  );
}
