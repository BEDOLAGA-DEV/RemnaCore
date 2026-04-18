import { apiGet, cn, LoadingSpinner } from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Server } from "lucide-react";

type HostEntry = {
	panel_id: string;
	host: {
		uuid: string;
		remark: string;
		address: string;
		port: number;
		isDisabled: boolean;
		securityLayer: string;
		sni: string;
		fingerprint: string;
		alpn: string;
		path: string;
		host: string;
	};
};

export default function RemnawaveHosts() {
	const {
		data: hosts,
		isLoading,
		isError,
	} = useQuery({
		queryKey: ["remnawave", "hosts"],
		queryFn: () => apiGet<HostEntry[]>("/api/remnawave/hosts"),
	});

	if (isLoading) return <LoadingSpinner />;

	if (isError) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-16">
				<AlertTriangle size={32} className="text-destructive" />
				<p className="text-[13px] text-destructive">Failed to load hosts</p>
			</div>
		);
	}

	const list: HostEntry[] = Array.isArray(hosts) ? hosts : [];

	return (
		<div className="space-y-6">
			<div className="flex items-center gap-3">
				<Server size={18} className="text-muted-foreground" />
				<div>
					<h1 className="text-[15px] font-semibold text-foreground">Hosts</h1>
					<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
						{list.length} {list.length === 1 ? "host" : "hosts"}
					</p>
				</div>
			</div>

			{list.length === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<Server size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No hosts found</p>
				</div>
			) : (
				<div className="overflow-x-auto rounded-xl border border-border bg-card">
					<table className="w-full">
						<thead>
							<tr className="border-b border-border">
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Remark</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Address</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Port</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Security</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">SNI</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Status</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Panel</th>
							</tr>
						</thead>
						<tbody>
							{list.map((entry) => {
								const h = entry.host;
								return (
									<tr
										key={`${entry.panel_id}-${h.uuid}`}
										className="border-b border-border/50 last:border-0 transition-colors hover:bg-secondary/30"
									>
										<td className="px-4 py-3 text-[13px] font-semibold text-foreground">{h.remark || "—"}</td>
										<td className="px-4 py-3 font-mono text-[12px] text-foreground">{h.address || "—"}</td>
										<td className="px-4 py-3 font-mono text-[12px] text-foreground">{h.port || "—"}</td>
										<td className="px-4 py-3 text-[12px] text-foreground">{h.securityLayer || "none"}</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{h.sni || "—"}</td>
										<td className="px-4 py-3">
											<span
												className={cn(
													"inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium",
													h.isDisabled
														? "bg-red-500/10 text-red-500"
														: "bg-emerald-500/10 text-emerald-500",
												)}
											>
												{h.isDisabled ? "disabled" : "active"}
											</span>
										</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{entry.panel_id}</td>
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
