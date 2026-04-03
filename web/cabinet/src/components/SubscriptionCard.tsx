import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Calendar, ExternalLink } from "lucide-react";
import { cn, formatDate } from "@remnacore/shared";
import type { Subscription, SubscriptionStatus } from "@remnacore/shared";

type SubscriptionCardProps = {
  subscription: Subscription;
};

function statusColor(status: SubscriptionStatus): string {
  const colors: Record<SubscriptionStatus, string> = {
    active: "bg-green-500/10 text-green-500",
    pending: "bg-yellow-500/10 text-yellow-500",
    cancelled: "bg-red-500/10 text-red-500",
    expired: "bg-gray-500/10 text-gray-500",
    paused: "bg-blue-500/10 text-blue-500",
  };
  return colors[status];
}

export function SubscriptionCard({ subscription }: SubscriptionCardProps) {
  const { t } = useTranslation();

  return (
    <div className="rounded-xl border border-border bg-card p-5 shadow-sm transition-all hover:shadow-md">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-semibold text-foreground font-mono text-sm">
            {subscription.id.slice(0, 8)}...
          </h3>
          <span
            className={cn(
              "mt-1 inline-block rounded-full px-2.5 py-0.5 text-xs font-medium",
              statusColor(subscription.status),
            )}
          >
            {t(`subscriptions.status.${subscription.status}`)}
          </span>
        </div>
        <Link
          to="/subscriptions/$id"
          params={{ id: subscription.id }}
          className="rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
        >
          <ExternalLink size={16} />
        </Link>
      </div>

      <div className="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
        <Calendar size={14} />
        <span>
          {t("subscriptions.periodEnd")}:{" "}
          {formatDate(subscription.period_end)}
        </span>
      </div>
    </div>
  );
}
