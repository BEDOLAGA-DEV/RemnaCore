import { zodResolver } from "@hookform/resolvers/zod";
import type { Tenant } from "@remnacore/shared";
import {
  formatDate,
  useAdminTenants,
  useCreateTenant,
} from "@remnacore/shared";
import { useNavigate } from "@tanstack/react-router";
import { Loader2, Plus } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  type Column,
  DataTable,
  PageHeader,
  Panel,
  PanelHeader,
  StatusPill,
  TermButton,
  TermInput,
} from "@/components/ui";

const createTenantSchema = z.object({
  name: z.string().min(1),
  domain: z.string().optional(),
  owner_user_id: z.string().min(1),
});

type CreateTenantValues = z.infer<typeof createTenantSchema>;

export function TenantsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [showForm, setShowForm] = useState(false);
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const {
    data: tenants,
    isLoading,
    error,
    refetch,
  } = useAdminTenants(pagination);
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

  const columns: Column<Tenant>[] = [
    {
      key: "name",
      header: t("common.name"),
      render: (tn) => <span className="text-t2">{tn.name}</span>,
    },
    {
      key: "domain",
      header: t("admin.tenants.domain"),
      render: (tn) => (
        <span className="tabular-nums text-t5">{tn.domain ?? "—"}</span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (tn) =>
        tn.is_active ? (
          <StatusPill
            label={t("admin.tenants.active").toUpperCase()}
            tone="ok"
          />
        ) : (
          <StatusPill
            label={t("admin.tenants.inactive").toUpperCase()}
            tone="danger"
          />
        ),
    },
    {
      key: "created",
      header: t("common.createdAt"),
      render: (tn) => (
        <span className="tabular-nums text-t5">
          {formatDate(tn.created_at)}
        </span>
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
      <PageHeader
        title="TENANTS"
        breadcrumb="REMNAWAVE PROVIDER / TENANTS"
        right={
          <TermButton
            type="button"
            variant="primary"
            onClick={() => setShowForm(!showForm)}
          >
            <Plus size={14} />
            {t("admin.tenants.create")}
          </TermButton>
        }
      />

      {showForm && (
        <Panel>
          <PanelHeader title={t("admin.tenants.create")} />
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 p-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <TermInput
                id="name"
                label={t("common.name")}
                error={errors.name ? t("common.name") : undefined}
                {...register("name")}
              />
              <TermInput
                id="domain"
                label={t("admin.tenants.domain")}
                {...register("domain")}
              />
              <TermInput
                id="owner_user_id"
                label={t("admin.tenants.ownerUserId")}
                error={
                  errors.owner_user_id
                    ? t("admin.tenants.ownerUserId")
                    : undefined
                }
                {...register("owner_user_id")}
              />
            </div>

            <TermButton
              type="submit"
              variant="primary"
              disabled={createTenant.isPending}
            >
              {createTenant.isPending ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                t("admin.tenants.create")
              )}
            </TermButton>
          </form>
        </Panel>
      )}

      <DataTable<Tenant>
        columns={columns}
        rows={isLoading ? [] : (tenants ?? [])}
        error={error}
        onRetry={refetch}
        cols="1.6fr 1fr .9fr 1fr 40px"
        rowKey={(tn) => tn.id}
        onRowClick={(tn) =>
          navigate({ to: "/tenants/$id", params: { id: tn.id } })
        }
        empty={
          isLoading
            ? t("common.loading").toUpperCase()
            : t("common.noResults").toUpperCase()
        }
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
          disabled={(tenants?.length ?? 0) < pagination.limit}
        >
          {t("common.next")}
        </TermButton>
      </div>
    </div>
  );
}
