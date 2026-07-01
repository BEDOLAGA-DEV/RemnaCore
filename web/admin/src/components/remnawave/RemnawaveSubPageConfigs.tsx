import { apiGet, ENDPOINTS, LoadingSpinner } from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, FileText } from "lucide-react";

type ConfigEntry = {
	panel_id: string;
	config: Record<string, unknown>;
};

export default function RemnawaveSubPageConfigs() {
	const {
		data: configs,
		isLoading,
		isError,
	} = useQuery({
		queryKey: ["remnawave", "subscription-page-configs"],
		queryFn: () => apiGet<ConfigEntry[]>(ENDPOINTS.remnawave.subPageConfigs),
	});

	if (isLoading) return <LoadingSpinner />;

	if (isError) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-16">
				<AlertTriangle size={32} className="text-destructive" />
				<p className="text-[13px] text-destructive">Failed to load subscription page configs</p>
			</div>
		);
	}

	const list: ConfigEntry[] = Array.isArray(configs) ? configs : [];

	return (
		<div className="space-y-6">
			<div className="flex items-center gap-3">
				<FileText size={18} className="text-muted-foreground" />
				<div>
					<h1 className="text-[15px] font-semibold text-foreground">Subscription Page Configs</h1>
					<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
						{list.length} {list.length === 1 ? "config" : "configs"}
					</p>
				</div>
			</div>

			{list.length === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<FileText size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No subscription page configs found</p>
				</div>
			) : (
				<div className="grid gap-4">
					{list.map((entry, i) => {
						const cfg = entry.config;
						const uuid = String(cfg.uuid ?? `config-${i}`);
						const name = String(cfg.name ?? cfg.templateName ?? "Unnamed");

						return (
							<div
								key={`${entry.panel_id}-${uuid}`}
								className="rounded-xl border border-border bg-card p-5"
							>
								<div className="flex items-center justify-between mb-3">
									<div>
										<h3 className="text-[14px] font-semibold text-foreground">{name}</h3>
										<p className="font-mono text-[11px] text-muted-foreground">{uuid}</p>
									</div>
									<span className="rounded bg-secondary px-2 py-0.5 font-mono text-[10px] text-muted-foreground">
										{entry.panel_id}
									</span>
								</div>

								<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
									{Object.entries(cfg).map(([key, value]) => {
										if (key === "uuid" || key === "name") return null;
										const displayValue =
											value === null || value === undefined
												? "—"
												: typeof value === "object"
													? JSON.stringify(value).slice(0, 80)
													: String(value);
										return (
											<div key={key} className="rounded-lg bg-secondary/50 px-3 py-2">
												<p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
													{key.replace(/_/g, " ").replace(/([a-z])([A-Z])/g, "$1 $2")}
												</p>
												<p className="font-mono text-[12px] text-foreground truncate" title={String(value)}>
													{displayValue}
												</p>
											</div>
										);
									})}
								</div>
							</div>
						);
					})}
				</div>
			)}
		</div>
	);
}
