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
      <h1 className="text-2xl font-bold text-foreground">
        {t("traffic.title")}
      </h1>

      {/* Summary cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="text-sm text-muted-foreground">{t("traffic.total")}</p>
          <p className="mt-1 text-2xl font-bold text-foreground">
            {formatBytes(totalTraffic)}
          </p>
        </div>
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="text-sm text-muted-foreground">
            {t("bindings.title")}
          </p>
          <p className="mt-1 text-2xl font-bold text-foreground">
            {allBindings.length}
          </p>
        </div>
        <div className="rounded-xl border border-border bg-card p-5">
          <p className="text-sm text-muted-foreground">
            {t("bindings.status.synced")}
          </p>
          <p className="mt-1 text-2xl font-bold text-foreground">
            {allBindings.filter((b) => b.status === "synced").length}
          </p>
        </div>
      </div>

      {/* Traffic chart */}
      <TrafficChart bindings={allBindings} />
    </div>
  );
}
