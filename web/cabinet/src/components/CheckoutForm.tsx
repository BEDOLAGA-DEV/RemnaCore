import type { Plan } from "@remnacore/shared";
import { formatMoney, useStartCheckout } from "@remnacore/shared";
import { Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";

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
    <div className="rounded-lg border border-border bg-card p-6">
      <h3 className="text-lg font-bold tracking-[-0.03em] text-foreground">
        {t("checkout.summary")}
      </h3>

      <div className="mt-4 space-y-3">
        <div className="flex justify-between text-sm">
          <span className="text-muted-foreground">{t("checkout.plan")}</span>
          <span className="font-mono font-medium text-foreground">
            {plan.name} -{" "}
            {formatMoney(plan.base_price_amount, plan.base_price_currency)}
          </span>
        </div>

        {selectedAddonIds.length > 0 && (
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">
              {t("checkout.addons")}
            </span>
            <span className="font-mono font-medium text-foreground">
              {formatMoney(addonTotal, plan.base_price_currency)}
            </span>
          </div>
        )}

        <div className="border-t border-border pt-3">
          <div className="flex justify-between text-base font-bold">
            <span className="text-foreground">{t("checkout.total")}</span>
            <span className="font-mono text-foreground">
              {formatMoney(total, plan.base_price_currency)}
            </span>
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={handleCheckout}
        disabled={checkout.isPending}
        className="mt-6 w-full rounded-[10px] bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground transition-all hover:brightness-110 hover:translate-y-[-1px] disabled:opacity-50"
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
        <p className="mt-3 text-sm text-destructive">{t("common.error")}</p>
      )}
    </div>
  );
}
