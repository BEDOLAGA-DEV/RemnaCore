import type { Plugin, PluginStatus } from "@remnacore/shared";
import {
  LoadingSpinner,
  useDisablePlugin,
  useEnablePlugin,
  usePlugins,
} from "@remnacore/shared";
import { Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  type Column,
  DataTable,
  PageHeader,
  StatusPill,
  TermButton,
  type Tone,
} from "@/components/ui";

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

export function PluginsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: plugins, isLoading, isError } = usePlugins();
  const enablePlugin = useEnablePlugin();
  const disablePlugin = useDisablePlugin();

  if (isLoading) return <LoadingSpinner />;

  if (isError) {
    return (
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("common.error")}
      </div>
    );
  }

  const togglePending = enablePlugin.isPending || disablePlugin.isPending;

  const columns: Column<Plugin>[] = [
    {
      key: "name",
      header: t("admin.plugins.title"),
      render: (p) => (
        <span className="flex items-center gap-2.5">
          <span className="flex h-[26px] w-[26px] items-center justify-center border border-line bg-input text-[12px] uppercase text-accent">
            {p.name.charAt(0)}
          </span>
          <span className="text-t2">{p.name}</span>
          {p.provides_bot && <StatusPill label="BOT" tone="ok" />}
        </span>
      ),
    },
    {
      key: "slug",
      header: "SLUG",
      render: (p) => (
        <span className="tabular-nums text-t5">
          {p.slug} v{p.version}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (p) => (
        <StatusPill
          label={p.status.toUpperCase()}
          tone={statusTone(p.status)}
        />
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (p) => {
        if (p.status === "error") {
          return <span className="text-t8">—</span>;
        }
        const isEnabled = p.status === "enabled";
        return (
          <TermButton
            type="button"
            variant={isEnabled ? "ghost" : "primary"}
            disabled={togglePending}
            onClick={(e) => {
              e.stopPropagation();
              if (isEnabled) {
                disablePlugin.mutate(p.id);
              } else {
                enablePlugin.mutate(p.id);
              }
            }}
          >
            {isEnabled ? t("admin.plugins.disable") : t("admin.plugins.enable")}
          </TermButton>
        );
      },
    },
  ];

  return (
    <div className="space-y-3.5">
      <PageHeader
        title="PLUGINS"
        breadcrumb={`REMNAWAVE PROVIDER / EXTENSIONS / ${(plugins ?? []).filter((p) => p.status === "enabled").length} ACTIVE`}
        right={
          <Link to="/plugins/install">
            <TermButton type="button" variant="primary">
              + {t("admin.plugins.install")}
            </TermButton>
          </Link>
        }
      />

      <DataTable<Plugin>
        columns={columns}
        rows={plugins ?? []}
        cols="1.6fr 1.2fr .9fr .9fr"
        rowKey={(p) => p.id}
        onRowClick={(p) =>
          navigate({ to: "/plugins/$id", params: { id: p.id } })
        }
        empty={t("admin.plugins.noPlugins").toUpperCase()}
      />
    </div>
  );
}
