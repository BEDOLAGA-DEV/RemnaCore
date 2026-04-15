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
    active: "bg-[rgba(45,212,191,0.10)] text-[#2dd4bf]",
    pending: "bg-[rgba(245,158,11,0.10)] text-[#f59e0b]",
    cancelled: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
    expired: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
    paused: "bg-[rgba(196,149,106,0.10)] text-[#c4956a]",
  };
  return colors[status];
}

function statusDotColor(status: SubscriptionStatus): string {
  const colors: Record<SubscriptionStatus, string> = {
    active: "bg-[#2dd4bf]",
    pending: "bg-[#f59e0b]",
    cancelled: "bg-[rgba(245,236,227,0.24)]",
    expired: "bg-[rgba(245,236,227,0.24)]",
    paused: "bg-[#c4956a]",
  };
  return colors[status];
}

function cardGradient(status: SubscriptionStatus): string {
  if (status === "active") {
    return "bg-gradient-to-br from-[rgba(45,212,191,0.03)] to-transparent";
  }
  if (status === "pending") {
    return "bg-gradient-to-br from-[rgba(245,158,11,0.03)] to-transparent";
  }
  if (status === "paused") {
    return "bg-gradient-to-br from-[rgba(196,149,106,0.03)] to-transparent";
  }
  return "";
}

export function SubscriptionCard({ subscription }: SubscriptionCardProps) {
  const { t } = useTranslation();

  return (
    <div
      className={cn(
        "relative rounded-[16px] border border-border bg-card p-5 transition-all hover:translate-y-[-1px] hover:shadow-lg",
        cardGradient(subscription.status),
      )}
    >
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-mono text-sm font-semibold text-foreground">
            {subscription.id.slice(0, 8)}...
          </h3>
          <div className="mt-1.5 flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              <span
                className={cn(
                  "absolute inline-flex h-full w-full rounded-full",
                  statusDotColor(subscription.status),
                  subscription.status === "active" && "animate-ping opacity-75",
                )}
              />
              <span
                className={cn(
                  "relative inline-flex h-2 w-2 rounded-full",
                  statusDotColor(subscription.status),
                )}
              />
            </span>
            <span
              className={cn(
                "inline-block rounded-full px-2.5 py-0.5 text-xs font-medium",
                statusColor(subscription.status),
              )}
            >
              {t(`subscriptions.status.${subscription.status}`)}
            </span>
          </div>
        </div>
        <Link
          to="/subscriptions/$id"
          params={{ id: subscription.id }}
          className="rounded-[10px] p-2 text-muted-foreground transition-all hover:bg-[rgba(245,236,227,0.06)] hover:text-foreground"
        >
          <ExternalLink size={16} />
        </Link>
      </div>

      <div className="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
        <Calendar size={14} />
        <span className="font-mono">
          {t("subscriptions.periodEnd")}:{" "}
          {formatDate(subscription.period_end)}
        </span>
      </div>
    </div>
  );
}
