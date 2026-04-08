import { zodResolver } from "@hookform/resolvers/zod";
import {
	LoadingSpinner,
	cn,
	formatDateTime,
	usePluginCollection,
	usePluginPages,
} from "@remnacore/shared";
import type { PluginDocument } from "@remnacore/shared";
import { useParams } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { Loader2, Pencil, Plus, Trash2, X } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { DataTable } from "../../components/DataTable.js";

// ─── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Derive visible columns from the first document's data keys.
 * Falls back to empty array if no documents exist.
 */
function deriveDataKeys(documents: PluginDocument[]): string[] {
	const first = documents[0];
	if (!first) return [];
	return Object.keys(first.data);
}

/**
 * Infer a field's input type from its value.
 */
function inferFieldType(value: unknown): "boolean" | "number" | "text" {
	if (typeof value === "boolean") return "boolean";
	if (typeof value === "number") return "number";
	return "text";
}

/**
 * Format a cell value for display.
 */
function formatCellValue(value: unknown, t: (key: string) => string): string {
	if (value === null || value === undefined) return "—";
	if (typeof value === "boolean")
		return value ? t("admin.pluginPage.true") : t("admin.pluginPage.false");
	if (typeof value === "number") return String(value);
	if (typeof value === "object") return JSON.stringify(value);
	return String(value);
}

/**
 * Prettify a data key into a human-readable label.
 * e.g. "tariff_name" -> "Tariff Name", "userUUID" -> "User UUID"
 */
function prettifyKey(key: string): string {
	return key
		.replace(/_/g, " ")
		.replace(/([a-z])([A-Z])/g, "$1 $2")
		.replace(/\b\w/g, (c) => c.toUpperCase());
}

// ─── Dynamic form schema builder ──────────────────────────────────────────────

function buildFormSchema(
	dataKeys: string[],
	sampleDoc: PluginDocument | undefined,
) {
	const shape: Record<string, z.ZodTypeAny> = {};

	for (const key of dataKeys) {
		const sampleValue = sampleDoc?.data[key];
		const fieldType = inferFieldType(sampleValue);

		if (fieldType === "boolean") {
			shape[key] = z.boolean().default(false);
		} else if (fieldType === "number") {
			shape[key] = z.coerce.number().default(0);
		} else {
			shape[key] = z.string().default("");
		}
	}

	return z.object(shape);
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function PluginPageView() {
	const { slug, pagePath } = useParams({ strict: false }) as {
		slug: string;
		pagePath: string;
	};
	const { t } = useTranslation();
	const { data: pluginPages, isLoading: pagesLoading } = usePluginPages();
	const { list, create, update, remove } = usePluginCollection(slug, pagePath);

	const [formMode, setFormMode] = useState<"closed" | "create" | "edit">(
		"closed",
	);
	const [editingDocId, setEditingDocId] = useState<string | null>(null);

	const pageInfo = pluginPages?.find(
		(p) => p.plugin_slug === slug && p.path === pagePath,
	);

	const documents = list.data ?? [];
	const dataKeys = deriveDataKeys(documents);

	// ─── Table columns ───────────────────────────────────────────────────────

	const columns: ColumnDef<PluginDocument, unknown>[] = [
		...dataKeys.map(
			(key): ColumnDef<PluginDocument, unknown> => ({
				id: `data_${key}`,
				header: prettifyKey(key),
				cell: ({ row }) => {
					const value = row.original.data[key];
					return (
						<span className="text-[12px]">{formatCellValue(value, t)}</span>
					);
				},
			}),
		),
		{
			id: "created_at",
			header: t("admin.pluginPage.fieldCreatedAt"),
			cell: ({ row }) => (
				<span className="font-mono text-[11px] text-muted-foreground">
					{formatDateTime(row.original.created_at)}
				</span>
			),
		},
		{
			id: "actions",
			header: "",
			cell: ({ row }) => (
				<div className="flex items-center justify-end gap-1">
					<button
						type="button"
						onClick={() => handleEdit(row.original)}
						className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
					>
						<Pencil size={14} />
					</button>
					<button
						type="button"
						onClick={() => handleDelete(row.original.id)}
						className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
					>
						<Trash2 size={14} />
					</button>
				</div>
			),
		},
	];

	// ─── Actions ──────────────────────────────────────────────────────────────

	function handleCreate() {
		setEditingDocId(null);
		setFormMode("create");
	}

	function handleEdit(doc: PluginDocument) {
		setEditingDocId(doc.id);
		setFormMode("edit");
	}

	function handleDelete(docId: string) {
		if (window.confirm(t("admin.pluginPage.deleteConfirm"))) {
			remove.mutate(docId);
		}
	}

	function handleFormClose() {
		setFormMode("closed");
		setEditingDocId(null);
	}

	function handleFormSubmit(data: Record<string, unknown>) {
		if (formMode === "create") {
			create.mutate(data, { onSuccess: handleFormClose });
		} else if (formMode === "edit" && editingDocId) {
			update.mutate({ id: editingDocId, data }, { onSuccess: handleFormClose });
		}
	}

	// ─── Loading / Error states ───────────────────────────────────────────────

	if (pagesLoading || list.isLoading) {
		return <LoadingSpinner />;
	}

	if (list.isError) {
		return (
			<div className="py-12 text-center">
				<p className="text-destructive">{t("admin.pluginPage.errorLoading")}</p>
			</div>
		);
	}

	const editingDoc = editingDocId
		? documents.find((d) => d.id === editingDocId)
		: undefined;

	// ─── Render ───────────────────────────────────────────────────────────────

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-[15px] font-semibold text-foreground">
						{pageInfo?.title ?? t("admin.pluginPage.unknownPage")}
					</h1>
					{pageInfo && (
						<p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
							{pageInfo.plugin_name} / {pageInfo.path}
						</p>
					)}
				</div>
				<button
					type="button"
					onClick={handleCreate}
					className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
				>
					<Plus size={14} />
					{t("admin.pluginPage.createItem")}
				</button>
			</div>

			{/* Form (create/edit) */}
			{formMode !== "closed" && (
				<PluginDocumentForm
					mode={formMode}
					dataKeys={dataKeys}
					sampleDoc={documents[0]}
					editingDoc={editingDoc}
					isPending={create.isPending || update.isPending}
					onSubmit={handleFormSubmit}
					onClose={handleFormClose}
				/>
			)}

			{/* Table */}
			{documents.length > 0 ? (
				<DataTable data={documents} columns={columns} isLoading={false} />
			) : (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<p className="text-[13px] text-muted-foreground">
						{t("admin.pluginPage.noDocuments")}
					</p>
				</div>
			)}
		</div>
	);
}

// ─── Document Form ────────────────────────────────────────────────────────────

type PluginDocumentFormProps = {
	mode: "create" | "edit";
	dataKeys: string[];
	sampleDoc: PluginDocument | undefined;
	editingDoc: PluginDocument | undefined;
	isPending: boolean;
	onSubmit: (data: Record<string, unknown>) => void;
	onClose: () => void;
};

function PluginDocumentForm({
	mode,
	dataKeys,
	sampleDoc,
	editingDoc,
	isPending,
	onSubmit,
	onClose,
}: PluginDocumentFormProps) {
	const { t } = useTranslation();

	const schema = buildFormSchema(dataKeys, sampleDoc);
	type FormValues = z.infer<typeof schema>;

	const defaultValues: Record<string, unknown> = {};
	for (const key of dataKeys) {
		if (mode === "edit" && editingDoc) {
			defaultValues[key] = editingDoc.data[key] ?? "";
		} else {
			const sampleValue = sampleDoc?.data[key];
			const fieldType = inferFieldType(sampleValue);
			if (fieldType === "boolean") defaultValues[key] = false;
			else if (fieldType === "number") defaultValues[key] = 0;
			else defaultValues[key] = "";
		}
	}

	const {
		register,
		handleSubmit,
		setValue,
		watch,
		formState: { errors },
	} = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: defaultValues as FormValues,
	});

	const handleFormSubmit = (values: FormValues) => {
		onSubmit(values as Record<string, unknown>);
	};

	return (
		<div className="rounded-xl border border-border bg-card p-6">
			<div className="mb-4 flex items-center justify-between">
				<h2 className="text-[15px] font-semibold text-foreground">
					{mode === "create"
						? t("admin.pluginPage.createTitle")
						: t("admin.pluginPage.editTitle")}
				</h2>
				<button
					type="button"
					onClick={onClose}
					className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					<X size={16} />
				</button>
			</div>

			<form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
				<div className="grid gap-4 sm:grid-cols-2">
					{dataKeys.map((key) => {
						const sampleValue = sampleDoc?.data[key];
						const fieldType = inferFieldType(sampleValue);

						if (fieldType === "boolean") {
							const currentValue = watch(key as keyof FormValues);
							return (
								<div key={key} className="flex items-center gap-3">
									<button
										type="button"
										role="switch"
										aria-label={prettifyKey(key)}
										aria-checked={!!currentValue}
										onClick={() =>
											setValue(
												key as keyof FormValues,
												!currentValue as FormValues[keyof FormValues],
											)
										}
										className={cn(
											"relative h-5 w-9 rounded-full transition-colors",
											currentValue ? "bg-primary" : "bg-muted",
										)}
									>
										<span
											className={cn(
												"absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-background transition-transform",
												currentValue && "translate-x-4",
											)}
										/>
									</button>
									<span className="text-[13px] text-foreground">
										{prettifyKey(key)}
									</span>
								</div>
							);
						}

						return (
							<div key={key}>
								<label
									htmlFor={`field_${key}`}
									className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground"
								>
									{prettifyKey(key)}
								</label>
								<input
									id={`field_${key}`}
									type={fieldType === "number" ? "number" : "text"}
									step={fieldType === "number" ? "any" : undefined}
									{...register(key as keyof FormValues, {
										valueAsNumber: fieldType === "number",
									})}
									className="w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
								/>
								{errors[key as keyof FormValues] && (
									<p className="mt-1 text-[11px] text-destructive">
										{String(errors[key as keyof FormValues]?.message)}
									</p>
								)}
							</div>
						);
					})}
				</div>

				{dataKeys.length === 0 && (
					<p className="text-[12px] text-muted-foreground">
						{t("admin.pluginPage.noDocuments")}
					</p>
				)}

				<div className="flex items-center gap-3 pt-2">
					<button
						type="submit"
						disabled={isPending}
						className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background transition-colors hover:bg-primary/90 disabled:opacity-40"
					>
						{isPending ? (
							<>
								<Loader2 size={14} className="animate-spin" />
								{t("admin.pluginPage.saving")}
							</>
						) : (
							t("common.save")
						)}
					</button>
					<button
						type="button"
						onClick={onClose}
						className="rounded-lg border border-border bg-card px-4 py-2 text-[13px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
					>
						{t("common.cancel")}
					</button>
				</div>
			</form>
		</div>
	);
}
