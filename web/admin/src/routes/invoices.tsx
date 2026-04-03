import { useState } from "react";
import { useTranslation } from "react-i18next";
import { type ColumnDef } from "@tanstack/react-table";
import { useAdminInvoices, formatDate, formatMoney, cn } from "@remnacore/shared";
import type { Invoice, InvoiceStatus } from "@remnacore/shared";
import { DataTable } from "../components/DataTable.js";

function statusColor(status: InvoiceStatus): string {
  const colors: Record<InvoiceStatus, string> = {
    pending: "bg-yellow-500/10 text-yellow-500",
    paid: "bg-green-500/10 text-green-500",
    cancelled: "bg-gray-500/10 text-gray-500",
    refunded: "bg-blue-500/10 text-blue-500",
  };
  return colors[status];
}

export function AdminInvoicesPage() {
  const { t } = useTranslation();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: invoices, isLoading } = useAdminInvoices(pagination);

  const columns: ColumnDef<Invoice, unknown>[] = [
    {
      accessorKey: "id",
      header: "ID",
      cell: ({ row }) => (
        <span className="font-mono text-xs text-foreground">
          {row.original.id.slice(0, 8)}...
        </span>
      ),
    },
    {
      accessorKey: "total_amount",
      header: t("invoices.amount"),
      cell: ({ row }) => (
        <span className="font-medium text-foreground">
          {formatMoney(row.original.total_amount, row.original.currency)}
        </span>
      ),
    },
    {
      accessorKey: "status",
      header: t("common.status"),
      cell: ({ row }) => (
        <span
          className={cn(
            "rounded-full px-2 py-0.5 text-xs font-medium",
            statusColor(row.original.status),
          )}
        >
          {row.original.status}
        </span>
      ),
    },
    {
      accessorKey: "paid_at",
      header: t("invoices.paidAt"),
      cell: ({ row }) => (
        <span className="text-xs text-muted-foreground">
          {formatDate(row.original.paid_at)}
        </span>
      ),
    },
    {
      accessorKey: "created_at",
      header: t("common.createdAt"),
      cell: ({ row }) => (
        <span className="text-xs text-muted-foreground">
          {formatDate(row.original.created_at)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-foreground">
        {t("admin.invoices.title")}
      </h1>

      <DataTable
        data={invoices ?? []}
        columns={columns}
        isLoading={isLoading}
      />

      <div className="flex items-center justify-end gap-2">
        <button
          type="button"
          onClick={() =>
            setPagination((p) => ({
              ...p,
              offset: Math.max(0, p.offset - p.limit),
            }))
          }
          disabled={pagination.offset === 0}
          className="rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent disabled:opacity-50"
        >
          {t("common.back")}
        </button>
        <button
          type="button"
          onClick={() =>
            setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
          }
          disabled={(invoices?.length ?? 0) < pagination.limit}
          className="rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent disabled:opacity-50"
        >
          {t("common.next")}
        </button>
      </div>
    </div>
  );
}
