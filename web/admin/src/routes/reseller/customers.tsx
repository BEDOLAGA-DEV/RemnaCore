import {
  formatDate,
  type ResellerCustomer,
  useResellerCustomers,
  useShopStore,
} from "@remnacore/shared";
import { useTranslation } from "react-i18next";
import { type Column, DataTable, PageHeader } from "@/components/ui";
import { NoShop } from "./dashboard";

export function ResellerCustomersPage() {
  const { t } = useTranslation();
  const { activeShopId } = useShopStore();
  const { data: customers, isLoading } = useResellerCustomers(activeShopId);

  if (!activeShopId) {
    return <NoShop title={t("reseller.customers.title")} />;
  }

  const columns: Column<ResellerCustomer>[] = [
    {
      key: "email",
      header: t("common.email"),
      render: (c) => <span className="text-t2">{c.email}</span>,
    },
    {
      key: "name",
      header: t("reseller.customers.displayName"),
      render: (c) => <span className="text-t4">{c.display_name || "—"}</span>,
    },
    {
      key: "subs",
      header: t("reseller.customers.activeSubs"),
      render: (c) => (
        <span className="tabular-nums text-t3">{c.active_subs_count}</span>
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
      <PageHeader title={t("reseller.customers.title")} />
      <DataTable
        columns={columns}
        rows={customers ?? []}
        cols="1.4fr 1fr .6fr 1fr"
        rowKey={(c) => c.user_id}
        empty={isLoading ? t("common.loading") : t("reseller.customers.empty")}
      />
    </>
  );
}
