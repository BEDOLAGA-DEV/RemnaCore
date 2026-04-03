import { useTranslation } from "react-i18next";
import { cn } from "@remnacore/shared";

const HEALTH_STATES = {
  healthy: "healthy",
  degraded: "degraded",
  down: "down",
} as const;

type HealthState = (typeof HEALTH_STATES)[keyof typeof HEALTH_STATES];

type NodeHealthBadgeProps = {
  health: HealthState;
};

function healthColor(health: HealthState): string {
  const colors: Record<HealthState, string> = {
    healthy: "bg-green-500/10 text-green-500",
    degraded: "bg-yellow-500/10 text-yellow-500",
    down: "bg-red-500/10 text-red-500",
  };
  return colors[health];
}

export function NodeHealthBadge({ health }: NodeHealthBadgeProps) {
  const { t } = useTranslation();

  const labelKey = `admin.nodes.${health}` as const;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
        healthColor(health),
      )}
    >
      <span
        className={cn(
          "h-1.5 w-1.5 rounded-full",
          health === "healthy" && "bg-green-500",
          health === "degraded" && "bg-yellow-500",
          health === "down" && "bg-red-500",
        )}
      />
      {t(labelKey)}
    </span>
  );
}
