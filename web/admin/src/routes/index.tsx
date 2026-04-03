import { useTranslation } from "react-i18next";
import { Users, Package, DollarSign } from "lucide-react";
import {
  useAdminUsers,
  useAdminSubscriptions,
  useAdminInvoices,
  LoadingSpinner,
  PAGINATION_DEFAULTS,
} from "@remnacore/shared";
import { StatsCard } from "../components/StatsCard.js";

export function AdminDashboardPage() {
  const { t } = useTranslation();

  // Fetch with default limit to get reasonable approximations.
  // TODO: Replace with a dedicated /admin/stats endpoint that returns
  //       accurate totals without fetching full entity lists.
  const { data: users, isLoading: usersLoading } = useAdminUsers({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: subs, isLoading: subsLoading } = useAdminSubscriptions({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });
  const { data: invoices, isLoading: invoicesLoading } = useAdminInvoices({
    limit: PAGINATION_DEFAULTS.maxLimit,
    offset: 0,
  });

  const isLoading = usersLoading || subsLoading || invoicesLoading;

  if (isLoading) return <LoadingSpinner />;

  // Counts are approximate (capped at maxLimit) until a proper count endpoint exists.
  const activeSubs =
    subs?.filter((s) => s.status === "active").length ?? 0;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-foreground">
        {t("admin.dashboard.title")}
      </h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          label={t("admin.dashboard.totalUsers")}
          value={users?.length ?? 0}
          icon={Users}
          colorClass="text-blue-500 bg-blue-500/10"
        />
        <StatsCard
          label={t("admin.dashboard.totalSubscriptions")}
          value={subs?.length ?? 0}
          icon={Package}
          colorClass="text-green-500 bg-green-500/10"
        />
        <StatsCard
          label={t("admin.dashboard.activeSubscriptions")}
          value={activeSubs}
          icon={Package}
          colorClass="text-emerald-500 bg-emerald-500/10"
        />
        <StatsCard
          label={t("admin.dashboard.totalRevenue")}
          value={invoices?.length ?? 0}
          icon={DollarSign}
          colorClass="text-yellow-500 bg-yellow-500/10"
        />
      </div>
    </div>
  );
}
