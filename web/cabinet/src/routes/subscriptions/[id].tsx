import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import {
  ArrowLeft,
  Loader2,
  CreditCard,
  Hash,
  Shield,
  CalendarDays,
  CalendarRange,
  Timer,
  CircleDot,
  RotateCcw,
  Inbox,
  XCircle,
} from "lucide-react";
import {
  useSubscription,
  useCancelSubscription,
  useSubscriptionBindings,
  useInvoices,
  usePlans,
  LoadingSpinner,
  formatDate,
  formatBytes,
  formatMoney,
  cn,
} from "@remnacore/shared";
import type { SubscriptionStatus, InvoiceStatus } from "@remnacore/shared";
import { BindingLinks } from "../../components/BindingLinks.js";

// ─── Status styling ──────────────────────────────────────────────────────────

const STATUS_BADGE: Record<SubscriptionStatus, string> = {
  active: "bg-primary/15 text-primary",
  pending: "bg-yellow-500/15 text-yellow-500",
  cancelled: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
  expired: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
  paused: "bg-[var(--warm)]/15 text-[var(--warm)]",
};

const STATUS_DOT: Record<SubscriptionStatus, string> = {
  active: "bg-primary",
  pending: "bg-yellow-500",
  cancelled: "bg-[rgba(245,236,227,0.24)]",
  expired: "bg-[rgba(245,236,227,0.24)]",
  paused: "bg-[var(--warm)]",
};

const STATUS_GLOW: Record<SubscriptionStatus, string> = {
  active: "shadow-[0_0_20px_rgba(45,212,191,0.15)]",
  pending: "shadow-[0_0_20px_rgba(245,158,11,0.10)]",
  cancelled: "",
  expired: "",
  paused: "shadow-[0_0_20px_rgba(196,149,106,0.10)]",
};

const INVOICE_STYLE: Record<InvoiceStatus, string> = {
  paid: "bg-primary/15 text-primary",
  pending: "bg-yellow-500/15 text-yellow-500",
  cancelled: "bg-muted text-muted-foreground",
  refunded: "bg-[var(--warm)]/15 text-[var(--warm)]",
};

const TIER_BADGE: Record<string, string> = {
  basic: "bg-surface-2 text-muted-foreground",
  standard: "bg-primary/15 text-primary",
  premium: "bg-[var(--warm)]/15 text-[var(--warm)]",
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

const MS_PER_DAY = 1000 * 60 * 60 * 24;

function daysRemaining(periodEnd: string): number {
  const end = new Date(periodEnd).getTime();
  const now = Date.now();
  const diff = end - now;
  return Math.max(0, Math.ceil(diff / MS_PER_DAY));
}

function trafficPercent(used: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(Math.round((used / limit) * 100), 100);
}

function barColor(percent: number): string {
  if (percent >= 90) return "bg-destructive";
  if (percent >= 70) return "bg-[var(--warm)]";
  return "bg-primary";
}

// ─── Info Cell ───────────────────────────────────────────────────────────────

function InfoCell({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-2">
        {icon}
      </div>
      <div className="min-w-0">
        <p className="text-xs uppercase tracking-wider text-muted-foreground">
          {label}
        </p>
        <div className="mt-0.5">{children}</div>
      </div>
    </div>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

export function SubscriptionDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };

  const { data: subscription, isLoading } = useSubscription(id);
  const { data: allBindings, isLoading: bindingsLoading } =
    useSubscriptionBindings(id);
  const { data: allInvoices, isLoading: invoicesLoading } = useInvoices();
  const { data: plans } = usePlans();
  const cancelMutation = useCancelSubscription();

  const plan = useMemo(() => {
    if (!subscription || !plans) return null;
    return plans.find((p) => p.id === subscription.plan_id) ?? null;
  }, [subscription, plans]);

  const subInvoices = useMemo(() => {
    if (!allInvoices || !subscription) return [];
    return allInvoices
      .filter((inv) => inv.subscription_id === subscription.id)
      .sort(
        (a, b) =>
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
      );
  }, [allInvoices, subscription]);

  const trafficStats = useMemo(() => {
    const bindings = allBindings ?? [];
    const totalUsed = bindings.reduce(
      (sum, b) => sum + b.traffic_limit_bytes,
      0,
    );
    const totalLimit = plan?.traffic_limit_bytes ?? 0;
    return { totalUsed, totalLimit };
  }, [allBindings, plan]);

  const remaining = subscription ? daysRemaining(subscription.period_end) : 0;
  const canCancel = subscription?.status === "active";

  if (isLoading) return <LoadingSpinner />;

  if (!subscription) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      {/* Back link */}
      <Link
        to="/subscriptions"
        className="animate-fade-up inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      {/* Hero card */}
      <div
        className={cn(
          "glass-card relative overflow-hidden p-6 animate-fade-up",
          STATUS_GLOW[subscription.status],
        )}
        style={{ animationDelay: "50ms", animationFillMode: "backwards" }}
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-[-0.03em] text-foreground">
              {plan?.name ?? t("subscriptions.unknownPlan")}
            </h1>
            {plan && (
              <p className="mt-1 text-sm text-muted-foreground">
                {formatMoney(plan.base_price_amount, plan.base_price_currency)}
                <span className="text-tertiary"> / </span>
                {t(`plans.interval.${plan.billing_interval}`)}
              </p>
            )}
          </div>

          <div className="flex items-center gap-2">
            <span className="relative flex h-2.5 w-2.5">
              <span
                className={cn(
                  "absolute inline-flex h-full w-full rounded-full",
                  STATUS_DOT[subscription.status],
                  subscription.status === "active" &&
                    "animate-pulse-glow opacity-75",
                )}
              />
              <span
                className={cn(
                  "relative inline-flex h-2.5 w-2.5 rounded-full",
                  STATUS_DOT[subscription.status],
                )}
              />
            </span>
            <span
              className={cn(
                "rounded-full px-3 py-1 text-xs font-semibold",
                STATUS_BADGE[subscription.status],
              )}
            >
              {t(`subscriptions.status.${subscription.status}`)}
            </span>
          </div>
        </div>

        {/* Subtle gradient wash */}
        <div className="absolute -bottom-16 -right-16 h-40 w-40 rounded-full bg-primary/5 blur-3xl pointer-events-none" />
      </div>

      {/* Info grid */}
      <div
        className="glass-card p-6 animate-fade-up"
        style={{ animationDelay: "100ms", animationFillMode: "backwards" }}
      >
        <div className="grid gap-5 sm:grid-cols-2">
          <InfoCell
            icon={<Hash size={14} className="text-muted-foreground" />}
            label={t("subscriptions.subscriptionId")}
          >
            <p className="font-mono text-sm font-medium text-foreground break-all">
              {subscription.id}
            </p>
          </InfoCell>

          <InfoCell
            icon={<Shield size={14} className="text-muted-foreground" />}
            label={t("subscriptions.planTier")}
          >
            {plan ? (
              <span
                className={cn(
                  "inline-block rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize",
                  TIER_BADGE[plan.tier] ?? TIER_BADGE.basic,
                )}
              >
                {plan.tier}
              </span>
            ) : (
              <span className="text-sm text-muted-foreground">--</span>
            )}
          </InfoCell>

          <InfoCell
            icon={<CalendarDays size={14} className="text-muted-foreground" />}
            label={t("common.createdAt")}
          >
            <p className="font-mono text-sm font-medium text-foreground">
              {formatDate(subscription.created_at)}
            </p>
          </InfoCell>

          <InfoCell
            icon={<CalendarRange size={14} className="text-muted-foreground" />}
            label={t("subscriptions.period")}
          >
            <p className="font-mono text-sm font-medium text-foreground">
              {formatDate(subscription.period_start)}
              <span className="text-tertiary mx-1.5">&rarr;</span>
              {formatDate(subscription.period_end)}
            </p>
          </InfoCell>

          <InfoCell
            icon={<Timer size={14} className="text-muted-foreground" />}
            label={t("subscriptions.daysRemaining")}
          >
            <p
              className={cn(
                "font-mono text-sm font-bold",
                remaining <= 7 ? "text-destructive" : "text-foreground",
              )}
            >
              {remaining} {t("common.days")}
            </p>
          </InfoCell>

          <InfoCell
            icon={<CircleDot size={14} className="text-muted-foreground" />}
            label={t("subscriptions.statusLabel")}
          >
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  STATUS_DOT[subscription.status],
                )}
              />
              <span className="text-sm font-medium text-foreground capitalize">
                {t(`subscriptions.status.${subscription.status}`)}
              </span>
            </div>
          </InfoCell>
        </div>
      </div>

      {/* Traffic section */}
      <div
        className="glass-card p-6 animate-fade-up"
        style={{ animationDelay: "150ms", animationFillMode: "backwards" }}
      >
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold tracking-[-0.03em] text-foreground">
            {t("traffic.title")}
          </h2>
          {plan && (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <RotateCcw size={12} />
              <span>{t("traffic.resetsOnRenewal")}</span>
            </div>
          )}
        </div>

        <div className="flex items-end justify-between mb-2">
          <p className="font-mono text-sm text-foreground">
            {formatBytes(trafficStats.totalUsed, 1)}
            <span className="text-muted-foreground">
              {" "}
              / {formatBytes(trafficStats.totalLimit, 1)}
            </span>
          </p>
          <span className="font-mono text-xs text-muted-foreground">
            {trafficPercent(trafficStats.totalUsed, trafficStats.totalLimit)}%
          </span>
        </div>

        <div className="h-3 overflow-hidden rounded-full bg-surface-2">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-700",
              barColor(
                trafficPercent(
                  trafficStats.totalUsed,
                  trafficStats.totalLimit,
                ),
              ),
            )}
            style={{
              width: `${trafficPercent(trafficStats.totalUsed, trafficStats.totalLimit)}%`,
            }}
          />
        </div>
      </div>

      {/* Bindings section */}
      <div
        className="animate-fade-up space-y-4"
        style={{ animationDelay: "200ms", animationFillMode: "backwards" }}
      >
        <h2 className="text-lg font-bold tracking-[-0.03em] text-foreground">
          {t("bindings.title")}
        </h2>
        {bindingsLoading ? (
          <LoadingSpinner />
        ) : allBindings && allBindings.length > 0 ? (
          <BindingLinks bindings={allBindings} />
        ) : (
          <div className="flex flex-col items-center justify-center rounded-[var(--radius)] border border-dashed border-border bg-card/50 p-10">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-surface-2">
              <Inbox size={18} className="text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">
              {t("bindings.empty")}
            </p>
          </div>
        )}
      </div>

      {/* Invoices section */}
      <div
        className="animate-fade-up space-y-4"
        style={{ animationDelay: "250ms", animationFillMode: "backwards" }}
      >
        <h2 className="text-lg font-bold tracking-[-0.03em] text-foreground">
          {t("invoices.title")}
        </h2>

        {invoicesLoading ? (
          <LoadingSpinner />
        ) : subInvoices.length > 0 ? (
          <div className="glass-card overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left">
                    <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      {t("invoices.id")}
                    </th>
                    <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      {t("invoices.amount")}
                    </th>
                    <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      {t("invoices.date")}
                    </th>
                    <th className="px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                      {t("invoices.statusLabel")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {subInvoices.map((invoice, index) => (
                    <tr
                      key={invoice.id}
                      className={cn(
                        "border-b border-border/50 transition-colors hover:bg-surface-2/50",
                        index === subInvoices.length - 1 && "border-b-0",
                      )}
                    >
                      <td className="px-4 py-3 font-mono text-xs text-foreground">
                        {invoice.id.slice(0, 8)}
                      </td>
                      <td className="px-4 py-3 font-mono text-sm font-medium text-foreground">
                        {formatMoney(invoice.total_amount, invoice.currency)}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-muted-foreground">
                        {formatDate(invoice.created_at)}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={cn(
                            "inline-block rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize",
                            INVOICE_STYLE[invoice.status] ??
                              INVOICE_STYLE.pending,
                          )}
                        >
                          {t(`invoices.status.${invoice.status}`)}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center rounded-[var(--radius)] border border-dashed border-border bg-card/50 p-10">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-surface-2">
              <CreditCard size={18} className="text-muted-foreground" />
            </div>
            <p className="text-sm text-muted-foreground">
              {t("invoices.empty")}
            </p>
          </div>
        )}
      </div>

      {/* Actions */}
      {canCancel && (
        <div
          className="glass-card flex items-center justify-between p-6 animate-fade-up"
          style={{ animationDelay: "300ms", animationFillMode: "backwards" }}
        >
          <div>
            <h3 className="text-sm font-semibold text-foreground">
              {t("subscriptions.cancelSubscription")}
            </h3>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {t("subscriptions.cancelDescription")}
            </p>
          </div>
          <button
            type="button"
            onClick={() => {
              if (window.confirm(t("subscriptions.cancelConfirm"))) {
                cancelMutation.mutate(subscription.id);
              }
            }}
            disabled={cancelMutation.isPending}
            className="flex shrink-0 items-center gap-2 rounded-[10px] border border-destructive px-4 py-2.5 text-sm font-medium text-destructive transition-all hover:bg-destructive/10 disabled:opacity-50"
          >
            {cancelMutation.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <>
                <XCircle size={14} />
                {t("subscriptions.cancel")}
              </>
            )}
          </button>
        </div>
      )}
    </div>
  );
}
