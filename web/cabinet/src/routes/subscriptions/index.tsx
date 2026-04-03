import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useSubscriptions, LoadingSpinner } from "@remnacore/shared";
import { SubscriptionCard } from "../../components/SubscriptionCard.js";

export function SubscriptionsPage() {
  const { t } = useTranslation();
  const { data: subscriptions, isLoading, isError } = useSubscriptions();

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
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">
          {t("subscriptions.title")}
        </h1>
        <Link
          to="/plans"
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          {t("subscriptions.browsePlans")}
        </Link>
      </div>

      {subscriptions && subscriptions.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {subscriptions.map((sub) => (
            <SubscriptionCard key={sub.id} subscription={sub} />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <p className="text-muted-foreground">{t("subscriptions.empty")}</p>
          <Link
            to="/plans"
            className="mt-4 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            {t("subscriptions.browsePlans")}
          </Link>
        </div>
      )}
    </div>
  );
}
