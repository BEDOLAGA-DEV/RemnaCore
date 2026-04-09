import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Loader2, Trash2, Zap } from "lucide-react";
import { useState } from "react";
import {
  usePlugin,
  useUpdatePluginConfig,
  useUninstallPlugin,
  useEnablePlugin,
  useDisablePlugin,
  apiPost,
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
  const [testResult, setTestResult] = useState<{ success: boolean; message?: string; error?: string; node_count?: number } | null>(null);
  const [testPending, setTestPending] = useState(false);

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
        <p className="text-[12px] text-red-500">{t("common.error")}</p>
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
        className="text-[13px] text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-[18px] font-semibold text-foreground">
              {plugin.name}
            </h1>
            <p className="mt-1 font-mono text-[12px] text-muted-foreground">
              {plugin.slug} v{plugin.version}
            </p>
          </div>
          <span
            className={cn(
              "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
              statusColor(plugin.status),
            )}
          >
            {plugin.status}
          </span>
        </div>

        {plugin.description && (
          <p className="mt-4 text-[13px] text-muted-foreground leading-relaxed">
            {plugin.description}
          </p>
        )}

        <div className="mt-6 grid gap-5 sm:grid-cols-2">
          {plugin.author && (
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                {t("admin.plugins.author")}
              </p>
              <p className="text-[13px] text-foreground">{plugin.author}</p>
            </div>
          )}
          <div>
            <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
              {t("common.createdAt")}
            </p>
            <p className="font-mono text-[13px] text-foreground">
              {formatDateTime(plugin.installed_at)}
            </p>
          </div>
          {plugin.permissions && plugin.permissions.length > 0 && (
            <div className="sm:col-span-2">
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                {t("admin.plugins.permissions")}
              </p>
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {plugin.permissions.map((perm) => (
                  <span
                    key={perm}
                    className="rounded bg-secondary px-2 py-0.5 font-mono text-[11px] text-foreground"
                  >
                    {perm}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>

        {plugin.error_log && (
          <div className="mt-4 rounded-lg bg-red-500/5 border border-red-500/20 p-3">
            <p className="font-mono text-[11px] text-red-400 whitespace-pre-wrap">
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
              "rounded-lg px-4 py-2 text-[13px] font-medium transition-colors disabled:opacity-40",
              isEnabled
                ? "border border-border bg-card text-foreground hover:bg-secondary"
                : "bg-primary text-background hover:bg-primary/90",
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

          {plugin.slug === "remnawave-provider" && isEnabled && (
            <button
              type="button"
              onClick={async () => {
                setTestPending(true);
                setTestResult(null);
                try {
                  const res = await apiPost<{ success: boolean; message?: string; error?: string; node_count?: number }>("/api/remnawave/test-connection", {});
                  setTestResult(res);
                } catch {
                  setTestResult({ success: false, error: "Request failed" });
                } finally {
                  setTestPending(false);
                }
              }}
              disabled={testPending}
              className="flex items-center gap-2 rounded-lg border border-primary/30 px-4 py-2 text-[13px] font-medium text-primary hover:bg-primary/10 transition-colors disabled:opacity-40"
            >
              {testPending ? <Loader2 size={14} className="animate-spin" /> : <Zap size={14} />}
              Test Connection
            </button>
          )}

          {!plugin.is_builtin && (
            <button
              type="button"
              onClick={() => {
                if (window.confirm(t("common.confirm"))) {
                  uninstall.mutate(plugin.id);
                }
              }}
              disabled={uninstall.isPending}
              className="flex items-center gap-2 rounded-lg border border-red-500/30 text-red-500 px-4 py-2 text-[13px] font-medium hover:bg-red-500/10 transition-colors disabled:opacity-40"
            >
              <Trash2 size={14} />
              {t("admin.plugins.uninstall")}
            </button>
          )}
        </div>

        {testResult && (
          <div className={cn(
            "mt-4 rounded-lg p-3 font-mono text-[12px]",
            testResult.success
              ? "bg-emerald-500/10 border border-emerald-500/20 text-emerald-400"
              : "bg-red-500/10 border border-red-500/20 text-red-400",
          )}>
            {testResult.success
              ? `Connected successfully. Nodes: ${testResult.node_count ?? 0}`
              : `Connection failed: ${testResult.error ?? "Unknown error"}`}
          </div>
        )}
      </div>

      {/* Config editor */}
      <div className="rounded-xl border border-border bg-card p-6">
        <h2 className="mb-4 text-[15px] font-semibold text-foreground">
          {t("admin.plugins.config")}
        </h2>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <textarea
            rows={8}
            {...register("configJson")}
            className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[12px] text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 resize-none transition-colors"
          />
          {errors.configJson && (
            <p className="text-[12px] text-red-500">
              {errors.configJson.message}
            </p>
          )}
          <button
            type="submit"
            disabled={updateConfig.isPending}
            className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background hover:bg-primary/90 transition-colors disabled:opacity-40"
          >
            {updateConfig.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              t("admin.plugins.updateConfig")
            )}
          </button>
          {updateConfig.isSuccess && (
            <p className="text-[12px] text-emerald-500">
              {t("profile.updateSuccess")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
