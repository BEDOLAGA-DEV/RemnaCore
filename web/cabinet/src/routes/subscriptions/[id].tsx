import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Loader2 } from "lucide-react";
import {
  useSubscription,
  useCancelSubscription,
  useSubscriptionBindings,
  LoadingSpinner,
  formatDate,
  cn,
} from "@remnacore/shared";
import type { SubscriptionStatus } from "@remnacore/shared";
import { BindingLinks } from "../../components/BindingLinks.js";

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

export function SubscriptionDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: subscription, isLoading } = useSubscription(id);
  const { data: bindings, isLoading: bindingsLoading } =
    useSubscriptionBindings(id);
  const cancelMutation = useCancelSubscription();

  if (isLoading) return <LoadingSpinner />;

  if (!subscription) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  const canCancel = subscription.status === "active";

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
              {t("common.details")}
            </h1>
            <p className="mt-1 font-mono text-sm text-muted-foreground">
              {subscription.id}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium",
              statusColor(subscription.status),
            )}
          >
            {t(`subscriptions.status.${subscription.status}`)}
          </span>
        </div>

        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <span className="text-xs text-muted-foreground">
              {t("subscriptions.periodEnd")}
            </span>
            <p className="text-sm font-medium text-foreground">
              {formatDate(subscription.period_end)}
            </p>
          </div>
          <div>
            <span className="text-xs text-muted-foreground">
              {t("common.createdAt")}
            </span>
            <p className="text-sm font-medium text-foreground">
              {formatDate(subscription.created_at)}
            </p>
          </div>
        </div>

        {canCancel && (
          <div className="mt-6">
            <button
              type="button"
              onClick={() => {
                if (window.confirm(t("subscriptions.cancelConfirm"))) {
                  cancelMutation.mutate(subscription.id);
                }
              }}
              disabled={cancelMutation.isPending}
              className="rounded-lg border border-destructive px-4 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
            >
              {cancelMutation.isPending ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                t("subscriptions.cancelSubscription")
              )}
            </button>
          </div>
        )}
      </div>

      {/* Bindings */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">
          {t("bindings.title")}
        </h2>
        {bindingsLoading ? (
          <LoadingSpinner />
        ) : (
          <BindingLinks bindings={bindings ?? []} />
        )}
      </div>
    </div>
  );
}
