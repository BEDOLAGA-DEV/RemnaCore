import { useTranslation } from "react-i18next";
import { useStartCheckout } from "@remnacore/shared";
import { formatMoney } from "@remnacore/shared";
import { Loader2 } from "lucide-react";
import type { Plan } from "@remnacore/shared";

type CheckoutFormProps = {
  plan: Plan;
  selectedAddonIds: string[];
};

export function CheckoutForm({ plan, selectedAddonIds }: CheckoutFormProps) {
  const { t } = useTranslation();
  const checkout = useStartCheckout();

  const addonTotal = (plan.addons ?? [])
    .filter((a) => selectedAddonIds.includes(a.id))
    .reduce((sum, a) => sum + a.price_amount, 0);

  const total = plan.base_price_amount + addonTotal;

  const handleCheckout = () => {
    checkout.mutate({
      plan_id: plan.id,
      addon_ids: selectedAddonIds,
      return_url: `${window.location.origin}/subscriptions`,
      cancel_url: `${window.location.origin}/plans`,
    });
  };

  // Redirect to payment URL on success
  if (checkout.isSuccess && checkout.data.payment_url) {
    window.location.href = checkout.data.payment_url;
  }

  return (
    <div className="rounded-xl border border-border bg-card p-6">
      <h3 className="text-lg font-semibold text-foreground">
        {t("checkout.summary")}
      </h3>

      <div className="mt-4 space-y-3">
        <div className="flex justify-between text-sm">
          <span className="text-muted-foreground">{t("checkout.plan")}</span>
          <span className="font-medium text-foreground">
            {plan.name} -{" "}
            {formatMoney(plan.base_price_amount, plan.base_price_currency)}
          </span>
        </div>

        {selectedAddonIds.length > 0 && (
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">
              {t("checkout.addons")}
            </span>
            <span className="font-medium text-foreground">
              {formatMoney(addonTotal, plan.base_price_currency)}
            </span>
          </div>
        )}

        <div className="border-t border-border pt-3">
          <div className="flex justify-between text-base font-semibold">
            <span className="text-foreground">{t("checkout.total")}</span>
            <span className="text-foreground">
              {formatMoney(total, plan.base_price_currency)}
            </span>
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={handleCheckout}
        disabled={checkout.isPending}
        className="mt-6 w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
      >
        {checkout.isPending ? (
          <span className="flex items-center justify-center gap-2">
            <Loader2 size={16} className="animate-spin" />
            {t("checkout.processing")}
          </span>
        ) : (
          t("checkout.pay")
        )}
      </button>

      {checkout.isError && (
        <p className="mt-3 text-sm text-destructive">
          {t("common.error")}
        </p>
      )}
    </div>
  );
}
