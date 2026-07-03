import {
  formatDate,
  formatMoney,
  type ResellerCommission,
  useResellerCommissions,
  useShopStore,
} from "@remnacore/shared";
import { useTranslation } from "react-i18next";
import {
  type Column,
  DataTable,
  PageHeader,
  StatusPill,
} from "@/components/ui";
import { NoShop } from "./dashboard";

export function ResellerCommissionsPage() {
  const { t } = useTranslation();
  const { activeShopId } = useShopStore();
  const { data: commissions, isLoading } = useResellerCommissions(activeShopId);

  if (!activeShopId) {
    return <NoShop title={t("reseller.commissions.title")} />;
  }

  const columns: Column<ResellerCommission>[] = [
    {
      key: "sale",
      header: t("reseller.commissions.saleId"),
      render: (c) => <span className="text-t3">{c.sale_id}</span>,
    },
    {
      key: "amount",
      header: t("reseller.commissions.amount"),
      render: (c) => (
        <span className="tabular-nums text-t2">
          {formatMoney(c.amount, c.currency)}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (c) => (
        <StatusPill
          label={c.status.toUpperCase()}
          tone={c.status === "paid" ? "ok" : "muted"}
        />
      ),
    },
    {
      key: "created",
      header: t("common.createdAt"),
      render: (c) => (
        <span className="text-t5">{formatDate(c.created_at)}</span>
      ),
    },
  ];

  return (
    <>
      <PageHeader title={t("reseller.commissions.title")} />
      <DataTable
        columns={columns}
        rows={commissions ?? []}
        cols="1.4fr .8fr .8fr 1fr"
        rowKey={(c) => c.id}
        empty={
          isLoading ? t("common.loading") : t("reseller.commissions.empty")
        }
      />
    </>
  );
}
