import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useSubscription, LoadingSpinner, formatDate, cn } from "@remnacore/shared";
import type { SubscriptionStatus } from "@remnacore/shared";

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

export function AdminSubscriptionDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: sub, isLoading } = useSubscription(id);

  if (isLoading) return <LoadingSpinner />;

  if (!sub) {
    return (
      <div className="text-center py-12">
        <p className="text-[12px] text-red-500">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/subscriptions"
        className="text-[13px] text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-[18px] font-semibold text-foreground">
              {t("admin.subscriptions.title")}
            </h1>
            <p className="mt-1 font-mono text-[12px] text-muted-foreground">
              {sub.id}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
              statusColor(sub.status),
            )}
          >
            {sub.status}
          </span>
        </div>

        <div className="mt-6 grid gap-5 sm:grid-cols-2">
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("admin.subscriptions.userId")}
            </p>
            <Link
              to="/users/$id"
              params={{ id: sub.user_id }}
              className="font-mono text-[12px] text-primary hover:text-primary/80 transition-colors"
            >
              {sub.user_id}
            </Link>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("admin.subscriptions.planId")}
            </p>
            <p className="font-mono text-[12px] text-foreground">{sub.plan_id}</p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("subscriptions.periodEnd")}
            </p>
            <p className="font-mono text-[13px] text-foreground">
              {formatDate(sub.period_end)}
            </p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("common.createdAt")}
            </p>
            <p className="font-mono text-[13px] text-foreground">
              {formatDate(sub.created_at)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
