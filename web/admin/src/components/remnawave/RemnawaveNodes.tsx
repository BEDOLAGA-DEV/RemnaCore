import { apiGet, apiPost, cn } from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Power, PowerOff, RotateCcw, Server } from "lucide-react";
import { useState } from "react";

// ─── Types ───────────────────────────────────────────────────────────────────

type RemnawaveNode = {
	panel_id: string;
	node: {
		uuid: string;
		name: string;
		address: string;
		port: number;
		isConnected: boolean;
		trafficUsedBytes: number;
	};
};

type NodeAction = "restart" | "enable" | "disable";

// ─── Constants ───────────────────────────────────────────────────────────────

const NODES_QUERY_KEY = ["remnawave", "nodes"] as const;
const BYTES_PER_GB = 1024 * 1024 * 1024;

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatTrafficGB(bytes: number): string {
	return `${(bytes / BYTES_PER_GB).toFixed(2)} GB`;
}

function buildActionUrl(panelID: string, nodeUUID: string, action: NodeAction): string {
	return `/api/remnawave/nodes/${panelID}/${nodeUUID}/${action}`;
}

// ─── Main Component ──────────────────────────────────────────────────────────

function RemnawaveNodes() {
	const queryClient = useQueryClient();

	const {
		data: nodes,
		isLoading,
		isError,
	} = useQuery({
		queryKey: NODES_QUERY_KEY,
		queryFn: () => apiGet<RemnawaveNode[]>("/api/remnawave/nodes"),
	});

	const [pendingAction, setPendingAction] = useState<string | null>(null);

	const mutation = useMutation({
		mutationFn: ({ panelID, nodeUUID, action }: { panelID: string; nodeUUID: string; action: NodeAction }) =>
			apiPost(buildActionUrl(panelID, nodeUUID, action), {}),
		onMutate: ({ panelID, nodeUUID, action }) => {
			setPendingAction(`${panelID}:${nodeUUID}:${action}`);
		},
		onSettled: () => {
			setPendingAction(null);
			queryClient.invalidateQueries({ queryKey: NODES_QUERY_KEY });
		},
	});

	function handleAction(panelID: string, nodeUUID: string, action: NodeAction) {
		mutation.mutate({ panelID, nodeUUID, action });
	}

	function isActionPending(panelID: string, nodeUUID: string, action: NodeAction): boolean {
		return pendingAction === `${panelID}:${nodeUUID}:${action}`;
	}

	// ─── Loading ─────────────────────────────────────────────────────────────

	if (isLoading) {
		return (
			<div className="flex items-center justify-center py-20">
				<Loader2 size={24} className="animate-spin text-muted-foreground" />
			</div>
		);
	}

	// ─── Error ───────────────────────────────────────────────────────────────

	if (isError) {
		return (
			<div className="py-12 text-center">
				<p className="text-destructive">Failed to load nodes.</p>
			</div>
		);
	}

	// API may return the array directly or inside a wrapper — handle both
	const rawData = nodes as unknown;
	const nodeList: RemnawaveNode[] = Array.isArray(rawData) ? rawData : [];

	// ─── Render ──────────────────────────────────────────────────────────────

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-3">
					<Server size={18} className="text-muted-foreground" />
					<div>
						<h1 className="text-[15px] font-semibold text-foreground">Nodes</h1>
						<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
							{nodeList.length} {nodeList.length === 1 ? "node" : "nodes"}
						</p>
					</div>
				</div>
			</div>

			{/* Empty state */}
			{nodeList.length === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<Server size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No nodes found.</p>
				</div>
			) : (
				/* Table */
				<div className="overflow-x-auto rounded-xl border border-border">
					<table className="w-full">
						<thead className="border-b border-border">
							<tr>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Name
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Panel ID
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Address
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Status
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Traffic Used
								</th>
								<th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Actions
								</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-border">
							{nodeList.map((entry) => {
								const { panel_id, node } = entry;
								return (
									<tr
										key={`${panel_id}:${node.uuid}`}
										className="transition-colors hover:bg-secondary/30"
									>
										{/* Name */}
										<td className="px-4 py-3 text-[13px] font-semibold text-foreground">
											{node.name}
										</td>

										{/* Panel ID */}
										<td className="px-4 py-3">
											<span className="font-mono text-[12px] text-muted-foreground">
												{panel_id}
											</span>
										</td>

										{/* Address:Port */}
										<td className="px-4 py-3 font-mono text-[12px] text-foreground">
											{node.address}:{node.port}
										</td>

										{/* Status */}
										<td className="px-4 py-3">
											<span
												className={cn(
													"inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-mono font-medium",
													node.isConnected
														? "bg-green-500/10 text-green-500"
														: "bg-red-500/10 text-red-500",
												)}
											>
												<span
													className={cn(
														"h-1.5 w-1.5 rounded-full",
														node.isConnected
															? "bg-green-500 shadow-[0_0_4px_theme(colors.green.500)]"
															: "bg-red-500 shadow-[0_0_4px_theme(colors.red.500)]",
													)}
												/>
												{node.isConnected ? "Connected" : "Disconnected"}
											</span>
										</td>

										{/* Traffic Used */}
										<td className="px-4 py-3 font-mono text-[12px] text-foreground">
											{formatTrafficGB(node.trafficUsedBytes)}
										</td>

										{/* Actions */}
										<td className="px-4 py-3">
											<div className="flex items-center justify-end gap-1">
												<button
													type="button"
													title="Restart"
													disabled={isActionPending(panel_id, node.uuid, "restart")}
													onClick={() => handleAction(panel_id, node.uuid, "restart")}
													className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:opacity-40"
												>
													{isActionPending(panel_id, node.uuid, "restart") ? (
														<Loader2 size={14} className="animate-spin" />
													) : (
														<RotateCcw size={14} />
													)}
												</button>
												<button
													type="button"
													title="Enable"
													disabled={isActionPending(panel_id, node.uuid, "enable")}
													onClick={() => handleAction(panel_id, node.uuid, "enable")}
													className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-green-500/10 hover:text-green-500 disabled:opacity-40"
												>
													{isActionPending(panel_id, node.uuid, "enable") ? (
														<Loader2 size={14} className="animate-spin" />
													) : (
														<Power size={14} />
													)}
												</button>
												<button
													type="button"
													title="Disable"
													disabled={isActionPending(panel_id, node.uuid, "disable")}
													onClick={() => handleAction(panel_id, node.uuid, "disable")}
													className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive disabled:opacity-40"
												>
													{isActionPending(panel_id, node.uuid, "disable") ? (
														<Loader2 size={14} className="animate-spin" />
													) : (
														<PowerOff size={14} />
													)}
												</button>
											</div>
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			)}
		</div>
	);
}

export default RemnawaveNodes;
