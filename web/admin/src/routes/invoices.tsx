import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useAdminInvoices, formatDate, formatMoney } from "@remnacore/shared";
import type { Invoice, InvoiceStatus } from "@remnacore/shared";
import {
  PageHeader,
  DataTable,
  type Column,
  StatusPill,
  type Tone,
  TermButton,
} from "@/components/ui";

function statusTone(status: InvoiceStatus): Tone {
  const tones: Record<InvoiceStatus, Tone> = {
    paid: "ok",
    pending: "warn",
    cancelled: "danger",
    refunded: "danger",
  };
  return tones[status];
}

export function AdminInvoicesPage() {
  const { t } = useTranslation();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: invoices, isLoading } = useAdminInvoices(pagination);

  const columns: Column<Invoice>[] = [
    {
      key: "id",
      header: "ID",
      render: (inv) => (
        <span className="tabular-nums text-accent">{inv.id.slice(0, 8)}</span>
      ),
    },
    {
      key: "user",
      header: t("admin.subscriptions.userId"),
      render: (inv) => (
        <span className="tabular-nums text-t2">
          {inv.user_id.slice(0, 8)}
        </span>
      ),
    },
    {
      key: "amount",
      header: t("invoices.amount"),
      align: "right",
      render: (inv) => (
        <span className="tabular-nums font-semibold text-t1">
          {formatMoney(inv.total_amount, inv.currency)}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (inv) => (
        <StatusPill
          label={inv.status.toUpperCase()}
          tone={statusTone(inv.status)}
        />
      ),
    },
    {
      key: "paid",
      header: t("invoices.paidAt"),
      render: (inv) => (
        <span className="tabular-nums text-t5">{formatDate(inv.paid_at)}</span>
      ),
    },
    {
      key: "created",
      header: t("common.createdAt"),
      render: (inv) => (
        <span className="tabular-nums text-t5">
          {formatDate(inv.created_at)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-3.5">
      <PageHeader
        title="INVOICES"
        breadcrumb="REMNAWAVE PROVIDER / BILLING / LEDGER"
      />

      <DataTable<Invoice>
        columns={columns}
        rows={isLoading ? [] : (invoices ?? [])}
        cols=".9fr 1.1fr .9fr 1fr 1.1fr 1.1fr"
        rowKey={(inv) => inv.id}
        empty={isLoading ? t("common.loading").toUpperCase() : "NO INVOICES"}
      />

      <div className="flex items-center justify-end gap-2">
        <TermButton
          type="button"
          variant="ghost"
          onClick={() =>
            setPagination((p) => ({
              ...p,
              offset: Math.max(0, p.offset - p.limit),
            }))
          }
          disabled={pagination.offset === 0}
        >
          {t("common.back")}
        </TermButton>
        <TermButton
          type="button"
          variant="ghost"
          onClick={() =>
            setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
          }
          disabled={(invoices?.length ?? 0) < pagination.limit}
        >
          {t("common.next")}
        </TermButton>
      </div>
    </div>
  );
}
