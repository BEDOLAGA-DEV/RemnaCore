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
    active: "bg-[rgba(45,212,191,0.10)] text-[#2dd4bf]",
    pending: "bg-[rgba(245,158,11,0.10)] text-[#f59e0b]",
    cancelled: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
    expired: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
    paused: "bg-[rgba(196,149,106,0.10)] text-[#c4956a]",
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
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div
        className="animate-fade-up rounded-lg border border-border bg-card p-6"
      >
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-bold tracking-[-0.03em] text-foreground">
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
            <p className="mt-0.5 font-mono text-sm font-medium text-foreground">
              {formatDate(subscription.period_end)}
            </p>
          </div>
          <div>
            <span className="text-xs text-muted-foreground">
              {t("common.createdAt")}
            </span>
            <p className="mt-0.5 font-mono text-sm font-medium text-foreground">
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
              className="rounded-[10px] border border-destructive px-4 py-2 text-sm font-medium text-destructive transition-all hover:bg-destructive/10 disabled:opacity-50"
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
      <div
        className="animate-fade-up space-y-4"
        style={{ animationDelay: "100ms" }}
      >
        <h2 className="text-lg font-bold tracking-[-0.03em] text-foreground">
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
