import type { User, UserRole } from "@remnacore/shared";
import { formatDate, USER_ROLES, useAdminUsers } from "@remnacore/shared";
import { useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  type Column,
  DataTable,
  PageHeader,
  Segmented,
  StatusPill,
  TermButton,
  TermInput,
} from "@/components/ui";

type RoleFilter = "all" | UserRole;

export function UsersPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const [roleFilter, setRoleFilter] = useState<RoleFilter>("all");
  const [search, setSearch] = useState("");
  const { data: users, isLoading } = useAdminUsers(pagination);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return (users ?? []).filter((u) => {
      if (roleFilter !== "all" && u.role !== roleFilter) return false;
      if (term && !u.email.toLowerCase().includes(term)) return false;
      return true;
    });
  }, [users, roleFilter, search]);

  const roleOptions: { value: RoleFilter; label: string }[] = [
    { value: "all", label: "ALL" },
    { value: USER_ROLES.customer, label: USER_ROLES.customer.toUpperCase() },
    { value: USER_ROLES.reseller, label: USER_ROLES.reseller.toUpperCase() },
    { value: USER_ROLES.admin, label: USER_ROLES.admin.toUpperCase() },
  ];

  const columns: Column<User>[] = [
    {
      key: "user",
      header: t("common.email"),
      render: (u) => (
        <span className="flex items-center gap-2.5">
          <span className="flex h-[26px] w-[26px] items-center justify-center border border-line bg-input text-[11px] uppercase text-accent">
            {u.email.charAt(0)}
          </span>
          <span className="text-t2">{u.email}</span>
        </span>
      ),
    },
    {
      key: "role",
      header: t("admin.users.role"),
      render: (u) => (
        <span className="uppercase tracking-[0.5px] text-t5">{u.role}</span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (u) =>
        u.email_verified ? (
          <StatusPill label="VERIFIED" tone="ok" />
        ) : (
          <StatusPill label="UNVERIFIED" tone="muted" />
        ),
    },
    {
      key: "created",
      header: t("common.createdAt"),
      render: (u) => (
        <span className="tabular-nums text-t5">{formatDate(u.created_at)}</span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: () => <span className="text-t7">⋯</span>,
    },
  ];

  return (
    <div className="space-y-3.5">
      <PageHeader title="USERS" breadcrumb="REMNAWAVE PROVIDER / CUSTOMERS" />

      <div className="flex flex-wrap items-center gap-2.5">
        <Segmented
          value={roleFilter}
          options={roleOptions}
          onChange={setRoleFilter}
        />
        <div className="flex-1" />
        <div className="min-w-[240px]">
          <TermInput
            type="search"
            placeholder={t("common.search").toUpperCase()}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>

      <DataTable<User>
        columns={columns}
        rows={isLoading ? [] : filtered}
        cols="1.6fr .8fr .9fr 1fr 40px"
        rowKey={(u) => u.id}
        onRowClick={(u) => navigate({ to: "/users/$id", params: { id: u.id } })}
        empty={isLoading ? t("common.loading").toUpperCase() : "NO USERS"}
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
          disabled={(users?.length ?? 0) < pagination.limit}
        >
          {t("common.next")}
        </TermButton>
      </div>
    </div>
  );
}
