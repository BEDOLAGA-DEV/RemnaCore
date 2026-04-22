import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { usePlans, LoadingSpinner, cn, formatMoney, formatBytes } from "@remnacore/shared";
import type { Plan, BillingInterval } from "@remnacore/shared";
import { Check, Zap, Crown, Shield } from "lucide-react";
import { useMemo, useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

type PlanGroup = {
  groupId: string;
  name: string;
  description: string | null;
  plans: Plan[]; // one per period
};

// ─── Helpers ────────────────────────────────────────────────────────────────

function intervalLabel(interval: BillingInterval): string {
  const labels: Record<string, string> = {
    month: "/ month",
    monthly: "/ month",
    quarter: "/ 3 months",
    quarterly: "/ 3 months",
    year: "/ year",
    yearly: "/ year",
  };
  return labels[interval] ?? `/ ${interval}`;
}

function intervalShort(interval: BillingInterval): string {
  const labels: Record<string, string> = {
    month: "1 mo",
    monthly: "1 mo",
    quarter: "3 mo",
    quarterly: "3 mo",
    year: "12 mo",
    yearly: "12 mo",
  };
  return labels[interval] ?? interval;
}

function tierIcon(tier: string) {
  if (tier === "premium") return Crown;
  if (tier === "ultra") return Zap;
  return Shield;
}

// ─── Component ──────────────────────────────────────────────────────────────

export function PlansPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: plans, isLoading, isError } = usePlans();

  // Group plans by group_id (tariff). Plans with no group_id are standalone.
  const groups = useMemo<PlanGroup[]>(() => {
    if (!plans) return [];
    const map = new Map<string, PlanGroup>();
    for (const plan of plans) {
      const gid = plan.group_id || plan.id;
      if (!map.has(gid)) {
        // Extract base name (remove " — period" suffix)
        const baseName = plan.name.includes(" — ")
          ? plan.name.split(" — ")[0]!
          : plan.name;
        map.set(gid, {
          groupId: gid,
          name: baseName,
          description: plan.description,
          plans: [],
        });
      }
      map.get(gid)!.plans.push(plan);
    }
    return Array.from(map.values());
  }, [plans]);

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-8 animate-fade-up">
      <div className="text-center">
        <h1 className="text-3xl font-bold tracking-[-0.03em] text-foreground">
          {t("plans.title")}
        </h1>
        <p className="mt-2 text-muted-foreground">{t("plans.subtitle")}</p>
      </div>

      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {groups.map((group, index) => (
          <div
            key={group.groupId}
            className="animate-fade-up"
            style={{ animationDelay: `${index * 80}ms` }}
          >
            <PlanGroupCard
              group={group}
              onSelect={(plan) =>
                navigate({ to: "/checkout", search: { planId: plan.id } })
              }
            />
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Plan Group Card ────────────────────────────────────────────────────────

function PlanGroupCard({
  group,
  onSelect,
}: {
  group: PlanGroup;
  onSelect: (plan: Plan) => void;
}) {
  const { t } = useTranslation();
  const hasMultiplePeriods = group.plans.length > 1;

  // Default to the first plan or the one with isDefault
  const [selectedIdx, setSelectedIdx] = useState(() => {
    // Plans don't have isDefault on the billing side, so just pick first
    return 0;
  });

  const selectedPlan = group.plans[selectedIdx] ?? group.plans[0]!;
  const isPopular = selectedPlan.tier === "standard" || selectedPlan.tier === "premium";
  const TierIcon = tierIcon(selectedPlan.tier);

  return (
    <div
      className={cn(
        "relative flex flex-col overflow-hidden rounded-[16px] border border-border bg-card p-6 transition-all hover:border-[rgba(255,235,210,0.12)] hover:translate-y-[-1px]",
        isPopular && "border-primary/30 ring-1 ring-primary/20",
      )}
    >
      {/* Popular badge */}
      {isPopular && (
        <>
          <div className="pointer-events-none absolute -top-24 left-1/2 -translate-x-1/2 h-48 w-48 rounded-full bg-primary/[0.06] blur-3xl" />
          <div className="absolute -top-3 left-1/2 -translate-x-1/2 z-10">
            <span className="rounded-full bg-primary px-3 py-1 text-xs font-semibold text-primary-foreground">
              {t("plans.popular")}
            </span>
          </div>
        </>
      )}

      {/* Header */}
      <div className="mb-4 flex items-center gap-3">
        <div
          className={cn(
            "flex h-10 w-10 items-center justify-center rounded-xl",
            isPopular ? "bg-primary/10" : "bg-[rgba(245,236,227,0.06)]",
          )}
        >
          <TierIcon
            size={18}
            className={isPopular ? "text-primary" : "text-muted-foreground"}
          />
        </div>
        <div>
          <h3 className="text-lg font-bold tracking-[-0.02em] text-foreground">
            {group.name}
          </h3>
          {group.description && (
            <p className="text-xs text-muted-foreground">{group.description}</p>
          )}
        </div>
      </div>

      {/* Period selector tabs */}
      {hasMultiplePeriods && (
        <div className="mb-4 flex gap-1 rounded-xl bg-[rgba(245,236,227,0.04)] p-1">
          {group.plans.map((plan, idx) => (
            <button
              key={plan.id}
              type="button"
              onClick={() => setSelectedIdx(idx)}
              className={cn(
                "flex-1 rounded-lg px-2 py-1.5 text-xs font-medium transition-all",
                idx === selectedIdx
                  ? "bg-primary text-primary-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {intervalShort(plan.billing_interval)}
            </button>
          ))}
        </div>
      )}

      {/* Price */}
      <div className="mb-5">
        <div className="flex items-baseline gap-1">
          <span className="font-mono text-3xl font-bold text-foreground">
            {formatMoney(selectedPlan.base_price_amount, selectedPlan.base_price_currency)}
          </span>
          <span className="text-sm text-muted-foreground">
            {intervalLabel(selectedPlan.billing_interval)}
          </span>
        </div>
      </div>

      {/* Features */}
      <ul className="mb-6 flex flex-col gap-2.5">
        <li className="flex items-center gap-2.5 text-sm text-foreground">
          <Check size={15} className="shrink-0 text-primary" />
          {t("plans.traffic")}: {selectedPlan.traffic_limit_bytes > 0 ? formatBytes(selectedPlan.traffic_limit_bytes) : t("plans.unlimited")}
        </li>
        <li className="flex items-center gap-2.5 text-sm text-foreground">
          <Check size={15} className="shrink-0 text-primary" />
          {t("plans.devices")}: {selectedPlan.device_limit || t("plans.unlimited")}
        </li>
        <li className="flex items-center gap-2.5 text-sm text-foreground">
          <Check size={15} className="shrink-0 text-primary" />
          {t("plans.bindings")}: {selectedPlan.max_remnawave_bindings}
        </li>
        {selectedPlan.family_enabled && (
          <li className="flex items-center gap-2.5 text-sm text-foreground">
            <Zap size={15} className="shrink-0 text-primary" />
            {t("plans.familySharing")} — {t("plans.upToMembers", { count: selectedPlan.max_family_members })}
          </li>
        )}
      </ul>

      {/* CTA */}
      <button
        type="button"
        onClick={() => onSelect(selectedPlan)}
        className="mt-auto w-full rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground transition-all hover:brightness-110 hover:translate-y-[-1px]"
      >
        {t("plans.subscribe")}
      </button>
    </div>
  );
}
