import { apiGet, LoadingSpinner } from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, Boxes } from "lucide-react";

type SquadEntry = {
	panel_id: string;
	squad: {
		uuid: string;
		name: string;
		info?: { membersCount?: number } | null;
		createdAt?: string;
		updatedAt?: string;
	};
};

export default function RemnawaveSquadsExternal() {
	const {
		data: squads,
		isLoading,
		isError,
	} = useQuery({
		queryKey: ["remnawave", "squads", "external"],
		queryFn: () => apiGet<SquadEntry[]>("/api/remnawave/squads/external"),
	});

	if (isLoading) return <LoadingSpinner />;

	if (isError) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-16">
				<AlertTriangle size={32} className="text-destructive" />
				<p className="text-[13px] text-destructive">Failed to load external squads</p>
			</div>
		);
	}

	const list: SquadEntry[] = Array.isArray(squads) ? squads : [];

	return (
		<div className="space-y-6">
			<div className="flex items-center gap-3">
				<Boxes size={18} className="text-muted-foreground" />
				<div>
					<h1 className="text-[15px] font-semibold text-foreground">External Squads</h1>
					<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
						{list.length} {list.length === 1 ? "squad" : "squads"}
					</p>
				</div>
			</div>

			{list.length === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<Boxes size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No external squads found</p>
				</div>
			) : (
				<div className="rounded-xl border border-border bg-card">
					<table className="w-full">
						<thead>
							<tr className="border-b border-border">
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Name</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Panel</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Members</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">UUID</th>
							</tr>
						</thead>
						<tbody>
							{list.map((entry) => {
								const s = entry.squad;
								const info = s.info as Record<string, unknown> | null;
								return (
									<tr
										key={`${entry.panel_id}-${s.uuid}`}
										className="border-b border-border/50 last:border-0 transition-colors hover:bg-secondary/30"
									>
										<td className="px-4 py-3 text-[13px] font-semibold text-foreground">{s.name}</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{entry.panel_id}</td>
										<td className="px-4 py-3 text-[13px] text-foreground">
											{info?.membersCount != null ? String(info.membersCount) : "—"}
										</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{s.uuid}</td>
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
