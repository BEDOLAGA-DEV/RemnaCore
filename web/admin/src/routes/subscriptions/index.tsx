import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { type ColumnDef } from "@tanstack/react-table";
import { useAdminSubscriptions, formatDate, cn } from "@remnacore/shared";
import type { Subscription, SubscriptionStatus } from "@remnacore/shared";
import { DataTable } from "../../components/DataTable.js";

function statusColor(status: SubscriptionStatus): string {
  const colors: Record<SubscriptionStatus, string> = {
    active: "bg-green-500/10 text-green-500",
    pending: "bg-yellow-500/10 text-yellow-500",
    cancelled: "bg-red-500/10 text-red-500",
    expired: "bg-gray-500/10 text-gray-500",
    paused: "bg-blue-500/10 text-blue-500",
  };
  return colors[status];
}

export function AdminSubscriptionsPage() {
  const { t } = useTranslation();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: subs, isLoading } = useAdminSubscriptions(pagination);

  const columns: ColumnDef<Subscription, unknown>[] = [
    {
      accessorKey: "id",
      header: "ID",
      cell: ({ row }) => (
        <Link
          to="/subscriptions/$id"
          params={{ id: row.original.id }}
          className="font-mono text-[11px] text-primary hover:text-primary/80"
        >
          {row.original.id.slice(0, 8)}...
        </Link>
      ),
    },
    {
      accessorKey: "user_id",
      header: t("admin.subscriptions.userId"),
      cell: ({ row }) => (
        <Link
          to="/users/$id"
          params={{ id: row.original.user_id }}
          className="font-mono text-[11px] text-primary hover:text-primary/80"
        >
          {row.original.user_id.slice(0, 8)}...
        </Link>
      ),
    },
    {
      accessorKey: "status",
      header: t("common.status"),
      cell: ({ row }) => (
        <span
          className={cn(
            "rounded-full px-2 py-0.5 font-mono text-[11px] font-medium",
            statusColor(row.original.status),
          )}
        >
          {row.original.status}
        </span>
      ),
    },
    {
      accessorKey: "period_end",
      header: t("subscriptions.periodEnd"),
      cell: ({ row }) => (
        <span className="font-mono text-[11px] text-muted-foreground">
          {formatDate(row.original.period_end)}
        </span>
      ),
    },
    {
      accessorKey: "created_at",
      header: t("common.createdAt"),
      cell: ({ row }) => (
        <span className="font-mono text-[11px] text-muted-foreground">
          {formatDate(row.original.created_at)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold text-foreground">
          {t("admin.subscriptions.title")}
        </h1>
      </div>

      <DataTable data={subs ?? []} columns={columns} isLoading={isLoading} />

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
          className="rounded-lg border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
        >
          {t("common.back")}
        </button>
        <button
          type="button"
          onClick={() =>
            setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
          }
          disabled={(subs?.length ?? 0) < pagination.limit}
          className="rounded-lg border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
        >
          {t("common.next")}
        </button>
      </div>
    </div>
  );
}
