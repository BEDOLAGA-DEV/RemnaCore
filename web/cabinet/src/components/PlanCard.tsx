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
    monthly: t("plans.perMonth"),
    quarterly: t("plans.perQuarter"),
    yearly: t("plans.perYear"),
  };
  return labels[interval];
}

export function PlanCard({ plan, isCurrentPlan, onSelect }: PlanCardProps) {
  const { t } = useTranslation();
  const isPopular = plan.tier === "standard";

  return (
    <div
      className={cn(
        "relative flex flex-col rounded-xl border border-border bg-card p-6 shadow-sm transition-all hover:shadow-md",
        isPopular && "border-primary ring-1 ring-primary",
        isCurrentPlan && "opacity-75",
      )}
    >
      {isPopular && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2">
          <span className="rounded-full bg-primary px-3 py-1 text-xs font-semibold text-primary-foreground">
            {t("plans.popular")}
          </span>
        </div>
      )}

      <div className="mb-4">
        <h3 className="text-lg font-semibold text-foreground">{plan.name}</h3>
        {plan.description && (
          <p className="mt-1 text-sm text-muted-foreground">
            {plan.description}
          </p>
        )}
      </div>

      <div className="mb-6">
        <span className="text-3xl font-bold text-foreground">
          {formatMoney(plan.base_price_amount, plan.base_price_currency)}
        </span>
        <span className="text-sm text-muted-foreground">
          {intervalLabel(plan.billing_interval, t)}
        </span>
      </div>

      <ul className="mb-6 flex flex-col gap-3">
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="text-primary" />
          {t("plans.traffic")}: {formatBytes(plan.traffic_limit_bytes)}
        </li>
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="text-primary" />
          {t("plans.devices")}: {plan.device_limit}
        </li>
        <li className="flex items-center gap-2 text-sm text-foreground">
          <Check size={16} className="text-primary" />
          {t("plans.bindings")}: {plan.max_remnawave_bindings}
        </li>
        {plan.family_enabled && (
          <li className="flex items-center gap-2 text-sm text-foreground">
            <Zap size={16} className="text-primary" />
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
          "mt-auto w-full rounded-lg px-4 py-2.5 text-sm font-medium transition-colors",
          isCurrentPlan
            ? "bg-muted text-muted-foreground cursor-not-allowed"
            : "bg-primary text-primary-foreground hover:bg-primary/90",
        )}
      >
        {isCurrentPlan ? t("plans.currentPlan") : t("plans.subscribe")}
      </button>
    </div>
  );
}
