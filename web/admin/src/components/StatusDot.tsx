import { cn } from "@remnacore/shared";

type StatusDotProps = {
  status: "healthy" | "degraded" | "unhealthy";
};

const STATUS_CLASSES = {
  healthy: "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]",
  degraded: "bg-amber-500 shadow-[0_0_6px_rgba(245,158,11,0.6)]",
  unhealthy: "bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)]",
} as const;

export function StatusDot({ status }: StatusDotProps) {
  return (
    <span
      className={cn(
        "inline-block size-1.5 shrink-0 rounded-full",
        STATUS_CLASSES[status],
      )}
    />
  );
}
