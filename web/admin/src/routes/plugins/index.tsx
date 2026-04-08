import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { usePlugins, LoadingSpinner } from "@remnacore/shared";
import { PluginCard } from "../../components/PluginCard.js";

export function PluginsPage() {
  const { t } = useTranslation();
  const { data: plugins, isLoading, isError } = usePlugins();

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold text-foreground">
          {t("admin.plugins.title")}
        </h1>
        <Link
          to="/plugins/install"
          className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
        >
          <Plus size={14} />
          {t("admin.plugins.install")}
        </Link>
      </div>

      {plugins && plugins.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {plugins.map((plugin) => (
            <PluginCard key={plugin.id} plugin={plugin} />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <p className="text-[13px] text-muted-foreground">
            {t("admin.plugins.noPlugins")}
          </p>
          <Link
            to="/plugins/install"
            className="mt-4 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
          >
            {t("admin.plugins.install")}
          </Link>
        </div>
      )}
    </div>
  );
}
