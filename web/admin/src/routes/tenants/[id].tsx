import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Loader2, Check } from "lucide-react";
import {
  useAdminTenant,
  useUpdateBranding,
  LoadingSpinner,
  formatDate,
  cn,
} from "@remnacore/shared";

const brandingSchema = z.object({
  logo: z.string(),
  primary_color: z.string(),
  app_name: z.string(),
  support_email: z.string().email().or(z.literal("")),
  support_url: z.string().url().or(z.literal("")),
});

type BrandingFormValues = z.infer<typeof brandingSchema>;

export function TenantDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: tenant, isLoading } = useAdminTenant(id);
  const updateBranding = useUpdateBranding();

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
      <div className="text-center py-12">
        <p className="text-[12px] text-red-500">{t("common.error")}</p>
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
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/tenants"
        className="text-[13px] text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-[18px] font-semibold text-foreground">
              {tenant.name}
            </h1>
            <p className="mt-1 font-mono text-[12px] text-muted-foreground">
              {tenant.id}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
              tenant.is_active
                ? "bg-green-500/10 text-green-500"
                : "bg-red-500/10 text-red-500",
            )}
          >
            {tenant.is_active
              ? t("admin.tenants.active")
              : t("admin.tenants.inactive")}
          </span>
        </div>

        <div className="mt-6 grid gap-5 sm:grid-cols-2">
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("admin.tenants.domain")}
            </p>
            <p className={cn(
              "font-mono text-[13px]",
              tenant.domain ? "text-foreground" : "text-muted-foreground",
            )}>
              {tenant.domain ?? "\u2014"}
            </p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("common.createdAt")}
            </p>
            <p className="font-mono text-[13px] text-foreground">
              {formatDate(tenant.created_at)}
            </p>
          </div>
        </div>
      </div>

      {/* Branding form */}
      <div className="rounded-xl border border-border bg-card p-6">
        <h2 className="mb-4 text-[15px] font-semibold text-foreground">
          {t("admin.tenants.branding")}
        </h2>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label
                htmlFor="app_name"
                className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
              >
                {t("admin.tenants.appName")}
              </label>
              <input
                id="app_name"
                {...register("app_name")}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
              />
            </div>

            <div>
              <label
                htmlFor="primary_color"
                className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
              >
                {t("admin.tenants.primaryColor")}
              </label>
              <div className="relative">
                <input
                  id="primary_color"
                  {...register("primary_color")}
                  placeholder="#6366f1"
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 pr-10 text-[13px] text-foreground font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
                />
                {primaryColorValue && (
                  <span
                    className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 rounded border border-border"
                    style={{ backgroundColor: primaryColorValue }}
                  />
                )}
              </div>
            </div>

            <div>
              <label
                htmlFor="logo"
                className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
              >
                {t("admin.tenants.logo")}
              </label>
              <input
                id="logo"
                {...register("logo")}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
              />
            </div>

            <div>
              <label
                htmlFor="support_email"
                className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
              >
                {t("admin.tenants.supportEmail")}
              </label>
              <input
                id="support_email"
                type="email"
                {...register("support_email")}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
              />
              {errors.support_email && (
                <p className="mt-1 text-[12px] text-red-500">
                  {errors.support_email.message}
                </p>
              )}
            </div>

            <div className="sm:col-span-2">
              <label
                htmlFor="support_url"
                className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
              >
                {t("admin.tenants.supportUrl")}
              </label>
              <input
                id="support_url"
                {...register("support_url")}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
              />
              {errors.support_url && (
                <p className="mt-1 text-[12px] text-red-500">
                  {errors.support_url.message}
                </p>
              )}
            </div>
          </div>

          <button
            type="submit"
            disabled={updateBranding.isPending}
            className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background hover:bg-primary/90 transition-colors disabled:opacity-40"
          >
            {updateBranding.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              t("common.save")
            )}
          </button>

          {updateBranding.isSuccess && (
            <p className="flex items-center gap-1 text-[12px] text-emerald-500">
              <Check size={14} />
              {t("profile.updateSuccess")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
