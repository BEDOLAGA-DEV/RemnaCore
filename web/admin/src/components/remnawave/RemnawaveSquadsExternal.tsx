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
		createdAt?: string;
		updatedAt?: string;
	};
};

type FormMode = "closed" | "create" | "edit";

type SquadFormData = {
	panelID: string;
	name: string;
	description: string;
};

// ─── Constants ──────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "squads", "external"] as const;

const EMPTY_FORM: SquadFormData = { panelID: "", name: "", description: "" };

// ─── Main Component ────────────────────────────────────────────────────────

export default function RemnawaveSquadsExternal() {
	const queryClient = useQueryClient();

	const {
		data: squads,
		isLoading,
		isError,
	} = useQuery({
		queryKey: QUERY_KEY,
		queryFn: () => apiGet<SquadEntry[]>("/api/remnawave/squads/external"),
	});

	const [formMode, setFormMode] = useState<FormMode>("closed");
	const [editingItem, setEditingItem] = useState<SquadEntry | null>(null);
	const [formData, setFormData] = useState<SquadFormData>(EMPTY_FORM);
	const [deleteTarget, setDeleteTarget] = useState<SquadEntry | null>(null);

	// ─── Mutations ──────────────────────────────────────────────────────────

	const createMutation = useMutation({
		mutationFn: (data: SquadFormData) =>
			apiPost(`/api/remnawave/squads/external/${data.panelID}`, {
				name: data.name,
				description: data.description,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const updateMutation = useMutation({
		mutationFn: ({ panelID, uuid, data }: { panelID: string; uuid: string; data: SquadFormData }) =>
			apiPut(`/api/remnawave/squads/external/${panelID}/${uuid}`, {
				uuid,
				name: data.name,
				description: data.description,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const deleteMutation = useMutation({
		mutationFn: ({ panelID, uuid }: { panelID: string; uuid: string }) =>
			apiDelete(`/api/remnawave/squads/external/${panelID}/${uuid}`),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			setDeleteTarget(null);
		},
	});

	// ─── Handlers ───────────────────────────────────────────────────────────

	function closeForm() {
		setFormMode("closed");
		setEditingItem(null);
		setFormData(EMPTY_FORM);
	}

	function openCreate() {
		setFormMode("create");
		setEditingItem(null);
		setFormData(EMPTY_FORM);
	}

	function openEdit(entry: SquadEntry) {
		setFormMode("edit");
		setEditingItem(entry);
		setFormData({
			panelID: entry.panel_id,
			name: entry.squad.name,
			description: "",
		});
	}

	function handleSubmit() {
		if (formMode === "create") {
			createMutation.mutate(formData);
		} else if (formMode === "edit" && editingItem) {
			updateMutation.mutate({
				panelID: editingItem.panel_id,
				uuid: editingItem.squad.uuid,
				data: formData,
			});
		}
	}

	function updateField(field: keyof SquadFormData, value: string) {
		setFormData((prev) => ({ ...prev, [field]: value }));
	}

	const isSaving = createMutation.isPending || updateMutation.isPending;

	// ─── Loading / Error ────────────────────────────────────────────────────

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

	// ─── Render ─────────────────────────────────────────────────────────────

	return (
		<div className="space-y-6">
			{/* Header */}
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
				<button
					type="button"
					onClick={openCreate}
					className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-[13px] font-medium text-background transition-colors hover:bg-primary/90"
				>
					<Plus size={14} />
					Create Squad
				</button>
			</div>

			{/* Form */}
			{formMode !== "closed" && (
				<div className="rounded-xl border border-border bg-card p-6">
					<h2 className="mb-4 text-[14px] font-semibold text-foreground">
						{formMode === "create" ? "Create Squad" : "Edit Squad"}
					</h2>
					<div className="grid gap-4 sm:grid-cols-2">
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Panel ID
							</label>
							<input
								type="text"
								value={formData.panelID}
								onChange={(e) => updateField("panelID", e.target.value)}
								disabled={formMode === "edit"}
								placeholder="e.g. panel-01"
								className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50"
							/>
						</div>
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Name
							</label>
							<input
								type="text"
								value={formData.name}
								onChange={(e) => updateField("name", e.target.value)}
								placeholder="Squad name"
								className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
							/>
						</div>
						<div className="sm:col-span-2">
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Description
							</label>
							<input
								type="text"
								value={formData.description}
								onChange={(e) => updateField("description", e.target.value)}
								placeholder="Optional description"
								className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
							/>
						</div>
					</div>
					<div className="flex gap-3 pt-4">
						<button
							type="button"
							onClick={handleSubmit}
							disabled={isSaving || !formData.panelID || !formData.name}
							className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background transition-colors hover:bg-primary/90 disabled:opacity-50"
						>
							{isSaving ? "Saving..." : "Save"}
						</button>
						<button
							type="button"
							onClick={closeForm}
							className="rounded-lg border border-border px-4 py-2 text-[13px] text-muted-foreground transition-colors hover:bg-secondary"
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			{/* Delete Confirmation */}
			{deleteTarget && (
				<div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6">
					<p className="text-[13px] text-foreground">
						Delete squad <span className="font-semibold">{deleteTarget.squad.name}</span> from panel{" "}
						<span className="font-mono text-[12px]">{deleteTarget.panel_id}</span>?
					</p>
					<div className="flex gap-3 pt-4">
						<button
							type="button"
							onClick={() =>
								deleteMutation.mutate({
									panelID: deleteTarget.panel_id,
									uuid: deleteTarget.squad.uuid,
								})
							}
							disabled={deleteMutation.isPending}
							className="rounded-lg bg-destructive px-4 py-2 text-[13px] font-medium text-white transition-colors hover:bg-destructive/90 disabled:opacity-50"
						>
							{deleteMutation.isPending ? "Deleting..." : "Delete"}
						</button>
						<button
							type="button"
							onClick={() => setDeleteTarget(null)}
							className="rounded-lg border border-border px-4 py-2 text-[13px] text-muted-foreground transition-colors hover:bg-secondary"
						>
							Cancel
						</button>
					</div>
				</div>
			)}

			{/* Table */}
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
								<th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Actions</th>
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
											{info?.membersCount != null ? String(info.membersCount) : "\u2014"}
										</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{s.uuid}</td>
										<td className="px-4 py-3">
											<div className="flex items-center justify-end gap-1">
												<button
													type="button"
													title="Edit"
													onClick={() => openEdit(entry)}
													className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
												>
													<Pencil size={14} />
												</button>
												<button
													type="button"
													title="Delete"
													onClick={() => setDeleteTarget(entry)}
													className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
												>
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
