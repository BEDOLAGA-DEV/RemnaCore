import { zodResolver } from "@hookform/resolvers/zod";
import {
	LoadingSpinner,
	apiGet,
	apiPost,
	cn,
	formatDateTime,
	usePluginCollection,
	usePluginPages,
} from "@remnacore/shared";
import type { PluginDocument, PluginPageField } from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { Check, Loader2, Pencil, Plug, Plus, Trash2, X, XCircle } from "lucide-react";
import { useCallback, useState } from "react";
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
	if (value === null || value === undefined) return "\u2014";
	if (typeof value === "boolean")
		return value ? t("admin.pluginPage.true") : t("admin.pluginPage.false");
	if (typeof value === "number") return String(value);
	if (Array.isArray(value)) return value.join(", ");
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

// ─── Dynamic form schema builder (auto-detect mode) ──────────────────────────

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

// ─── Schema-driven form helpers ──────────────────────────────────────────────

/**
 * Build a Zod schema from the plugin's declared field definitions.
 */
function buildZodSchemaFromFields(fields: PluginPageField[]) {
	const shape: Record<string, z.ZodTypeAny> = {};

	for (const field of fields) {
		switch (field.type) {
			case "number":
				shape[field.key] = field.required
					? z.coerce.number()
					: z.coerce.number().optional();
				break;
			case "boolean":
				shape[field.key] = z.boolean();
				break;
			case "multiselect":
				shape[field.key] = z.array(z.string());
				break;
			case "textarea":
				shape[field.key] = field.required
					? z.string().min(1)
					: z.string();
				break;
			case "select":
				shape[field.key] = field.required
					? z.string().min(1)
					: z.string();
				break;
			default:
				shape[field.key] = field.required
					? z.string().min(1)
					: z.string();
		}
	}

	return z.object(shape);
}

/**
 * Build default form values from field definitions.
 */
function buildDefaultValues(
	fields: PluginPageField[],
): Record<string, unknown> {
	const values: Record<string, unknown> = {};

	for (const field of fields) {
		switch (field.type) {
			case "boolean":
				values[field.key] = field.default === "true";
				break;
			case "number":
				values[field.key] = field.default ? Number(field.default) : 0;
				break;
			case "multiselect":
				values[field.key] = [];
				break;
			default:
				values[field.key] = field.default ?? "";
		}
	}

	return values;
}

/**
 * Transform form values before submission.
 * - textarea fields with keys ending in _uuids or features -> split into arrays
 * - number fields -> ensure numeric
 * - multiselect -> already arrays
 */
function transformFormValues(
	values: Record<string, unknown>,
	fields: PluginPageField[],
): Record<string, unknown> {
	const result: Record<string, unknown> = {};

	for (const field of fields) {
		const value = values[field.key];

		if (
			field.type === "textarea" &&
			typeof value === "string" &&
			(field.key.endsWith("_uuids") || field.key.endsWith("features"))
		) {
			result[field.key] = value
				.split("\n")
				.map((line) => line.trim())
				.filter((line) => line.length > 0);
		} else if (field.type === "number") {
			result[field.key] = Number(value);
		} else {
			result[field.key] = value;
		}
	}

	return result;
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function PluginPageView() {
	const { slug, pagePath } = useParams({ strict: false }) as {
		slug: string;
		pagePath: string;
	};
	const { t } = useTranslation();
	const { data: pluginPages, isLoading: pagesLoading } = usePluginPages();

	// Resolve page info to get collection name and form field schema.
	// Try exact match first, then fallback to path-only match.
	const pageInfo =
		pluginPages?.find(
			(p) => p.plugin_slug === slug && p.path === pagePath,
		) ?? pluginPages?.find((p) => p.path === pagePath);
	const collectionName = pageInfo?.collection || pagePath;
	const pageFields =
		pageInfo?.fields && pageInfo.fields.length > 0
			? pageInfo.fields
			: undefined;

	// Debug: log when page schema is not found despite pages being loaded.
	if (!pagesLoading && pluginPages && !pageInfo) {
		// eslint-disable-next-line no-console
		console.warn("[PluginPage] pageInfo not found", {
			slug,
			pagePath,
			availablePages: pluginPages.map((p) => `${p.plugin_slug}/${p.path}`),
		});
	}

	const { list, create, update, remove } = usePluginCollection(slug, collectionName);

	const [formMode, setFormMode] = useState<"closed" | "create" | "edit">(
		"closed",
	);
	const [editingDocId, setEditingDocId] = useState<string | null>(null);
	const [testResults, setTestResults] = useState<Record<string, { loading: boolean; success?: boolean; message?: string }>>({});

	// Determine if this page supports a "test connection" action.
	// Currently: remnawave-provider panels page.
	const supportsTestConnection = slug === "remnawave-provider" && collectionName === "panel_connections";

	const handleTestConnection = useCallback(async (docId: string) => {
		setTestResults((prev) => ({ ...prev, [docId]: { loading: true } }));
		try {
			const result = await apiPost<{ success: boolean; node_count?: number; version?: string; error?: string }>(
				`/api/remnawave/panels/${docId}/test`,
				{},
			);
			setTestResults((prev) => ({
				...prev,
				[docId]: {
					loading: false,
					success: result.success,
					message: result.success
						? `v${result.version ?? "?"} · ${result.node_count ?? 0} nodes`
						: result.error ?? "Connection failed",
				},
			}));
		} catch (err) {
			setTestResults((prev) => ({
				...prev,
				[docId]: { loading: false, success: false, message: "Request failed" },
			}));
		}
	}, []);

	const documents = list.data ?? [];
	const dataKeys = deriveDataKeys(documents);

	// When fields are declared, use field keys for columns; otherwise auto-detect
	const columnKeys = pageFields ? pageFields.map((f) => f.key) : dataKeys;

	// ─── Table columns ───────────────────────────────────────────────────────

	const columns: ColumnDef<PluginDocument, unknown>[] = [
		...columnKeys.map(
			(key): ColumnDef<PluginDocument, unknown> => ({
				id: `data_${key}`,
				header: pageFields
					? (pageFields.find((f) => f.key === key)?.label ?? prettifyKey(key))
					: prettifyKey(key),
				cell: ({ row }) => {
					const value = row.original.data[key];
					// Mask secret fields — show only last 4 chars
					const fieldDef = pageFields?.find((f) => f.key === key);
					if (fieldDef?.type === "secret" && typeof value === "string" && value.length > 0) {
						const masked = value.length <= 4
							? "••••"
							: "••••••••" + value.slice(-4);
						return (
							<span className="font-mono text-[12px] text-muted-foreground">{masked}</span>
						);
					}
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
			cell: ({ row }) => {
				const docId = row.original.id;
				const testState = testResults[docId];

				return (
					<div className="flex items-center justify-end gap-1">
						{/* Test Connection button — only for supported pages */}
						{supportsTestConnection && (
							<button
								type="button"
								onClick={() => handleTestConnection(docId)}
								disabled={testState?.loading}
								title={testState?.message ?? "Test connection"}
								className={cn(
									"flex items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium transition-colors",
									testState?.loading
										? "text-muted-foreground"
										: testState?.success === true
											? "bg-success/10 text-success"
											: testState?.success === false
												? "bg-destructive/10 text-destructive"
												: "text-muted-foreground hover:bg-secondary hover:text-foreground",
								)}
							>
								{testState?.loading ? (
									<Loader2 size={14} className="animate-spin" />
								) : testState?.success === true ? (
									<Check size={14} />
								) : testState?.success === false ? (
									<XCircle size={14} />
								) : (
									<Plug size={14} />
								)}
								{testState?.loading
									? "Testing..."
									: testState?.message
										? testState.message.length > 25
											? testState.message.slice(0, 25) + "…"
											: testState.message
										: "Test"}
							</button>
						)}
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
				);
			},
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
			{formMode !== "closed" &&
				(pageFields ? (
					<SchemaForm
						fields={pageFields}
						mode={formMode}
						initialValues={
							formMode === "edit" && editingDoc
								? editingDoc.data
								: undefined
						}
						isPending={create.isPending || update.isPending}
						onSubmit={handleFormSubmit}
						onCancel={handleFormClose}
					/>
				) : (
					<PluginDocumentForm
						mode={formMode}
						dataKeys={dataKeys}
						sampleDoc={documents[0]}
						editingDoc={editingDoc}
						isPending={create.isPending || update.isPending}
						onSubmit={handleFormSubmit}
						onClose={handleFormClose}
					/>
				))}

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

// ─── Schema-Driven Form ──────────────────────────────────────────────────────

type SchemaFormProps = {
	fields: PluginPageField[];
	mode: "create" | "edit";
	initialValues: Record<string, unknown> | undefined;
	isPending: boolean;
	onSubmit: (data: Record<string, unknown>) => void;
	onCancel: () => void;
};

function SchemaForm({
	fields,
	mode,
	initialValues,
	isPending,
	onSubmit,
	onCancel,
}: SchemaFormProps) {
	const { t } = useTranslation();

	const schema = buildZodSchemaFromFields(fields);
	type FormValues = z.infer<typeof schema>;

	const defaults = buildDefaultValues(fields);
	const mergedDefaults: Record<string, unknown> = initialValues
		? { ...defaults, ...initialValues }
		: defaults;

	const {
		register,
		handleSubmit,
		setValue,
		watch,
		formState: { errors },
	} = useForm<FormValues>({
		resolver: zodResolver(schema),
		defaultValues: mergedDefaults as FormValues,
	});

	const handleFormSubmit = (values: FormValues) => {
		const transformed = transformFormValues(
			values as Record<string, unknown>,
			fields,
		);
		onSubmit(transformed);
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
					onClick={onCancel}
					className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
				>
					<X size={16} />
				</button>
			</div>

			<form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4">
				<div className="grid gap-4 sm:grid-cols-2">
					{fields.map((field) => (
						<SchemaFormField
							key={field.key}
							field={field}
							register={register}
							setValue={setValue}
							watch={watch}
							error={errors[field.key as keyof FormValues]?.message as string | undefined}
						/>
					))}
				</div>

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
						onClick={onCancel}
						className="rounded-lg border border-border bg-card px-4 py-2 text-[13px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
					>
						{t("common.cancel")}
					</button>
				</div>
			</form>
		</div>
	);
}

// ─── Schema Form Field Renderer ──────────────────────────────────────────────

type SchemaFormFieldProps = {
	field: PluginPageField;
	register: ReturnType<typeof useForm>["register"];
	setValue: ReturnType<typeof useForm>["setValue"];
	watch: ReturnType<typeof useForm>["watch"];
	error: string | undefined;
};

function SchemaFormField({
	field,
	register,
	setValue,
	watch,
	error,
}: SchemaFormFieldProps) {
	const fieldId = `schema_${field.key}`;

	switch (field.type) {
		case "boolean":
			return (
				<BooleanField
					field={field}
					fieldId={fieldId}
					watch={watch}
					setValue={setValue}
				/>
			);
		case "select":
			return (
				<SelectField
					field={field}
					fieldId={fieldId}
					register={register}
					error={error}
				/>
			);
		case "multiselect":
			return (
				<MultiselectField
					field={field}
					fieldId={fieldId}
					watch={watch}
					setValue={setValue}
					error={error}
				/>
			);
		case "textarea":
			return (
				<TextareaField
					field={field}
					fieldId={fieldId}
					register={register}
					error={error}
				/>
			);
		case "number":
			return (
				<NumberField
					field={field}
					fieldId={fieldId}
					register={register}
					error={error}
				/>
			);
		case "secret":
			return (
				<SecretField
					field={field}
					fieldId={fieldId}
					register={register}
					error={error}
				/>
			);
		default:
			return (
				<TextField
					field={field}
					fieldId={fieldId}
					register={register}
					error={error}
				/>
			);
	}
}

// ─── Field Label ─────────────────────────────────────────────────────────────

type FieldLabelProps = {
	field: PluginPageField;
	fieldId: string;
};

function FieldLabel({ field, fieldId }: FieldLabelProps) {
	return (
		<label
			htmlFor={fieldId}
			className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground"
		>
			{field.label}
			{field.required && <span className="ml-0.5 text-destructive">*</span>}
		</label>
	);
}

// ─── Text Field ──────────────────────────────────────────────────────────────

type TextFieldProps = {
	field: PluginPageField;
	fieldId: string;
	register: ReturnType<typeof useForm>["register"];
	error: string | undefined;
};

function TextField({ field, fieldId, register, error }: TextFieldProps) {
	return (
		<div>
			<FieldLabel field={field} fieldId={fieldId} />
			<input
				id={fieldId}
				type="text"
				{...register(field.key)}
				className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
			/>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Secret Field ───────────────────────────────────────────────────────────

type SecretFieldProps = {
	field: PluginPageField;
	fieldId: string;
	register: ReturnType<typeof useForm>["register"];
	error: string | undefined;
};

function SecretField({ field, fieldId, register, error }: SecretFieldProps) {
	return (
		<div>
			<FieldLabel field={field} fieldId={fieldId} />
			<input
				id={fieldId}
				type="password"
				autoComplete="off"
				{...register(field.key)}
				placeholder="••••••••"
				className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
			/>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Number Field ────────────────────────────────────────────────────────────

type NumberFieldProps = {
	field: PluginPageField;
	fieldId: string;
	register: ReturnType<typeof useForm>["register"];
	error: string | undefined;
};

function NumberField({ field, fieldId, register, error }: NumberFieldProps) {
	return (
		<div>
			<FieldLabel field={field} fieldId={fieldId} />
			<input
				id={fieldId}
				type="number"
				step="any"
				{...register(field.key, { valueAsNumber: true })}
				className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
			/>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Textarea Field ──────────────────────────────────────────────────────────

type TextareaFieldProps = {
	field: PluginPageField;
	fieldId: string;
	register: ReturnType<typeof useForm>["register"];
	error: string | undefined;
};

function TextareaField({
	field,
	fieldId,
	register,
	error,
}: TextareaFieldProps) {
	return (
		<div className="sm:col-span-2">
			<FieldLabel field={field} fieldId={fieldId} />
			<textarea
				id={fieldId}
				rows={4}
				{...register(field.key)}
				className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
			/>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Boolean Field ───────────────────────────────────────────────────────────

type BooleanFieldProps = {
	field: PluginPageField;
	fieldId: string;
	watch: ReturnType<typeof useForm>["watch"];
	setValue: ReturnType<typeof useForm>["setValue"];
};

function BooleanField({ field, fieldId, watch, setValue }: BooleanFieldProps) {
	const currentValue = watch(field.key) as boolean;

	return (
		<div className="flex items-center gap-3">
			<button
				type="button"
				id={fieldId}
				role="switch"
				aria-label={field.label}
				aria-checked={!!currentValue}
				onClick={() => setValue(field.key, !currentValue)}
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
				{field.label}
				{field.required && (
					<span className="ml-0.5 text-destructive">*</span>
				)}
			</span>
		</div>
	);
}

// ─── Select Field ────────────────────────────────────────────────────────────

type SelectFieldProps = {
	field: PluginPageField;
	fieldId: string;
	register: ReturnType<typeof useForm>["register"];
	error: string | undefined;
};

function SelectField({ field, fieldId, register, error }: SelectFieldProps) {
	const { t } = useTranslation();

	return (
		<div>
			<FieldLabel field={field} fieldId={fieldId} />
			<select
				id={fieldId}
				{...register(field.key)}
				className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
			>
				<option value="">{t("admin.pluginPage.selectPlaceholder")}</option>
				{field.options?.map((opt) => (
					<option key={opt} value={opt}>
						{opt}
					</option>
				))}
			</select>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Multiselect Field ───────────────────────────────────────────────────────

type MultiselectFieldProps = {
	field: PluginPageField;
	fieldId: string;
	watch: ReturnType<typeof useForm>["watch"];
	setValue: ReturnType<typeof useForm>["setValue"];
	error: string | undefined;
};

function MultiselectField({
	field,
	fieldId,
	watch,
	setValue,
	error,
}: MultiselectFieldProps) {
	const selectedValues = (watch(field.key) as string[]) ?? [];

	if (field.options_url) {
		return (
			<DynamicMultiselect
				field={field}
				fieldId={fieldId}
				selectedValues={selectedValues}
				onToggle={(optionValue) => {
					const next = selectedValues.includes(optionValue)
						? selectedValues.filter((v) => v !== optionValue)
						: [...selectedValues, optionValue];
					setValue(field.key, next);
				}}
				error={error}
			/>
		);
	}

	return (
		<div className="sm:col-span-2">
			<FieldLabel field={field} fieldId={fieldId} />
			<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{field.options?.map((opt) => {
					const isChecked = selectedValues.includes(opt);
					return (
						<label
							key={opt}
							className={cn(
								"flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-[13px] transition-colors",
								isChecked
									? "border-primary/30 bg-primary/5 text-foreground"
									: "border-border bg-background text-muted-foreground hover:border-border/80",
							)}
						>
							<input
								type="checkbox"
								checked={isChecked}
								onChange={() => {
									const next = isChecked
										? selectedValues.filter((v) => v !== opt)
										: [...selectedValues, opt];
									setValue(field.key, next);
								}}
								className="h-3.5 w-3.5 rounded border-border accent-primary"
							/>
							<span className="font-mono text-[12px]">{opt}</span>
						</label>
					);
				})}
			</div>
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Dynamic Multiselect (options from URL) ──────────────────────────────────

type DynamicMultiselectProps = {
	field: PluginPageField;
	fieldId: string;
	selectedValues: string[];
	onToggle: (value: string) => void;
	error: string | undefined;
};

function DynamicMultiselect({
	field,
	fieldId,
	selectedValues,
	onToggle,
	error,
}: DynamicMultiselectProps) {
	const { t } = useTranslation();

	const { data: remoteOptions, isLoading: optionsLoading } = useQuery({
		queryKey: ["plugin-options", field.options_url],
		queryFn: () =>
			apiGet<Record<string, unknown>[]>(field.options_url as string),
		enabled: !!field.options_url,
	});

	const valueKey = field.options_value_key ?? "value";
	const labelKey = field.options_label_key ?? "label";

	return (
		<div className="sm:col-span-2">
			<FieldLabel field={field} fieldId={fieldId} />
			{optionsLoading ? (
				<p className="text-[12px] text-muted-foreground">
					{t("admin.pluginPage.loadingOptions")}
				</p>
			) : !remoteOptions || remoteOptions.length === 0 ? (
				<p className="text-[12px] text-muted-foreground">
					{t("admin.pluginPage.noOptions")}
				</p>
			) : (
				<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
					{remoteOptions.map((option) => {
						const optValue = String(option[valueKey] ?? "");
						const optLabel = String(option[labelKey] ?? optValue);
						const isChecked = selectedValues.includes(optValue);
						return (
							<label
								key={optValue}
								className={cn(
									"flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-[13px] transition-colors",
									isChecked
										? "border-primary/30 bg-primary/5 text-foreground"
										: "border-border bg-background text-muted-foreground hover:border-border/80",
								)}
							>
								<input
									type="checkbox"
									checked={isChecked}
									onChange={() => onToggle(optValue)}
									className="h-3.5 w-3.5 rounded border-border accent-primary"
								/>
								<span className="font-mono text-[12px]">{optLabel}</span>
							</label>
						);
					})}
				</div>
			)}
			{error && (
				<p className="mt-1 text-[11px] text-destructive">{String(error)}</p>
			)}
		</div>
	);
}

// ─── Auto-Detect Document Form (fallback) ────────────────────────────────────

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
	const [rawJson, setRawJson] = useState("{}");
	const [rawJsonError, setRawJsonError] = useState<string | null>(null);
	const useRawJsonMode = dataKeys.length === 0;

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

	const handleRawJsonSubmit = () => {
		setRawJsonError(null);
		try {
			const parsed: unknown = JSON.parse(rawJson);
			if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
				setRawJsonError(t("admin.pluginPage.jsonMustBeObject"));
				return;
			}
			onSubmit(parsed as Record<string, unknown>);
		} catch {
			setRawJsonError(t("admin.pluginPage.invalidJson"));
		}
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

			{useRawJsonMode ? (
				<div className="space-y-4">
					<div>
						<label
							htmlFor="raw_json"
							className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
						>
							{t("admin.pluginPage.documentJson")}
						</label>
						<textarea
							id="raw_json"
							rows={6}
							value={rawJson}
							onChange={(e) => setRawJson(e.target.value)}
							className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[12px] text-foreground placeholder:text-muted-foreground/50 transition-colors focus:border-primary/50 focus:outline-none focus:ring-1 focus:ring-primary/50"
							placeholder='{"name": "value", "price": 100}'
						/>
						{rawJsonError && (
							<p className="mt-1 text-[11px] text-destructive">{rawJsonError}</p>
						)}
					</div>
					<div className="flex items-center gap-3 pt-2">
						<button
							type="button"
							onClick={handleRawJsonSubmit}
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
				</div>
			) : (
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
			)}
		</div>
	);
}
