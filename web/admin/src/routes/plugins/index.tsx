import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Plus, Puzzle } from "lucide-react";
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
        <h1 className="text-2xl font-bold text-foreground">
          {t("admin.plugins.title")}
        </h1>
        <Link
          to="/plugins/install"
          className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <Plus size={16} />
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
          <Puzzle size={48} className="text-muted-foreground" />
          <p className="mt-4 text-muted-foreground">
            {t("admin.plugins.noPlugins")}
          </p>
          <Link
            to="/plugins/install"
            className="mt-4 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            {t("admin.plugins.install")}
          </Link>
        </div>
      )}
    </div>
  );
}
