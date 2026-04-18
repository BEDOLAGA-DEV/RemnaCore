import { apiDelete, apiGet, apiPost, apiPut, LoadingSpinner } from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Boxes, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

type SquadEntry = {
	panel_id: string;
	squad: {
		uuid: string;
		name: string;
		info?: { membersCount?: number } | null;
	};
};

type PanelEntry = {
	id: string;
	slug: string;
	url: string;
	status: string;
};

type FormMode = "closed" | "create" | "edit";

// ─── Constants ──────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "squads", "external"] as const;

// ─── Component ──────────────────────────────────────────────────────────────

export default function RemnawaveSquadsExternal() {
	const queryClient = useQueryClient();

	const { data: squads, isLoading, isError } = useQuery({
		queryKey: QUERY_KEY,
		queryFn: () => apiGet<SquadEntry[]>("/api/remnawave/squads/external"),
	});

	const { data: panelsRaw } = useQuery({
		queryKey: ["remnawave", "panels"],
		queryFn: () => apiGet<PanelEntry[]>("/api/remnawave/panels"),
	});
	const panels = Array.isArray(panelsRaw) ? panelsRaw : [];

	const [formMode, setFormMode] = useState<FormMode>("closed");
	const [editingItem, setEditingItem] = useState<SquadEntry | null>(null);
	const [panelID, setPanelID] = useState("");
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [deleteTarget, setDeleteTarget] = useState<SquadEntry | null>(null);

	const createMutation = useMutation({
		mutationFn: () =>
			apiPost(`/api/remnawave/squads/external/${panelID}`, { name, description }),
		onSuccess: () => { queryClient.invalidateQueries({ queryKey: QUERY_KEY }); closeForm(); },
	});

	const updateMutation = useMutation({
		mutationFn: ({ pid, uuid }: { pid: string; uuid: string }) =>
			apiPut(`/api/remnawave/squads/external/${pid}/${uuid}`, { uuid, name, description }),
		onSuccess: () => { queryClient.invalidateQueries({ queryKey: QUERY_KEY }); closeForm(); },
	});

	const deleteMutation = useMutation({
		mutationFn: ({ pid, uuid }: { pid: string; uuid: string }) =>
			apiDelete(`/api/remnawave/squads/external/${pid}/${uuid}`),
		onSuccess: () => { queryClient.invalidateQueries({ queryKey: QUERY_KEY }); setDeleteTarget(null); },
	});

	function closeForm() {
		setFormMode("closed"); setEditingItem(null); setPanelID(""); setName(""); setDescription("");
	}
	function openCreate() { closeForm(); setFormMode("create"); }
	function openEdit(entry: SquadEntry) {
		setFormMode("edit"); setEditingItem(entry);
		setPanelID(entry.panel_id); setName(entry.squad.name); setDescription("");
	}
	function handleSubmit() {
		if (formMode === "create") createMutation.mutate();
		else if (formMode === "edit" && editingItem)
			updateMutation.mutate({ pid: editingItem.panel_id, uuid: editingItem.squad.uuid });
	}

	const isSaving = createMutation.isPending || updateMutation.isPending;

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
	const inputCls = "w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50";

	return (
		<div className="space-y-6">
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-3">
					<Boxes size={18} className="text-muted-foreground" />
					<div>
						<h1 className="text-[15px] font-semibold text-foreground">External Squads</h1>
						<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
							{list.length} {list.length === 1 ? "squad" : "squads"}
						</p>
					</div>
				</div>
				<button type="button" onClick={openCreate} className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-[13px] font-medium text-background hover:bg-primary/90">
					<Plus size={14} /> Create Squad
				</button>
			</div>

			{formMode !== "closed" && (
				<div className="rounded-xl border border-border bg-card p-6">
					<h2 className="mb-4 text-[14px] font-semibold text-foreground">
						{formMode === "create" ? "Create External Squad" : "Edit Squad"}
					</h2>
					<div className="grid gap-4 sm:grid-cols-2">
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Panel</label>
							<select value={panelID} onChange={(e) => setPanelID(e.target.value)} disabled={formMode === "edit"} className={inputCls}>
								<option value="">Select panel...</option>
								{panels.map((p) => (
									<option key={p.id} value={p.id}>{p.slug || p.id} — {p.url}</option>
								))}
							</select>
						</div>
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Name</label>
							<input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="Squad name" className={inputCls} />
						</div>
						<div className="sm:col-span-2">
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Description</label>
							<input type="text" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" className={inputCls} />
						</div>
					</div>
					<div className="flex gap-3 pt-4">
						<button type="button" onClick={handleSubmit} disabled={isSaving || !panelID || !name} className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background hover:bg-primary/90 disabled:opacity-50">
							{isSaving ? "Saving..." : "Save"}
						</button>
						<button type="button" onClick={closeForm} className="rounded-lg border border-border px-4 py-2 text-[13px] text-muted-foreground hover:bg-secondary">Cancel</button>
					</div>
				</div>
			)}

			{deleteTarget && (
				<div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4">
					<p className="text-[13px] text-foreground">Delete squad <strong>{deleteTarget.squad.name}</strong>?</p>
					<div className="mt-3 flex gap-2">
						<button type="button" onClick={() => deleteMutation.mutate({ pid: deleteTarget.panel_id, uuid: deleteTarget.squad.uuid })} disabled={deleteMutation.isPending} className="rounded-lg bg-destructive px-3 py-1.5 text-[12px] font-medium text-background">
							{deleteMutation.isPending ? "Deleting..." : "Delete"}
						</button>
						<button type="button" onClick={() => setDeleteTarget(null)} className="rounded-lg border border-border px-3 py-1.5 text-[12px] text-muted-foreground">Cancel</button>
					</div>
				</div>
			)}

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
										<td className="px-4 py-3">
											<div className="flex items-center justify-end gap-1">
												<button type="button" onClick={() => openEdit(entry)} className="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"><Pencil size={14} /></button>
												<button type="button" onClick={() => setDeleteTarget(entry)} className="rounded-lg p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"><Trash2 size={14} /></button>
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
