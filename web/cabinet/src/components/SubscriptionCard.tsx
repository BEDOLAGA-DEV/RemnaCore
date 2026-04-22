import { useState, useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  ChevronDown,
  Shield,
  Crown,
  Zap,
  Smartphone,
  ArrowUpRight,
  Calendar,
  Clock,
  Activity,
} from "lucide-react";
import {
  cn,
  formatDate,
  formatBytes,
  formatMoney,
  usePlans,
} from "@remnacore/shared";
import type {
  Subscription,
  SubscriptionStatus,
  PlanTier,
  BillingInterval,
  PlanAddon,
} from "@remnacore/shared";

// ─── Constants ─────────────────────────────────────────────────────────────

const STATUS_BADGE_STYLES: Record<SubscriptionStatus, string> = {
  active: "bg-[rgba(45,212,191,0.10)] text-[#2dd4bf]",
  pending: "bg-[rgba(245,158,11,0.10)] text-[#f59e0b]",
  cancelled: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
  expired: "bg-[rgba(245,236,227,0.06)] text-[rgba(245,236,227,0.24)]",
  paused: "bg-[rgba(196,149,106,0.10)] text-[#c4956a]",
};

const STATUS_DOT_STYLES: Record<SubscriptionStatus, string> = {
  active: "bg-[#2dd4bf]",
  pending: "bg-[#f59e0b]",
  cancelled: "bg-[rgba(245,236,227,0.24)]",
  expired: "bg-[rgba(245,236,227,0.24)]",
  paused: "bg-[#c4956a]",
};

const STATUS_BORDER_STYLES: Record<SubscriptionStatus, string> = {
  active:
    "border-[rgba(45,212,191,0.15)] shadow-[0_0_20px_rgba(45,212,191,0.06)]",
  pending: "border-[rgba(245,158,11,0.12)]",
  cancelled: "border-border opacity-60",
  expired: "border-border opacity-60",
  paused: "border-[rgba(196,149,106,0.12)]",
};

const STATUS_GRADIENT_STYLES: Record<SubscriptionStatus, string> = {
  active: "bg-gradient-to-br from-[rgba(45,212,191,0.04)] to-transparent",
  pending: "bg-gradient-to-br from-[rgba(245,158,11,0.04)] to-transparent",
  cancelled: "",
  expired: "",
  paused: "bg-gradient-to-br from-[rgba(196,149,106,0.03)] to-transparent",
};

const TIER_ICONS: Record<PlanTier, typeof Shield> = {
  basic: Shield,
  standard: Zap,
  premium: Crown,
};

const MS_PER_DAY = 86_400_000;

// ─── Helpers ───────────────────────────────────────────────────────────────

function intervalLabel(
  interval: BillingInterval,
  t: (key: string) => string,
): string {
  const labels: Record<BillingInterval, string> = {
    monthly: t("plans.perMonth"),
    quarterly: t("plans.perQuarter"),
    yearly: t("plans.perYear"),
  };
  return labels[interval];
}

function daysRemaining(periodEnd: string): number {
  const diff = new Date(periodEnd).getTime() - Date.now();
  return Math.max(0, Math.ceil(diff / MS_PER_DAY));
}

function trafficPercentage(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(100, Math.round((used / total) * 100));
}

// ─── Sub-components ────────────────────────────────────────────────────────

function StatusDot({ status }: { status: SubscriptionStatus }) {
  const isAlive = status === "active";
  return (
    <span className="relative flex h-2 w-2">
      {isAlive && (
        <span
          className={cn(
            "absolute inline-flex h-full w-full rounded-full opacity-75 animate-pulse-glow",
            STATUS_DOT_STYLES[status],
          )}
        />
      )}
      <span
        className={cn(
          "relative inline-flex h-2 w-2 rounded-full",
          STATUS_DOT_STYLES[status],
        )}
      />
    </span>
  );
}

function StatusBadge({ status }: { status: SubscriptionStatus }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2">
      <StatusDot status={status} />
      <span
        className={cn(
          "inline-block rounded-full px-2.5 py-0.5 text-xs font-medium",
          STATUS_BADGE_STYLES[status],
        )}
      >
        {t(`subscriptions.status.${status}`)}
      </span>
    </div>
  );
}

function MiniStat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Activity;
  label: string;
  value: string;
}) {
  return (
    <div className="flex flex-col gap-1 rounded-[10px] bg-[rgba(245,236,227,0.03)] px-3 py-2">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <Icon size={12} />
        <span className="text-[11px] font-medium uppercase tracking-wider">
          {label}
        </span>
      </div>
      <span className="font-mono text-sm font-semibold text-foreground">
        {value}
      </span>
    </div>
  );
}

function TrafficBar({
  usedBytes,
  totalBytes,
}: {
  usedBytes: number;
  totalBytes: number;
}) {
  const { t } = useTranslation();
  const pct = trafficPercentage(usedBytes, totalBytes);
  const barColor =
    pct >= 90
      ? "bg-destructive"
      : pct >= 70
        ? "bg-[#f59e0b]"
        : "bg-primary";

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{t("plans.traffic")}</span>
        <span className="font-mono text-foreground">
          {formatBytes(usedBytes)} / {formatBytes(totalBytes)}
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-[rgba(245,236,227,0.06)]">
        <div
          className={cn("h-full rounded-full transition-all duration-500", barColor)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{pct}% {t("traffic.used").toLowerCase()}</span>
        <span>
          {formatBytes(Math.max(0, totalBytes - usedBytes))}{" "}
          {t("traffic.remaining").toLowerCase()}
        </span>
      </div>
    </div>
  );
}

function DeviceSlots({
  connected,
  total,
}: {
  connected: number;
  total: number;
}) {
  const { t } = useTranslation();
  const slots = Array.from({ length: total }, (_, i) => i < connected);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{t("plans.devices")}</span>
        <span className="font-mono text-foreground">
          {connected} / {total}
        </span>
      </div>
      <div className="flex flex-wrap gap-2">
        {slots.map((isActive, i) => (
          <div
            key={i}
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-full border transition-colors",
              isActive
                ? "border-primary/30 bg-primary/10 text-primary"
                : "border-border bg-[rgba(245,236,227,0.03)] text-[rgba(245,236,227,0.15)]",
            )}
          >
            <Smartphone size={14} />
          </div>
        ))}
      </div>
    </div>
  );
}

function PendingBanner({ daysLeft }: { daysLeft: number }) {
  const { t } = useTranslation();
  return (
    <div className="overflow-hidden rounded-[10px] bg-gradient-to-r from-[rgba(245,158,11,0.15)] via-[rgba(245,158,11,0.08)] to-[rgba(245,158,11,0.15)] animate-shimmer px-4 py-2.5">
      <div className="flex items-center gap-2">
        <Clock size={14} className="text-[#f59e0b]" />
        <span className="text-sm font-medium text-[#f59e0b]">
          {t("subscriptions.status.pending")} — {daysLeft}{" "}
          {daysLeft === 1 ? "day" : "days"} {t("traffic.remaining").toLowerCase()}
        </span>
      </div>
    </div>
  );
}

function AddonChip({ addon }: { addon: PlanAddon }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-[rgba(245,236,227,0.03)] px-3 py-1 text-xs font-medium text-foreground">
      <Zap size={10} className="text-primary" />
      {addon.name}
    </span>
  );
}

// ─── Main Component ────────────────────────────────────────────────────────

type SubscriptionCardProps = {
  subscription: Subscription;
};

export function SubscriptionCard({ subscription }: SubscriptionCardProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const { data: plans } = usePlans();

  const plan = useMemo(
    () => plans?.find((p) => p.id === subscription.plan_id) ?? null,
    [plans, subscription.plan_id],
  );

  const activeAddons = useMemo(() => {
    if (!plan?.addons || subscription.addon_ids.length === 0) return [];
    return plan.addons.filter((a) => subscription.addon_ids.includes(a.id));
  }, [plan, subscription.addon_ids]);

  const daysLeft = daysRemaining(subscription.period_end);
  const TierIcon = plan ? TIER_ICONS[plan.tier] : Shield;

  // Placeholder: traffic used would come from bindings/usage API
  const trafficUsedBytes = 0;
  const trafficTotalBytes = plan?.traffic_limit_bytes ?? 0;

  // Placeholder: connected devices would come from bindings API
  const devicesConnected = 0;
  const deviceLimit = plan?.device_limit ?? 0;

  const toggle = () => setExpanded((prev) => !prev);

  return (
    <div
      className={cn(
        "glass-card overflow-hidden transition-all duration-300",
        STATUS_BORDER_STYLES[subscription.status],
        STATUS_GRADIENT_STYLES[subscription.status],
      )}
    >
      {/* ─── Collapsed: always visible ─────────────────────────── */}
      <button
        type="button"
        onClick={toggle}
        className="w-full cursor-pointer p-5 text-left"
      >
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-3">
            <div
              className={cn(
                "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px]",
                subscription.status === "active"
                  ? "bg-primary/10 text-primary"
                  : subscription.status === "paused"
                    ? "bg-[rgba(196,149,106,0.10)] text-[var(--warm)]"
                    : "bg-[rgba(245,236,227,0.04)] text-muted-foreground",
              )}
            >
              <TierIcon size={18} />
            </div>
            <div>
              <h3 className="font-semibold tracking-[-0.02em] text-foreground">
                {plan?.name ?? subscription.plan_id.slice(0, 8)}
              </h3>
              <div className="mt-1">
                <StatusBadge status={subscription.status} />
              </div>
            </div>
          </div>

          <div className="flex items-start gap-3">
            {plan && (
              <div className="text-right">
                <span className="font-mono text-lg font-bold text-foreground">
                  {formatMoney(
                    plan.base_price_amount,
                    plan.base_price_currency,
                  )}
                </span>
                <span className="ml-1 text-xs text-muted-foreground">
                  {intervalLabel(plan.billing_interval, t)}
                </span>
              </div>
            )}
            <ChevronDown
              size={18}
              className={cn(
                "mt-1.5 shrink-0 text-muted-foreground transition-transform duration-300",
                expanded && "rotate-180",
              )}
            />
          </div>
        </div>

        {/* Quick stats row */}
        <div className="mt-4 grid grid-cols-3 gap-2">
          <MiniStat
            icon={Activity}
            label={t("plans.traffic")}
            value={
              trafficTotalBytes > 0
                ? `${formatBytes(trafficUsedBytes)} / ${formatBytes(trafficTotalBytes)}`
                : "---"
            }
          />
          <MiniStat
            icon={Smartphone}
            label={t("plans.devices")}
            value={
              deviceLimit > 0
                ? `${devicesConnected} / ${deviceLimit}`
                : "---"
            }
          />
          <MiniStat
            icon={Calendar}
            label={t("traffic.remaining")}
            value={`${daysLeft}d`}
          />
        </div>
      </button>

      {/* ─── Expanded: details ─────────────────────────────────── */}
      <div
        className={cn(
          "grid transition-all duration-300",
          expanded ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
        )}
      >
        <div className="overflow-hidden">
          <div className="space-y-5 border-t border-border px-5 pb-5 pt-4">
            {/* Pending banner */}
            {subscription.status === "pending" && (
              <PendingBanner daysLeft={daysLeft} />
            )}

            {/* Plan details */}
            {plan && (
              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                    {t("checkout.plan")}
                  </span>
                  <p className="text-sm font-semibold text-foreground">
                    {plan.name}
                  </p>
                </div>
                <div className="space-y-1">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                    Tier
                  </span>
                  <p className="text-sm font-semibold capitalize text-foreground">
                    {plan.tier}
                  </p>
                </div>
                <div className="space-y-1">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                    Billing
                  </span>
                  <p className="text-sm font-semibold capitalize text-foreground">
                    {plan.billing_interval}
                  </p>
                </div>
              </div>
            )}

            {/* Traffic bar */}
            {trafficTotalBytes > 0 && (
              <TrafficBar
                usedBytes={trafficUsedBytes}
                totalBytes={trafficTotalBytes}
              />
            )}

            {/* Device slots */}
            {deviceLimit > 0 && (
              <DeviceSlots connected={devicesConnected} total={deviceLimit} />
            )}

            {/* Period info */}
            <div className="flex items-center gap-4 rounded-[10px] bg-[rgba(245,236,227,0.03)] px-4 py-3">
              <Calendar size={16} className="shrink-0 text-muted-foreground" />
              <div className="flex flex-1 items-center justify-between text-sm">
                <div className="space-y-0.5">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                    Period
                  </span>
                  <p className="font-mono text-foreground">
                    {formatDate(subscription.period_start)} &rarr;{" "}
                    {formatDate(subscription.period_end)}
                  </p>
                </div>
                <div className="text-right">
                  <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                    {t("traffic.remaining")}
                  </span>
                  <p className="font-mono font-semibold text-foreground">
                    {daysLeft} {daysLeft === 1 ? "day" : "days"}
                  </p>
                </div>
              </div>
            </div>

            {/* Addon chips */}
            {activeAddons.length > 0 && (
              <div className="space-y-2">
                <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  {t("checkout.addons")}
                </span>
                <div className="flex flex-wrap gap-2">
                  {activeAddons.map((addon) => (
                    <AddonChip key={addon.id} addon={addon} />
                  ))}
                </div>
              </div>
            )}

            {/* Action buttons */}
            <div className="flex items-center gap-3 pt-1">
              <Link
                to="/subscriptions/$id"
                params={{ id: subscription.id }}
                className="inline-flex items-center gap-1.5 rounded-[10px] bg-[rgba(245,236,227,0.06)] px-4 py-2 text-sm font-medium text-foreground transition-all hover:bg-[rgba(245,236,227,0.10)] hover:translate-y-[-1px]"
              >
                {t("subscriptions.manageAddons")}
                <ArrowUpRight size={14} />
              </Link>
              {subscription.status !== "cancelled" &&
                subscription.status !== "expired" && (
                  <Link
                    to="/plans"
                    className="inline-flex items-center gap-1.5 rounded-[10px] bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-all hover:brightness-110 hover:translate-y-[-1px]"
                  >
                    <Crown size={14} />
                    Upgrade
                  </Link>
                )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
