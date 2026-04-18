import { apiDelete, apiGet, apiPost, apiPut, LoadingSpinner } from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Boxes, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

type SquadEntry = {
	panel_id: string;
	squad: {
		uuid: string;
		name: string;
		info?: { membersCount?: number; inboundsCount?: number } | null;
	};
};

type PanelEntry = {
	id: string;
	data: { slug: string; url: string; status: string };
	created_at: string;
};

type InboundEntry = {
	uuid: string;
	tag: string;
	type: string;
};

type FormMode = "closed" | "create" | "edit";

// ─── Constants ──────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "squads", "internal"] as const;

// ─── Main Component ────────────────────────────────────────────────────────

export default function RemnawaveSquadsInternal() {
	const queryClient = useQueryClient();

	const { data: squads, isLoading, isError } = useQuery({
		queryKey: QUERY_KEY,
		queryFn: () => apiGet<SquadEntry[]>("/api/remnawave/squads/internal"),
	});

	// Fetch panels for dropdown
	const { data: panelsRaw } = useQuery({
		queryKey: ["plugins", "remnawave-provider", "collections", "panel_connections"],
		queryFn: () => apiGet<PanelEntry[]>("/api/plugins/remnawave-provider/collections/panel_connections"),
	});
	const panels = Array.isArray(panelsRaw) ? panelsRaw : [];

	const [formMode, setFormMode] = useState<FormMode>("closed");
	const [editingItem, setEditingItem] = useState<SquadEntry | null>(null);
	const [panelID, setPanelID] = useState("");
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [selectedInbounds, setSelectedInbounds] = useState<string[]>([]);
	const [deleteTarget, setDeleteTarget] = useState<SquadEntry | null>(null);

	// Fetch inbounds for selected panel
	const { data: inboundsRaw } = useQuery({
		queryKey: ["remnawave", "inbounds", panelID],
		queryFn: () => apiGet<{ panel_id: string; inbounds: InboundEntry[] }>(
			`/api/remnawave/config-profiles/${panelID}/inbounds`,
		),
		enabled: !!panelID && formMode === "create",
	});
	const inbounds: InboundEntry[] = Array.isArray(inboundsRaw)
		? inboundsRaw
		: (inboundsRaw as any)?.inbounds ?? [];

	// Auto-select all inbounds when they load
	useEffect(() => {
		if (formMode === "create" && inbounds.length > 0 && selectedInbounds.length === 0) {
			setSelectedInbounds(inbounds.map((ib) => ib.uuid));
		}
	}, [inbounds, formMode, selectedInbounds.length]);

	// ─── Mutations ──────────────────────────────────────────────────────────

	const createMutation = useMutation({
		mutationFn: () =>
			apiPost(`/api/remnawave/squads/${panelID}`, {
				name,
				description,
				inbounds: selectedInbounds,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const updateMutation = useMutation({
		mutationFn: ({ pid, uuid }: { pid: string; uuid: string }) =>
			apiPut(`/api/remnawave/squads/${pid}/${uuid}`, { uuid, name, description }),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const deleteMutation = useMutation({
		mutationFn: ({ pid, uuid }: { pid: string; uuid: string }) =>
			apiDelete(`/api/remnawave/squads/${pid}/${uuid}`),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			setDeleteTarget(null);
		},
	});

	// ─── Handlers ───────────────────────────────────────────────────────────

	function closeForm() {
		setFormMode("closed");
		setEditingItem(null);
		setPanelID("");
		setName("");
		setDescription("");
		setSelectedInbounds([]);
	}

	function openCreate() {
		closeForm();
		setFormMode("create");
	}

	function openEdit(entry: SquadEntry) {
		setFormMode("edit");
		setEditingItem(entry);
		setPanelID(entry.panel_id);
		setName(entry.squad.name);
		setDescription("");
		setSelectedInbounds([]);
	}

	function handleSubmit() {
		if (formMode === "create") {
			createMutation.mutate();
		} else if (formMode === "edit" && editingItem) {
			updateMutation.mutate({ pid: editingItem.panel_id, uuid: editingItem.squad.uuid });
		}
	}

	function toggleInbound(uuid: string) {
		setSelectedInbounds((prev) =>
			prev.includes(uuid) ? prev.filter((id) => id !== uuid) : [...prev, uuid],
		);
	}

	const isSaving = createMutation.isPending || updateMutation.isPending;

	if (isLoading) return <LoadingSpinner />;

	if (isError) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-16">
				<AlertTriangle size={32} className="text-destructive" />
				<p className="text-[13px] text-destructive">Failed to load internal squads</p>
			</div>
		);
	}

	const list: SquadEntry[] = Array.isArray(squads) ? squads : [];

	const inputCls =
		"w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50";

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-3">
					<Boxes size={18} className="text-muted-foreground" />
					<div>
						<h1 className="text-[15px] font-semibold text-foreground">Internal Squads</h1>
						<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
							{list.length} {list.length === 1 ? "squad" : "squads"}
						</p>
					</div>
				</div>
				<button
					type="button"
					onClick={openCreate}
					className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-[13px] font-medium text-background hover:bg-primary/90"
				>
					<Plus size={14} />
					Create Squad
				</button>
			</div>

			{/* Form */}
			{formMode !== "closed" && (
				<div className="rounded-xl border border-border bg-card p-6">
					<h2 className="mb-4 text-[14px] font-semibold text-foreground">
						{formMode === "create" ? "Create Internal Squad" : "Edit Squad"}
					</h2>
					<div className="grid gap-4 sm:grid-cols-2">
						{/* Panel selector */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Panel
							</label>
							<select
								value={panelID}
								onChange={(e) => {
									setPanelID(e.target.value);
									setSelectedInbounds([]);
								}}
								disabled={formMode === "edit"}
								className={inputCls}
							>
								<option value="">Select panel...</option>
								{panels.map((p) => (
									<option key={p.id} value={p.id}>
										{p.data.slug || p.id} — {p.data.url}
									</option>
								))}
							</select>
						</div>

						{/* Name */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Name
							</label>
							<input
								type="text"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="Squad name"
								className={inputCls}
							/>
						</div>

						{/* Description */}
						<div className="sm:col-span-2">
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Description
							</label>
							<input
								type="text"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Optional description"
								className={inputCls}
							/>
						</div>

						{/* Inbounds selector (create only) */}
						{formMode === "create" && panelID && (
							<div className="sm:col-span-2">
								<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Inbounds (required)
								</label>
								{inbounds.length === 0 ? (
									<p className="text-[12px] text-muted-foreground">
										{panelID ? "Loading inbounds..." : "Select a panel first"}
									</p>
								) : (
									<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
										{inbounds.map((ib) => {
											const checked = selectedInbounds.includes(ib.uuid);
											return (
												<label
													key={ib.uuid}
													className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-[12px] transition-colors ${
														checked
															? "border-primary/30 bg-primary/5 text-foreground"
															: "border-border bg-background text-muted-foreground hover:border-border/80"
													}`}
												>
													<input
														type="checkbox"
														checked={checked}
														onChange={() => toggleInbound(ib.uuid)}
														className="h-3.5 w-3.5 accent-primary"
													/>
													<span className="font-mono">{ib.tag}</span>
													<span className="text-[10px] text-muted-foreground">({ib.type})</span>
												</label>
											);
										})}
									</div>
								)}
							</div>
						)}
					</div>

					<div className="flex gap-3 pt-4">
						<button
							type="button"
							onClick={handleSubmit}
							disabled={isSaving || !panelID || !name || (formMode === "create" && selectedInbounds.length === 0)}
							className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background hover:bg-primary/90 disabled:opacity-50"
						>
							{isSaving ? "Saving..." : "Save"}
						</button>
						<button type="button" onClick={closeForm} className="rounded-lg border border-border px-4 py-2 text-[13px] text-muted-foreground hover:bg-secondary">
							Cancel
						</button>
					</div>
				</div>
			)}

			{/* Delete confirmation */}
			{deleteTarget && (
				<div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4">
					<p className="text-[13px] text-foreground">
						Delete squad <strong>{deleteTarget.squad.name}</strong>?
					</p>
					<div className="mt-3 flex gap-2">
						<button
							type="button"
							onClick={() => deleteMutation.mutate({ pid: deleteTarget.panel_id, uuid: deleteTarget.squad.uuid })}
							disabled={deleteMutation.isPending}
							className="rounded-lg bg-destructive px-3 py-1.5 text-[12px] font-medium text-background"
						>
							{deleteMutation.isPending ? "Deleting..." : "Delete"}
						</button>
						<button type="button" onClick={() => setDeleteTarget(null)} className="rounded-lg border border-border px-3 py-1.5 text-[12px] text-muted-foreground">
							Cancel
						</button>
					</div>
				</div>
			)}

			{/* Table */}
			{list.length === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<Boxes size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No internal squads found</p>
				</div>
			) : (
				<div className="rounded-xl border border-border bg-card">
					<table className="w-full">
						<thead>
							<tr className="border-b border-border">
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Name</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Panel</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Members</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Inbounds</th>
								<th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Actions</th>
							</tr>
						</thead>
						<tbody>
							{list.map((entry) => {
								const s = entry.squad;
								const info = s.info as Record<string, unknown> | null;
								return (
									<tr key={`${entry.panel_id}-${s.uuid}`} className="border-b border-border/50 last:border-0 hover:bg-secondary/30">
										<td className="px-4 py-3 text-[13px] font-semibold text-foreground">{s.name}</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{entry.panel_id}</td>
										<td className="px-4 py-3 text-[13px]">{info?.membersCount != null ? String(info.membersCount) : "—"}</td>
										<td className="px-4 py-3 text-[13px]">{info?.inboundsCount != null ? String(info.inboundsCount) : "—"}</td>
										<td className="px-4 py-3">
											<div className="flex items-center justify-end gap-1">
												<button type="button" onClick={() => openEdit(entry)} className="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground">
													<Pencil size={14} />
												</button>
												<button type="button" onClick={() => setDeleteTarget(entry)} className="rounded-lg p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive">
													<Trash2 size={14} />
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
