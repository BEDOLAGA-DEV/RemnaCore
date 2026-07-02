import type { Plugin, PluginStatus } from "@remnacore/shared";
import { cn, useDisablePlugin, useEnablePlugin } from "@remnacore/shared";
import { Link } from "@tanstack/react-router";
import { ExternalLink, Loader2, Puzzle } from "lucide-react";
import { useTranslation } from "react-i18next";

type PluginCardProps = {
  plugin: Plugin;
};

function statusColor(status: PluginStatus): string {
  const colors: Record<PluginStatus, string> = {
    enabled: "bg-green-500/10 text-green-500",
    installed: "bg-blue-500/10 text-blue-500",
    disabled: "bg-gray-500/10 text-gray-500",
    error: "bg-red-500/10 text-red-500",
  };
  return colors[status];
}

export function PluginCard({ plugin }: PluginCardProps) {
  const { t } = useTranslation();
  const enablePlugin = useEnablePlugin();
  const disablePlugin = useDisablePlugin();

  const isEnabled = plugin.status === "enabled";
  const togglePending = enablePlugin.isPending || disablePlugin.isPending;

  const handleToggle = () => {
    if (isEnabled) {
      disablePlugin.mutate(plugin.id);
    } else {
      enablePlugin.mutate(plugin.id);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="relative rounded-lg bg-muted p-2">
            <Puzzle size={20} className="text-primary" />
            {isEnabled && (
              <span className="absolute -right-0.5 -top-0.5 size-2 rounded-full bg-green-500" />
            )}
          </div>
          <div>
            <h3 className="text-[13px] font-medium text-foreground">
              {plugin.name}
            </h3>
            <p className="font-mono text-[11px] text-muted-foreground">
              {plugin.slug} v{plugin.version}
            </p>
          </div>
        </div>
        <Link
          to="/plugins/$id"
          params={{ id: plugin.id }}
          className="rounded-lg p-2 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          <ExternalLink size={16} />
        </Link>
      </div>

      {plugin.description && (
        <p className="mt-3 text-sm text-muted-foreground">
          {plugin.description}
        </p>
      )}

      <div className="mt-4 flex items-center justify-between">
        <span
          className={cn(
            "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
            statusColor(plugin.status),
          )}
        >
          {plugin.status}
        </span>
        <button
          type="button"
          onClick={handleToggle}
          disabled={togglePending || plugin.status === "error"}
          className={cn(
            "rounded-lg px-3 py-1 text-[11px] font-medium transition-colors disabled:opacity-50",
            isEnabled
              ? "bg-muted text-foreground hover:bg-muted/80"
              : "bg-primary text-primary-foreground hover:bg-primary/90",
          )}
        >
          {togglePending ? (
            <Loader2 size={12} className="animate-spin" />
          ) : isEnabled ? (
            t("admin.plugins.disable")
          ) : (
            t("admin.plugins.enable")
          )}
        </button>
      </div>
    </div>
  );
}
