import { LoadingSpinner, useSubscriptions } from "@remnacore/shared";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
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
    <div className="space-y-6 animate-fade-up">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-[-0.03em] text-foreground">
          {t("subscriptions.title")}
        </h1>
        <Link
          to="/plans"
          className="rounded-[10px] bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-all hover:brightness-110 hover:translate-y-[-1px]"
        >
          {t("subscriptions.browsePlans")}
        </Link>
      </div>

      {subscriptions && subscriptions.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {subscriptions.map((sub, index) => (
            <div
              key={sub.id}
              className="animate-fade-up"
              style={{ animationDelay: `${index * 60}ms` }}
            >
              <SubscriptionCard subscription={sub} />
            </div>
          ))}
        </div>
      ) : (
        <div
          className="animate-fade-up flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-card/50 p-12"
          style={{ animationDelay: "100ms" }}
        >
          <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="text-muted-foreground"
            >
              <path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8" />
              <path d="M21 3v5h-5" />
            </svg>
          </div>
          <p className="text-muted-foreground">{t("subscriptions.empty")}</p>
          <Link
            to="/plans"
            className="mt-4 rounded-[10px] bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-all hover:brightness-110 hover:translate-y-[-1px]"
          >
            {t("subscriptions.browsePlans")}
          </Link>
        </div>
      )}
    </div>
  );
}
