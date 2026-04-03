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
          className="font-medium text-primary hover:underline"
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
            "rounded-full px-2 py-0.5 text-xs font-medium",
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
            "text-xs",
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
        <span className="text-xs text-muted-foreground">
          {formatDate(row.original.created_at)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-foreground">
        {t("admin.users.title")}
      </h1>

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
          className="rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent disabled:opacity-50"
        >
          {t("common.back")}
        </button>
        <button
          type="button"
          onClick={() =>
            setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
          }
          disabled={(users?.length ?? 0) < pagination.limit}
          className="rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent disabled:opacity-50"
        >
          {t("common.next")}
        </button>
      </div>
    </div>
  );
}
