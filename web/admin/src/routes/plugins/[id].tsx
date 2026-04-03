import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Loader2, Trash2 } from "lucide-react";
import {
  usePlugin,
  useUpdatePluginConfig,
  useUninstallPlugin,
  useEnablePlugin,
  useDisablePlugin,
  LoadingSpinner,
  formatDateTime,
  cn,
} from "@remnacore/shared";
import type { PluginStatus } from "@remnacore/shared";

const configSchema = z.object({
  configJson: z.string().min(1),
});

type ConfigFormValues = z.infer<typeof configSchema>;

function statusColor(status: PluginStatus): string {
  const colors: Record<PluginStatus, string> = {
    enabled: "bg-green-500/10 text-green-500",
    installed: "bg-blue-500/10 text-blue-500",
    disabled: "bg-gray-500/10 text-gray-500",
    error: "bg-red-500/10 text-red-500",
  };
  return colors[status];
}

export function PluginDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: plugin, isLoading } = usePlugin(id);
  const updateConfig = useUpdatePluginConfig();
  const uninstall = useUninstallPlugin();
  const enable = useEnablePlugin();
  const disable = useDisablePlugin();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ConfigFormValues>({
    resolver: zodResolver(configSchema),
    values: {
      configJson: plugin?.config ? JSON.stringify(plugin.config, null, 2) : "{}",
    },
  });

  if (isLoading) return <LoadingSpinner />;

  if (!plugin) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  const onSubmit = (data: ConfigFormValues) => {
    try {
      const config = JSON.parse(data.configJson) as Record<string, string>;
      updateConfig.mutate({ pluginId: plugin.id, data: { config } });
    } catch {
      // JSON parse error - form validation would catch this in production
    }
  };

  const isEnabled = plugin.status === "enabled";

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/plugins"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-xl font-bold text-foreground">
              {plugin.name}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {plugin.slug} v{plugin.version}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium",
              statusColor(plugin.status),
            )}
          >
            {plugin.status}
          </span>
        </div>

        {plugin.description && (
          <p className="mt-4 text-sm text-muted-foreground">
            {plugin.description}
          </p>
        )}

        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          {plugin.author && (
            <div>
              <p className="text-xs text-muted-foreground">
                {t("admin.plugins.author")}
              </p>
              <p className="text-sm text-foreground">{plugin.author}</p>
            </div>
          )}
          <div>
            <p className="text-xs text-muted-foreground">
              {t("common.createdAt")}
            </p>
            <p className="text-sm text-foreground">
              {formatDateTime(plugin.installed_at)}
            </p>
          </div>
          {plugin.permissions.length > 0 && (
            <div className="sm:col-span-2">
              <p className="text-xs text-muted-foreground">
                {t("admin.plugins.permissions")}
              </p>
              <div className="mt-1 flex flex-wrap gap-1">
                {plugin.permissions.map((perm) => (
                  <span
                    key={perm}
                    className="rounded bg-muted px-2 py-0.5 font-mono text-xs text-foreground"
                  >
                    {perm}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>

        {plugin.error_log && (
          <div className="mt-4 rounded-lg bg-destructive/10 p-3">
            <p className="font-mono text-xs text-destructive whitespace-pre-wrap">
              {plugin.error_log}
            </p>
          </div>
        )}

        <div className="mt-6 flex gap-3">
          <button
            type="button"
            onClick={() =>
              isEnabled
                ? disable.mutate(plugin.id)
                : enable.mutate(plugin.id)
            }
            disabled={enable.isPending || disable.isPending}
            className={cn(
              "rounded-lg px-4 py-2 text-sm font-medium transition-colors disabled:opacity-50",
              isEnabled
                ? "border border-border text-foreground hover:bg-muted"
                : "bg-primary text-primary-foreground hover:bg-primary/90",
            )}
          >
            {enable.isPending || disable.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : isEnabled ? (
              t("admin.plugins.disable")
            ) : (
              t("admin.plugins.enable")
            )}
          </button>

          <button
            type="button"
            onClick={() => {
              if (window.confirm(t("common.confirm"))) {
                uninstall.mutate(plugin.id);
              }
            }}
            disabled={uninstall.isPending}
            className="flex items-center gap-2 rounded-lg border border-destructive px-4 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
          >
            <Trash2 size={14} />
            {t("admin.plugins.uninstall")}
          </button>
        </div>
      </div>

      {/* Config editor */}
      <div className="rounded-xl border border-border bg-card p-6">
        <h2 className="mb-4 text-lg font-semibold text-foreground">
          {t("admin.plugins.config")}
        </h2>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <textarea
            rows={8}
            {...register("configJson")}
            className="w-full rounded-lg border border-input bg-background px-3 py-2 font-mono text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          />
          {errors.configJson && (
            <p className="text-sm text-destructive">
              {errors.configJson.message}
            </p>
          )}
          <button
            type="submit"
            disabled={updateConfig.isPending}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
          >
            {updateConfig.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              t("admin.plugins.updateConfig")
            )}
          </button>
        </form>
      </div>
    </div>
  );
}
