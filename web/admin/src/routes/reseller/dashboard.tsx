import {
  formatMoney,
  useResellerDashboard,
  useShopStore,
} from "@remnacore/shared";
import { useTranslation } from "react-i18next";
import { KpiGrid, PageHeader, Panel, StatCell } from "@/components/ui";

export function ResellerDashboardPage() {
  const { t } = useTranslation();
  const { activeShopId } = useShopStore();
  const { data, isLoading } = useResellerDashboard(activeShopId);

  if (!activeShopId) {
    return <NoShop title={t("reseller.dashboard.title")} />;
  }

  return (
    <>
      <PageHeader
        title={t("reseller.dashboard.title")}
        breadcrumb={data?.tenant_name}
      />
      {isLoading || !data ? (
        <Panel>
          <p className="p-4 text-[12px] text-t5">{t("common.loading")}</p>
        </Panel>
      ) : (
        <KpiGrid cols={3}>
          <StatCell
            label={t("reseller.dashboard.activeCustomers")}
            value={data.summary.active_customers}
          />
          <StatCell
            label={t("reseller.dashboard.activeSubscriptions")}
            value={data.summary.active_subscriptions}
          />
          <StatCell
            label={t("reseller.dashboard.pendingCommission")}
            value={formatMoney(
              data.summary.pending_commission,
              data.summary.currency,
            )}
          />
        </KpiGrid>
      )}
    </>
  );
}

export function NoShop({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <>
      <PageHeader title={title} />
      <Panel>
        <div className="px-4 py-8 text-center text-[11px] uppercase tracking-[1px] text-t6">
          {t("reseller.selectShopFirst")}
        </div>
      </Panel>
    </>
  );
}
