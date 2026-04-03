import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Puzzle, ExternalLink, Loader2 } from "lucide-react";
import { cn } from "@remnacore/shared";
import { useEnablePlugin, useDisablePlugin } from "@remnacore/shared";
import type { Plugin, PluginStatus } from "@remnacore/shared";

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
    <div className="rounded-xl border border-border bg-card p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="rounded-lg bg-muted p-2">
            <Puzzle size={20} className="text-primary" />
          </div>
          <div>
            <h3 className="font-semibold text-foreground">{plugin.name}</h3>
            <p className="text-xs text-muted-foreground">
              {plugin.slug} v{plugin.version}
            </p>
          </div>
        </div>
        <Link
          to="/plugins/$id"
          params={{ id: plugin.id }}
          className="rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground"
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
            "rounded-full px-2.5 py-0.5 text-xs font-medium",
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
            "rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50",
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
