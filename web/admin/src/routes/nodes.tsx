import { useTranslation } from "react-i18next";
import { Server } from "lucide-react";
import { PageHeader, Panel } from "@/components/ui";

/**
 * Nodes page displays Remnawave node health from smart router data.
 * Currently a placeholder -- the backend does not expose a node health
 * endpoint yet. This page provides the UI foundation for when the
 * `/api/admin/nodes` endpoint is added.
 */
export function NodesPage() {
  const { t } = useTranslation();

  return (
    <div className="space-y-3.5">
      <PageHeader
        title="NODES"
        breadcrumb="REMNAWAVE PROVIDER / INFRASTRUCTURE"
      />

      <Panel scanline className="flex flex-col items-center justify-center gap-4 px-4 py-16 text-center">
        <span className="flex h-12 w-12 items-center justify-center border border-line bg-input text-t6">
          <Server size={20} />
        </span>
        <div className="text-[13px] uppercase tracking-[2px] text-t4">
          NO NODE ENDPOINT
        </div>
        <p className="max-w-md text-[11px] uppercase tracking-[1px] text-t7">
          {t("admin.nodes.noNodes")}
        </p>
      </Panel>
    </div>
  );
}
