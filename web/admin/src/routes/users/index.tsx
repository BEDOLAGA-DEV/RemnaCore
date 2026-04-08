import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { type ColumnDef } from "@tanstack/react-table";
import { useAdminUsers, formatDate, cn, USER_ROLES } from "@remnacore/shared";
import type { User } from "@remnacore/shared";
import { DataTable } from "../../components/DataTable.js";

export function UsersPage() {
  const { t } = useTranslation();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: users, isLoading } = useAdminUsers(pagination);

  const columns: ColumnDef<User, unknown>[] = [
    {
      accessorKey: "email",
      header: t("common.email"),
      cell: ({ row }) => (
        <Link
          to="/users/$id"
          params={{ id: row.original.id }}
          className="font-medium text-primary hover:text-primary/80"
        >
          {row.original.email}
        </Link>
      ),
    },
    {
      accessorKey: "role",
      header: t("admin.users.role"),
      cell: ({ row }) => (
        <span
          className={cn(
            "rounded-full px-2 py-0.5 font-mono text-[11px] font-medium",
            row.original.role === USER_ROLES.admin
              ? "bg-purple-500/10 text-purple-500"
              : row.original.role === USER_ROLES.reseller
                ? "bg-blue-500/10 text-blue-500"
                : "bg-muted text-muted-foreground",
          )}
        >
          {row.original.role}
        </span>
      ),
    },
    {
      accessorKey: "email_verified",
      header: t("admin.users.emailVerified"),
      cell: ({ row }) => (
        <span
          className={cn(
            "font-mono text-[11px]",
            row.original.email_verified
              ? "text-green-500"
              : "text-muted-foreground",
          )}
        >
          {row.original.email_verified ? t("common.yes") : t("common.no")}
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
          {t("admin.users.title")}
        </h1>
      </div>

      <DataTable data={users ?? []} columns={columns} isLoading={isLoading} />

      {/* Simple pagination */}
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
          disabled={(users?.length ?? 0) < pagination.limit}
          className="rounded-lg border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
        >
          {t("common.next")}
        </button>
      </div>
    </div>
  );
}
