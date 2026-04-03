import { useTranslation } from "react-i18next";
import { Server } from "lucide-react";
import { NodeHealthBadge } from "../components/NodeHealthBadge.js";

/**
 * Nodes page displays Remnawave node health from smart router data.
 * Currently a placeholder -- the backend does not expose a node health
 * endpoint yet. This page provides the UI foundation for when the
 * `/api/admin/nodes` endpoint is added.
 */
export function NodesPage() {
  const { t } = useTranslation();

  // Placeholder data -- replace with a query hook when the endpoint exists
  const nodes: {
    id: string;
    name: string;
    address: string;
    health: "healthy" | "degraded" | "down";
  }[] = [];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-foreground">
        {t("admin.nodes.title")}
      </h1>

      {nodes.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {nodes.map((node) => (
            <div
              key={node.id}
              className="rounded-xl border border-border bg-card p-5"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Server size={18} className="text-muted-foreground" />
                  <div>
                    <p className="font-medium text-foreground">{node.name}</p>
                    <p className="font-mono text-xs text-muted-foreground">
                      {node.address}
                    </p>
                  </div>
                </div>
                <NodeHealthBadge health={node.health} />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <Server size={48} className="text-muted-foreground" />
          <p className="mt-4 text-muted-foreground">
            {t("admin.nodes.noNodes")}
          </p>
        </div>
      )}
    </div>
  );
}
