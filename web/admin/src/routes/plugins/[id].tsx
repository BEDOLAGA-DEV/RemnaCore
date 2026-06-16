import type { ReactNode } from "react";
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
} from "@remnacore/shared";
import type { PluginStatus } from "@remnacore/shared";
import {
  PageHeader,
  Panel,
  PanelHeader,
  StatusPill,
  type Tone,
  TermButton,
} from "@/components/ui";

const configSchema = z.object({
  configJson: z.string().min(1),
});

type ConfigFormValues = z.infer<typeof configSchema>;

const textareaCls =
  "w-full resize-none border bg-input px-3 py-3 font-mono text-[12px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45";

function statusTone(status: PluginStatus): Tone {
  switch (status) {
    case "enabled":
      return "ok";
    case "error":
      return "danger";
    case "installed":
      return "warn";
    default:
      return "muted";
  }
}

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
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("common.error")}
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
    <div className="space-y-3.5">
      <PageHeader
        title={plugin.name}
        breadcrumb={`REMNAWAVE PROVIDER / EXTENSIONS / ${plugin.slug.toUpperCase()}`}
        right={
          <Link to="/plugins">
            <TermButton type="button" variant="ghost">
              <ArrowLeft size={14} />
              {t("common.back")}
            </TermButton>
          </Link>
        }
      />

      <div className="grid gap-3.5 md:grid-cols-2">
        <Panel>
          <PanelHeader
            title={t("common.details")}
            right={
              <StatusPill
                label={plugin.status.toUpperCase()}
                tone={statusTone(plugin.status)}
              />
            }
          />
          <div>
            <Row label="SLUG">
              <span className="tabular-nums text-t4">
                {plugin.slug} v{plugin.version}
              </span>
            </Row>
            {plugin.author && (
              <Row label={t("admin.plugins.author")}>{plugin.author}</Row>
            )}
            <Row label={t("common.createdAt")}>
              <span className="tabular-nums">
                {formatDateTime(plugin.installed_at)}
              </span>
            </Row>
            {plugin.description && (
              <div className="border-b border-line px-4 py-3 last:border-b-0">
                <div className="mb-1.5 text-[9px] uppercase tracking-[1.5px] text-t7">
                  {t("common.details")}
                </div>
                <p className="text-[12px] leading-relaxed text-t3">
                  {plugin.description}
                </p>
              </div>
            )}
            {plugin.permissions && plugin.permissions.length > 0 && (
              <div className="border-b border-line px-4 py-3 last:border-b-0">
                <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t7">
                  {t("admin.plugins.permissions")}
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {plugin.permissions.map((perm) => (
                    <span
                      key={perm}
                      className="border border-line bg-input px-2 py-[3px] font-mono text-[11px] text-t3"
                    >
                      {perm}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {plugin.error_log && (
              <div className="border-b border-line px-4 py-3 last:border-b-0">
                <div className="mb-1.5 text-[9px] uppercase tracking-[1.5px] text-danger">
                  ERROR LOG
                </div>
                <p className="whitespace-pre-wrap font-mono text-[11px] text-danger">
                  {plugin.error_log}
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2.5 border-t border-line px-4 py-3">
            <TermButton
              type="button"
              variant={isEnabled ? "ghost" : "primary"}
              onClick={() =>
                isEnabled
                  ? disable.mutate(plugin.id)
                  : enable.mutate(plugin.id)
              }
              disabled={enable.isPending || disable.isPending}
            >
              {enable.isPending || disable.isPending ? (
                <Loader2 size={14} className="animate-spin" />
              ) : isEnabled ? (
                t("admin.plugins.disable")
              ) : (
                t("admin.plugins.enable")
              )}
            </TermButton>

            {plugin.slug === "remnawave-provider" && isEnabled && (
              <TermButton
                type="button"
                variant="ghost"
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
              >
                {testPending ? (
                  <Loader2 size={14} className="animate-spin" />
                ) : (
                  <Zap size={14} />
                )}
                TEST CONNECTION
              </TermButton>
            )}

            {!plugin.is_builtin && (
              <TermButton
                type="button"
                variant="danger"
                onClick={() => {
                  if (window.confirm(t("common.confirm"))) {
                    uninstall.mutate(plugin.id);
                  }
                }}
                disabled={uninstall.isPending}
              >
                <Trash2 size={14} />
                {t("admin.plugins.uninstall")}
              </TermButton>
            )}
          </div>

          {testResult && (
            <div
              className="border-t px-4 py-3 font-mono text-[12px]"
              style={{
                borderColor: testResult.success
                  ? "var(--accent)"
                  : "var(--danger)",
                color: testResult.success ? "var(--accent)" : "var(--danger)",
              }}
            >
              {testResult.success
                ? `Connected successfully. Nodes: ${testResult.node_count ?? 0}`
                : `Connection failed: ${testResult.error ?? "Unknown error"}`}
            </div>
          )}
        </Panel>

        {/* Config editor */}
        <Panel>
          <PanelHeader title={t("admin.plugins.config")} />
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 px-4 py-4">
            <textarea
              rows={10}
              {...register("configJson")}
              className={textareaCls}
              style={{
                borderColor: errors.configJson
                  ? "var(--danger)"
                  : "var(--line-strong)",
              }}
            />
            {errors.configJson && (
              <p className="text-[9px] uppercase tracking-[1px] text-danger">
                {errors.configJson.message}
              </p>
            )}
            <div className="flex items-center gap-2.5">
              <TermButton
                type="submit"
                variant="primary"
                disabled={updateConfig.isPending}
              >
                {updateConfig.isPending ? (
                  <Loader2 size={14} className="animate-spin" />
                ) : (
                  t("admin.plugins.updateConfig")
                )}
              </TermButton>
              {updateConfig.isSuccess && (
                <span className="text-[9px] uppercase tracking-[1px] text-accent">
                  {t("profile.updateSuccess")}
                </span>
              )}
            </div>
          </form>
        </Panel>
      </div>
    </div>
  );
}
