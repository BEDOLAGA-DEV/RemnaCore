import { apiDelete, apiGet, apiPatch, apiPost, cn, LoadingSpinner } from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Pencil, Plus, Server, Trash2 } from "lucide-react";
import { useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

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

type FormMode = "closed" | "create" | "edit";

type HostFormData = {
	panelID: string;
	remark: string;
	address: string;
	port: number;
	sni: string;
	host: string;
	path: string;
	securityLayer: string;
	alpn: string;
	fingerprint: string;
	isDisabled: boolean;
};

// ─── Constants ──────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "hosts"] as const;

const REMARK_MAX_LENGTH = 40;

const SECURITY_LAYER_OPTIONS = ["DEFAULT", "TLS", "NONE"] as const;

const ALPN_OPTIONS = ["", "h2", "h3", "http/1.1", "h2,http/1.1", "h3,h2,http/1.1", "h3,h2"] as const;

const FINGERPRINT_OPTIONS = [
	"",
	"chrome",
	"firefox",
	"safari",
	"ios",
	"android",
	"edge",
	"random",
	"randomized",
] as const;

const EMPTY_FORM: HostFormData = {
	panelID: "",
	remark: "",
	address: "",
	port: 443,
	sni: "",
	host: "",
	path: "",
	securityLayer: "DEFAULT",
	alpn: "",
	fingerprint: "",
	isDisabled: false,
};

// ─── Main Component ────────────────────────────────────────────────────────

export default function RemnawaveHosts() {
	const queryClient = useQueryClient();

	const {
		data: hosts,
		isLoading,
		isError,
	} = useQuery({
		queryKey: QUERY_KEY,
		queryFn: () => apiGet<HostEntry[]>("/api/remnawave/hosts"),
	});

	const [formMode, setFormMode] = useState<FormMode>("closed");
	const [editingItem, setEditingItem] = useState<HostEntry | null>(null);
	const [formData, setFormData] = useState<HostFormData>(EMPTY_FORM);
	const [deleteTarget, setDeleteTarget] = useState<HostEntry | null>(null);

	// ─── Mutations ──────────────────────────────────────────────────────────

	const createMutation = useMutation({
		mutationFn: (data: HostFormData) =>
			apiPost(`/api/remnawave/hosts/${data.panelID}`, {
				remark: data.remark,
				address: data.address,
				port: data.port,
				sni: data.sni,
				host: data.host,
				path: data.path,
				securityLayer: data.securityLayer,
				alpn: data.alpn,
				fingerprint: data.fingerprint,
				isDisabled: data.isDisabled,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const updateMutation = useMutation({
		mutationFn: ({ panelID, data }: { panelID: string; data: HostFormData & { uuid: string } }) =>
			apiPatch(`/api/remnawave/hosts/${panelID}`, {
				uuid: data.uuid,
				remark: data.remark,
				address: data.address,
				port: data.port,
				sni: data.sni,
				host: data.host,
				path: data.path,
				securityLayer: data.securityLayer,
				alpn: data.alpn,
				fingerprint: data.fingerprint,
				isDisabled: data.isDisabled,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: QUERY_KEY });
			closeForm();
		},
	});

	const deleteMutation = useMutation({
		mutationFn: ({ panelID, uuid }: { panelID: string; uuid: string }) =>
			apiDelete(`/api/remnawave/hosts/${panelID}/${uuid}`),
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

	function openEdit(entry: HostEntry) {
		setFormMode("edit");
		setEditingItem(entry);
		setFormData({
			panelID: entry.panel_id,
			remark: entry.host.remark,
			address: entry.host.address,
			port: entry.host.port,
			sni: entry.host.sni,
			host: entry.host.host,
			path: entry.host.path,
			securityLayer: entry.host.securityLayer,
			alpn: entry.host.alpn,
			fingerprint: entry.host.fingerprint,
			isDisabled: entry.host.isDisabled,
		});
	}

	function handleSubmit() {
		if (formMode === "create") {
			createMutation.mutate(formData);
		} else if (formMode === "edit" && editingItem) {
			updateMutation.mutate({
				panelID: editingItem.panel_id,
				data: { ...formData, uuid: editingItem.host.uuid },
			});
		}
	}

	function updateField<K extends keyof HostFormData>(field: K, value: HostFormData[K]) {
		setFormData((prev) => ({ ...prev, [field]: value }));
	}

	const isSaving = createMutation.isPending || updateMutation.isPending;

	// ─── Loading / Error ────────────────────────────────────────────────────

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

	// ─── Shared input classes ───────────────────────────────────────────────

	const inputClasses =
		"w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50";

	const selectClasses =
		"w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary";

	// ─── Render ─────────────────────────────────────────────────────────────

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-3">
					<Server size={18} className="text-muted-foreground" />
					<div>
						<h1 className="text-[15px] font-semibold text-foreground">Hosts</h1>
						<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
							{list.length} {list.length === 1 ? "host" : "hosts"}
						</p>
					</div>
				</div>
				<button
					type="button"
					onClick={openCreate}
					className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-[13px] font-medium text-background transition-colors hover:bg-primary/90"
				>
					<Plus size={14} />
					Create Host
				</button>
			</div>

			{/* Form */}
			{formMode !== "closed" && (
				<div className="rounded-xl border border-border bg-card p-6">
					<h2 className="mb-4 text-[14px] font-semibold text-foreground">
						{formMode === "create" ? "Create Host" : "Edit Host"}
					</h2>
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						{/* Panel ID */}
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
								className={inputClasses}
							/>
						</div>

						{/* Remark */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Remark
							</label>
							<input
								type="text"
								value={formData.remark}
								onChange={(e) => updateField("remark", e.target.value.slice(0, REMARK_MAX_LENGTH))}
								placeholder="Host remark"
								maxLength={REMARK_MAX_LENGTH}
								className={inputClasses}
							/>
						</div>

						{/* Address */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Address
							</label>
							<input
								type="text"
								value={formData.address}
								onChange={(e) => updateField("address", e.target.value)}
								placeholder="e.g. example.com"
								className={inputClasses}
							/>
						</div>

						{/* Port */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Port
							</label>
							<input
								type="number"
								value={formData.port}
								onChange={(e) => updateField("port", Number(e.target.value))}
								placeholder="443"
								className={inputClasses}
							/>
						</div>

						{/* SNI */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								SNI
							</label>
							<input
								type="text"
								value={formData.sni}
								onChange={(e) => updateField("sni", e.target.value)}
								placeholder="Server Name Indication"
								className={inputClasses}
							/>
						</div>

						{/* Host */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Host
							</label>
							<input
								type="text"
								value={formData.host}
								onChange={(e) => updateField("host", e.target.value)}
								placeholder="Host header"
								className={inputClasses}
							/>
						</div>

						{/* Path */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Path
							</label>
							<input
								type="text"
								value={formData.path}
								onChange={(e) => updateField("path", e.target.value)}
								placeholder="/path"
								className={inputClasses}
							/>
						</div>

						{/* Security Layer */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Security Layer
							</label>
							<select
								value={formData.securityLayer}
								onChange={(e) => updateField("securityLayer", e.target.value)}
								className={selectClasses}
							>
								{SECURITY_LAYER_OPTIONS.map((opt) => (
									<option key={opt} value={opt}>
										{opt}
									</option>
								))}
							</select>
						</div>

						{/* ALPN */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								ALPN
							</label>
							<select
								value={formData.alpn}
								onChange={(e) => updateField("alpn", e.target.value)}
								className={selectClasses}
							>
								{ALPN_OPTIONS.map((opt) => (
									<option key={opt} value={opt}>
										{opt || "(none)"}
									</option>
								))}
							</select>
						</div>

						{/* Fingerprint */}
						<div>
							<label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
								Fingerprint
							</label>
							<select
								value={formData.fingerprint}
								onChange={(e) => updateField("fingerprint", e.target.value)}
								className={selectClasses}
							>
								{FINGERPRINT_OPTIONS.map((opt) => (
									<option key={opt} value={opt}>
										{opt || "(none)"}
									</option>
								))}
							</select>
						</div>

						{/* Disabled */}
						<div className="flex items-end pb-1">
							<label className="flex items-center gap-2 text-[13px] text-foreground">
								<input
									type="checkbox"
									checked={formData.isDisabled}
									onChange={(e) => updateField("isDisabled", e.target.checked)}
									className="h-4 w-4 rounded border-border"
								/>
								Disabled
							</label>
						</div>
					</div>
					<div className="flex gap-3 pt-4">
						<button
							type="button"
							onClick={handleSubmit}
							disabled={isSaving || !formData.panelID || !formData.remark || !formData.address}
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
						Delete host <span className="font-semibold">{deleteTarget.host.remark}</span> from panel{" "}
						<span className="font-mono text-[12px]">{deleteTarget.panel_id}</span>?
					</p>
					<div className="flex gap-3 pt-4">
						<button
							type="button"
							onClick={() =>
								deleteMutation.mutate({
									panelID: deleteTarget.panel_id,
									uuid: deleteTarget.host.uuid,
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
					<Server size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">No hosts found</p>
				</div>
			) : (
				<div className="overflow-x-auto rounded-xl border border-border bg-card">
					<table className="w-full">
						<thead>
							<tr className="border-b border-border">
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Remark</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Address:Port</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Security</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">SNI</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Status</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Panel</th>
								<th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">Actions</th>
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
										<td className="px-4 py-3 text-[13px] font-semibold text-foreground">{h.remark || "\u2014"}</td>
										<td className="px-4 py-3 font-mono text-[12px] text-foreground">
											{h.address || "\u2014"}:{h.port || "\u2014"}
										</td>
										<td className="px-4 py-3 text-[12px] text-foreground">{h.securityLayer || "none"}</td>
										<td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">{h.sni || "\u2014"}</td>
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
