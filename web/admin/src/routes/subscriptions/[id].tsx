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
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/subscriptions"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-bold text-foreground">
              {t("admin.subscriptions.title")}
            </h1>
            <p className="mt-1 font-mono text-sm text-muted-foreground">
              {sub.id}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium",
              statusColor(sub.status),
            )}
          >
            {sub.status}
          </span>
        </div>

        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <p className="text-xs text-muted-foreground">
              {t("admin.subscriptions.userId")}
            </p>
            <Link
              to="/users/$id"
              params={{ id: sub.user_id }}
              className="font-mono text-sm text-primary hover:underline"
            >
              {sub.user_id}
            </Link>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">
              {t("admin.subscriptions.planId")}
            </p>
            <p className="font-mono text-sm text-foreground">{sub.plan_id}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">
              {t("subscriptions.periodEnd")}
            </p>
            <p className="text-sm font-medium text-foreground">
              {formatDate(sub.period_end)}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">
              {t("common.createdAt")}
            </p>
            <p className="text-sm font-medium text-foreground">
              {formatDate(sub.created_at)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
