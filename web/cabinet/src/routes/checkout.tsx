import { useTranslation } from "react-i18next";
import { Link, useSearch } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { usePlan, LoadingSpinner } from "@remnacore/shared";
import { CheckoutForm } from "../components/CheckoutForm.js";

export function CheckoutPage() {
  const { t } = useTranslation();
  const search = useSearch({ strict: false }) as { planId?: string };
  const planId = search.planId ?? "";
  const { data: plan, isLoading } = usePlan(planId);

  if (!planId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t("common.error")}</p>
        <Link to="/plans" className="mt-4 text-primary hover:underline">
          {t("subscriptions.browsePlans")}
        </Link>
      </div>
    );
  }

  if (isLoading) return <LoadingSpinner />;

  if (!plan) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-lg space-y-6">
      <Link
        to="/plans"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <h1 className="text-2xl font-bold text-foreground">
        {t("checkout.title")}
      </h1>

      <CheckoutForm plan={plan} selectedAddonIds={[]} />
    </div>
  );
}
