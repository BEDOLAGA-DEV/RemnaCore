import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Plus, Loader2 } from "lucide-react";
import { type ColumnDef } from "@tanstack/react-table";
import {
  useAdminTenants,
  useCreateTenant,
  formatDate,
  cn,
} from "@remnacore/shared";
import type { Tenant } from "@remnacore/shared";
import { DataTable } from "../../components/DataTable.js";

const createTenantSchema = z.object({
  name: z.string().min(1),
  domain: z.string().optional(),
  owner_user_id: z.string().min(1),
});

type CreateTenantValues = z.infer<typeof createTenantSchema>;

export function TenantsPage() {
  const { t } = useTranslation();
  const [showForm, setShowForm] = useState(false);
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: tenants, isLoading } = useAdminTenants(pagination);
  const createTenant = useCreateTenant();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CreateTenantValues>({
    resolver: zodResolver(createTenantSchema),
  });

  const onSubmit = (data: CreateTenantValues) => {
    createTenant.mutate(data, {
      onSuccess: () => {
        reset();
        setShowForm(false);
      },
    });
  };

  const columns: ColumnDef<Tenant, unknown>[] = [
    {
      accessorKey: "name",
      header: t("common.name"),
      cell: ({ row }) => (
        <Link
          to="/tenants/$id"
          params={{ id: row.original.id }}
          className="font-medium text-primary hover:text-primary/80"
        >
          {row.original.name}
        </Link>
      ),
    },
    {
      accessorKey: "domain",
      header: t("admin.tenants.domain"),
      cell: ({ row }) => (
        <span className="font-mono text-[11px] text-muted-foreground">
          {row.original.domain ?? "—"}
        </span>
      ),
    },
    {
      accessorKey: "is_active",
      header: t("common.status"),
      cell: ({ row }) => (
        <span
          className={cn(
            "rounded-full px-2 py-0.5 font-mono text-[11px] font-medium",
            row.original.is_active
              ? "bg-green-500/10 text-green-500"
              : "bg-red-500/10 text-red-500",
          )}
        >
          {row.original.is_active
            ? t("admin.tenants.active")
            : t("admin.tenants.inactive")}
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
          {t("admin.tenants.title")}
        </h1>
        <button
          type="button"
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
        >
          <Plus size={14} />
          {t("admin.tenants.create")}
        </button>
      </div>

      {showForm && (
        <div className="rounded-xl border border-border bg-card p-5">
          <h2 className="mb-4 text-[13px] font-semibold text-foreground">
            {t("admin.tenants.create")}
          </h2>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="name"
                  className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
                >
                  {t("common.name")}
                </label>
                <input
                  id="name"
                  {...register("name")}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                />
                {errors.name && (
                  <p className="mt-1 text-sm text-destructive">
                    {errors.name.message}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="domain"
                  className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
                >
                  {t("admin.tenants.domain")}
                </label>
                <input
                  id="domain"
                  {...register("domain")}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                />
              </div>

              <div>
                <label
                  htmlFor="owner_user_id"
                  className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
                >
                  {t("admin.tenants.ownerUserId")}
                </label>
                <input
                  id="owner_user_id"
                  {...register("owner_user_id")}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50"
                />
                {errors.owner_user_id && (
                  <p className="mt-1 text-sm text-destructive">
                    {errors.owner_user_id.message}
                  </p>
                )}
              </div>
            </div>

            <button
              type="submit"
              disabled={createTenant.isPending}
              className="rounded-lg bg-primary px-4 py-2 text-[12px] font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
            >
              {createTenant.isPending ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                t("admin.tenants.create")
              )}
            </button>
          </form>
        </div>
      )}

      {tenants && tenants.length > 0 ? (
        <>
          <DataTable
            data={tenants}
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
              className="rounded-lg border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
            >
              {t("common.back")}
            </button>
            <button
              type="button"
              onClick={() =>
                setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
              }
              disabled={(tenants.length ?? 0) < pagination.limit}
              className="rounded-lg border border-border bg-card px-3 py-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
            >
              {t("common.next")}
            </button>
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <p className="text-[13px] text-muted-foreground">
            {t("common.noResults")}
          </p>
        </div>
      )}
    </div>
  );
}
