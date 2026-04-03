import type { LucideIcon } from "lucide-react";
import { cn } from "@remnacore/shared";

type StatsCardProps = {
  label: string;
  value: string | number;
  icon: LucideIcon;
  colorClass?: string;
};

export function StatsCard({
  label,
  value,
  icon: Icon,
  colorClass = "text-primary bg-primary/10",
}: StatsCardProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <div className="flex items-center gap-4">
        <div className={cn("rounded-lg p-3", colorClass)}>
          <Icon size={20} />
        </div>
        <div>
          <p className="text-2xl font-bold text-foreground">{value}</p>
          <p className="text-sm text-muted-foreground">{label}</p>
        </div>
      </div>
    </div>
  );
}
