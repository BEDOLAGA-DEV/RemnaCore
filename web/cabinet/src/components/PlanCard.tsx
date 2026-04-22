import { useTranslation } from "react-i18next";
import { Check, Zap } from "lucide-react";
import { cn, formatBytes, formatMoney } from "@remnacore/shared";
import type { Plan, BillingInterval } from "@remnacore/shared";

type PlanCardProps = {
  plan: Plan;
  isCurrentPlan?: boolean;
  onSelect: (plan: Plan) => void;
};

function intervalLabel(interval: BillingInterval, t: (key: string) => string): string {
  const labels: Record<BillingInterval, string> = {
    month: t("plans.perMonth"),
    quarter: t("plans.perQuarter"),
    year: t("plans.perYear"),
  };
  return labels[interval];
}

export function PlanCard({ plan, isCurrentPlan, onSelect }: PlanCardProps) {
  const { t } = useTranslation();
  const isPopular = plan.tier === "standard";

  return (
    <div
      className={cn(
        "relative flex flex-col overflow-hidden rounded-lg border border-border bg-card p-6 transition-all hover:border-[rgba(255,235,210,0.12)]",
        isPopular && "border-primary ring-1 ring-primary/30",
        isCurrentPlan && "opacity-75",
      )}
    >
      {/* Subtle glow for popular plans */}
      {isPopular && (
        <div className="pointer-events-none absolute -top-24 left-1/2 -translate-x-1/2 h-48 w-48 rounded-full bg-primary/[0.06] blur-3xl" />
      )}

      {isPopular && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2 z-10">
          <span className="rounded-full bg-primary px-3 py-1 text-xs font-semibold text-primary-foreground">
            {t("plans.popular")}
          </span>
        </div>
      )}

      <div className="mb-4">
        <h3 className="text-lg font-bold tracking-[-0.03em] text-foreground">
          {plan.name}
        </h3>
        {plan.description && (
          <p className="mt-1 text-sm text-muted-foreground">
            {plan.description}
          </p>
        )}
      </div>

      <div className="mb-6">
        <span className="text-3xl font-bold text-foreground font-mono">
          {formatMoney(plan.base_price_amount, plan.base_price_currency)}
        </span>
        <span className="ml-1 text-sm text-muted-foreground">
          {intervalLabel(plan.billing_interval, t)}
        </span>
      </div>

      <ul className="mb-6 flex flex-col gap-3">
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="shrink-0 text-primary" />
          {t("plans.traffic")}: {formatBytes(plan.traffic_limit_bytes)}
        </li>
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="shrink-0 text-primary" />
          {t("plans.devices")}: {plan.device_limit}
        </li>
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="shrink-0 text-primary" />
          {t("plans.bindings")}: {plan.max_remnawave_bindings}
        </li>
        {plan.family_enabled && (
          <li className="flex items-center gap-2 text-sm text-foreground">
            <Zap size={16} className="shrink-0 text-primary" />
            {t("plans.familySharing")} -{" "}
            {t("plans.upToMembers", { count: plan.max_family_members })}
          </li>
        )}
      </ul>

      <button
        type="button"
        onClick={() => onSelect(plan)}
        disabled={isCurrentPlan}
        className={cn(
          "mt-auto w-full rounded-[10px] px-4 py-2.5 text-sm font-medium transition-all",
          isCurrentPlan
            ? "bg-muted text-muted-foreground cursor-not-allowed"
            : "bg-primary text-primary-foreground hover:brightness-110 hover:translate-y-[-1px]",
        )}
      >
        {isCurrentPlan ? t("plans.currentPlan") : t("plans.subscribe")}
      </button>
    </div>
  );
}
