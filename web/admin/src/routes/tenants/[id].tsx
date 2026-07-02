import { zodResolver } from "@hookform/resolvers/zod";
import {
  formatDate,
  LoadingSpinner,
  useAdminTenant,
  useBotPlugins,
  useTenantBot,
  useUpdateBranding,
  useUpdateTenantBot,
} from "@remnacore/shared";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Check, Loader2 } from "lucide-react";
import type { ReactNode } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { ShopBotForm } from "@/components/shop-bot/ShopBotForm";
import {
  PageHeader,
  Panel,
  PanelHeader,
  StatusPill,
  TermButton,
  TermInput,
} from "@/components/ui";

const brandingSchema = z.object({
  logo: z.string(),
  primary_color: z.string(),
  app_name: z.string(),
  support_email: z.email().or(z.literal("")),
  support_url: z.url().or(z.literal("")),
});

type BrandingFormValues = z.infer<typeof brandingSchema>;

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-line px-4 py-3 last:border-b-0">
      <span className="text-[9px] uppercase tracking-[1.5px] text-t7">
        {label}
      </span>
      <span className="text-right text-[12px] text-t2">{children}</span>
    </div>
  );
}

export function TenantDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: tenant, isLoading } = useAdminTenant(id);
  const updateBranding = useUpdateBranding();
  const { data: botConfig } = useTenantBot(id);
  const { data: botPlugins } = useBotPlugins();
  const updateTenantBot = useUpdateTenantBot();

  const {
    register,
    handleSubmit,
    formState: { errors },
    watch,
  } = useForm<BrandingFormValues>({
    resolver: zodResolver(brandingSchema),
    values: {
      logo: tenant?.branding_config?.logo ?? "",
      primary_color: tenant?.branding_config?.primary_color ?? "",
      app_name: tenant?.branding_config?.app_name ?? "",
      support_email: tenant?.branding_config?.support_email ?? "",
      support_url: tenant?.branding_config?.support_url ?? "",
    },
  });

  const primaryColorValue = watch("primary_color");

  if (isLoading) return <LoadingSpinner />;

  if (!tenant) {
    return (
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("common.error")}
      </div>
    );
  }

  const onSubmit = (data: BrandingFormValues) => {
    updateBranding.mutate({
      tenantId: tenant.id,
      data,
    });
  };

  return (
    <div className="space-y-3.5">
      <PageHeader
        title={tenant.name}
        breadcrumb="REMNAWAVE PROVIDER / TENANTS / DETAIL"
        right={
          <Link to="/tenants">
            <TermButton type="button" variant="ghost">
              <ArrowLeft size={14} />
              {t("common.back")}
            </TermButton>
          </Link>
        }
      />

      <Panel>
        <PanelHeader
          title={t("common.details")}
          right={
            tenant.is_active ? (
              <StatusPill
                label={t("admin.tenants.active").toUpperCase()}
                tone="ok"
              />
            ) : (
              <StatusPill
                label={t("admin.tenants.inactive").toUpperCase()}
                tone="danger"
              />
            )
          }
        />
        <div>
          <Row label="ID">
            <span className="font-mono text-[11px] tabular-nums text-t5">
              {tenant.id}
            </span>
          </Row>
          <Row label={t("admin.tenants.domain")}>
            {tenant.domain ? (
              <span className="tabular-nums text-t2">{tenant.domain}</span>
            ) : (
              <span className="text-t6">—</span>
            )}
          </Row>
          <Row label={t("common.createdAt")}>
            <span className="tabular-nums">
              {formatDate(tenant.created_at)}
            </span>
          </Row>
        </div>
      </Panel>

      {/* Branding form */}
      <Panel>
        <PanelHeader title={t("admin.tenants.branding")} />
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 p-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <TermInput
              id="app_name"
              label={t("admin.tenants.appName")}
              {...register("app_name")}
            />

            <div>
              <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
                {t("admin.tenants.primaryColor")}
              </div>
              <div className="relative">
                <input
                  id="primary_color"
                  {...register("primary_color")}
                  placeholder="#6366f1"
                  className="w-full border bg-input px-3 py-3 pr-10 text-[13px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45"
                  style={{ borderColor: "var(--line-strong)" }}
                />
                {primaryColorValue && (
                  <span
                    className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 border border-line"
                    style={{ backgroundColor: primaryColorValue }}
                  />
                )}
              </div>
            </div>

            <TermInput
              id="logo"
              label={t("admin.tenants.logo")}
              {...register("logo")}
            />

            <TermInput
              id="support_email"
              type="email"
              label={t("admin.tenants.supportEmail")}
              error={errors.support_email?.message}
              {...register("support_email")}
            />

            <div className="sm:col-span-2">
              <TermInput
                id="support_url"
                label={t("admin.tenants.supportUrl")}
                error={errors.support_url?.message}
                {...register("support_url")}
              />
            </div>
          </div>

          <TermButton
            type="submit"
            variant="primary"
            disabled={updateBranding.isPending}
          >
            {updateBranding.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              t("common.save")
            )}
          </TermButton>

          {updateBranding.isSuccess && (
            <p className="flex items-center gap-1.5 text-[10px] uppercase tracking-[1px] text-accent">
              <Check size={14} />
              {t("profile.updateSuccess")}
            </p>
          )}
        </form>
      </Panel>

      {/* Telegram Bot panel */}
      <Panel>
        <PanelHeader title={t("admin.tenantBot.title")} />
        <ShopBotForm
          config={botConfig}
          plugins={botPlugins}
          onSubmit={(data) => updateTenantBot.mutate({ tenantId: id, data })}
          isPending={updateTenantBot.isPending}
          isSuccess={updateTenantBot.isSuccess}
        />
      </Panel>
    </div>
  );
}
