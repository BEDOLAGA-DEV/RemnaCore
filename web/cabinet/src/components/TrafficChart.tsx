import type { Binding } from "@remnacore/shared";
import { formatBytes } from "@remnacore/shared";
import { useTranslation } from "react-i18next";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

type TrafficChartProps = {
  bindings: Binding[];
};

export function TrafficChart({ bindings }: TrafficChartProps) {
  const { t } = useTranslation();

  if (bindings.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-lg border border-dashed border-border bg-card">
        <p className="text-sm text-muted-foreground">{t("traffic.noData")}</p>
      </div>
    );
  }

  // Build chart data from binding traffic limits
  const chartData = bindings.map((binding) => ({
    name: binding.remnawave_username,
    traffic: binding.traffic_limit_bytes,
  }));

  return (
    <div className="rounded-lg border border-border bg-card p-6">
      <h3 className="mb-4 text-base font-semibold tracking-tight text-foreground">
        {t("traffic.title")}
      </h3>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={chartData}>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="rgba(255,235,210,0.06)"
          />
          <XAxis
            dataKey="name"
            className="text-xs"
            tick={{ fill: "var(--muted-foreground)" }}
          />
          <YAxis
            tickFormatter={(value: number) => formatBytes(value)}
            className="text-xs"
            tick={{ fill: "var(--muted-foreground)" }}
          />
          <Tooltip
            formatter={(value) => [
              formatBytes(Number(value)),
              t("traffic.used"),
            ]}
            contentStyle={{
              backgroundColor: "var(--card)",
              border: "1px solid var(--border)",
              borderRadius: "10px",
            }}
            labelStyle={{ color: "var(--foreground)" }}
          />
          <Area
            type="monotone"
            dataKey="traffic"
            stroke="var(--primary)"
            fill="var(--primary)"
            fillOpacity={0.1}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
