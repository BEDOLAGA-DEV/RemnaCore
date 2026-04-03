import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { usePlans, LoadingSpinner } from "@remnacore/shared";
import type { Plan } from "@remnacore/shared";
import { PlanCard } from "../components/PlanCard.js";

export function PlansPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: plans, isLoading, isError } = usePlans();

  const handleSelect = (plan: Plan) => {
    navigate({
      to: "/checkout",
      search: { planId: plan.id },
    });
  };

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="text-center">
        <h1 className="text-2xl font-bold text-foreground">
          {t("plans.title")}
        </h1>
        <p className="mt-1 text-muted-foreground">{t("plans.subtitle")}</p>
      </div>

      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {plans?.map((plan) => (
          <PlanCard key={plan.id} plan={plan} onSelect={handleSelect} />
        ))}
      </div>
    </div>
  );
}
