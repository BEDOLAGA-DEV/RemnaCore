import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Package, CreditCard, Wifi, Users } from "lucide-react";
import {
  useSubscriptions,
  useInvoices,
  useBindings,
  useMe,
  LoadingSpinner,
  formatDate,
  cn,
} from "@remnacore/shared";

export function DashboardPage() {
  const { t } = useTranslation();
  const { data: user } = useMe();
  const { data: subscriptions, isLoading: subsLoading } = useSubscriptions();
  const { data: invoices, isLoading: invoicesLoading } = useInvoices();
  const { data: bindings, isLoading: bindingsLoading } = useBindings();

  const isLoading = subsLoading || invoicesLoading || bindingsLoading;

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const activeSubs = subscriptions?.filter((s) => s.status === "active") ?? [];
  const pendingInvoices = invoices?.filter((i) => i.status === "pending") ?? [];
  const syncedBindings = bindings?.filter((b) => b.status === "synced") ?? [];

  const stats = [
    {
      label: t("nav.subscriptions"),
      value: activeSubs.length,
      icon: Package,
      to: "/subscriptions" as const,
      color: "text-blue-500 bg-blue-500/10",
    },
    {
      label: t("nav.invoices"),
      value: pendingInvoices.length,
      icon: CreditCard,
      to: "/subscriptions" as const,
      color: "text-yellow-500 bg-yellow-500/10",
    },
    {
      label: t("bindings.title"),
      value: syncedBindings.length,
      icon: Wifi,
      to: "/traffic" as const,
      color: "text-green-500 bg-green-500/10",
    },
  ];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-foreground">
          {t("common.dashboard")}
        </h1>
        {user && (
          <p className="mt-1 text-sm text-muted-foreground">
            {t("auth.signIn")}: {user.display_name ?? user.email}
          </p>
        )}
      </div>

      {/* Stats grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {stats.map((stat) => (
          <Link
            key={stat.label}
            to={stat.to}
            className="rounded-xl border border-border bg-card p-5 transition-all hover:shadow-md"
          >
            <div className="flex items-center gap-4">
              <div className={cn("rounded-lg p-3", stat.color)}>
                <stat.icon size={20} />
              </div>
              <div>
                <p className="text-2xl font-bold text-foreground">
                  {stat.value}
                </p>
                <p className="text-sm text-muted-foreground">{stat.label}</p>
              </div>
            </div>
          </Link>
        ))}
      </div>

      {/* Recent subscriptions */}
      {activeSubs.length > 0 && (
        <div className="rounded-xl border border-border bg-card p-5">
          <h2 className="mb-4 text-lg font-semibold text-foreground">
            {t("subscriptions.title")}
          </h2>
          <div className="space-y-3">
            {activeSubs.slice(0, 3).map((sub) => (
              <Link
                key={sub.id}
                to="/subscriptions/$id"
                params={{ id: sub.id }}
                className="flex items-center justify-between rounded-lg border border-border p-3 transition-colors hover:bg-accent"
              >
                <span className="font-mono text-sm text-foreground">
                  {sub.id.slice(0, 8)}...
                </span>
                <span className="text-xs text-muted-foreground">
                  {t("subscriptions.periodEnd")}: {formatDate(sub.period_end)}
                </span>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Empty state */}
      {activeSubs.length === 0 && (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <Users size={48} className="text-muted-foreground" />
          <h3 className="mt-4 text-lg font-semibold text-foreground">
            {t("subscriptions.empty")}
          </h3>
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
