import { zodResolver } from "@hookform/resolvers/zod";
import type { PluginDocument, PluginPageField } from "@remnacore/shared";
import {
  apiGet,
  apiPost,
  cn,
  formatDateTime,
  LoadingSpinner,
  usePluginCollection,
  usePluginPages,
} from "@remnacore/shared";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import {
  Check,
  ChevronDown,
  Loader2,
  Pencil,
  Plug,
  Plus,
  Trash2,
  X,
  XCircle,
} from "lucide-react";
import { lazy, Suspense, useCallback, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { PageHeader, Panel, PanelHeader, TermButton } from "@/components/ui";
import { DataTable } from "../../components/DataTable.js";

// ─── Shared terminal field styles ───────────────────────────────────────────

const FIELD_INPUT_CLS =
  "w-full border border-[var(--line-strong)] bg-input px-3 py-3 font-mono text-[13px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45";
const FIELD_TEXTAREA_CLS =
  "w-full resize-none border border-[var(--line-strong)] bg-input px-3 py-3 font-mono text-[12px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45";
const FIELD_LABEL_CLS =
  "mb-2 block text-[9px] uppercase tracking-[1.5px] text-t6";
const FIELD_ERROR_CLS = "mt-1 text-[9px] uppercase tracking-[1px] text-danger";

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
  if (Array.isArray(value)) {
    if (value.length === 0) return "—";
    if (typeof value[0] === "object") return `${value.length} items`;
    return value.join(", ");
  }
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
      case "pricing_periods":
        shape[field.key] = z.any().optional();
        break;
      case "textarea":
        shape[field.key] = field.required ? z.string().min(1) : z.string();
        break;
      case "select":
        shape[field.key] = field.required ? z.string().min(1) : z.string();
        break;
      default:
        shape[field.key] = field.required ? z.string().min(1) : z.string();
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
      case "pricing_periods":
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

    if (field.type === "pricing_periods") {
      // Pass array as-is; filter out empty periods.
      if (Array.isArray(value)) {
        result[field.key] = (
          value as { duration_days: number; price_amount: number }[]
        ).filter((p) => p.duration_days > 0 || p.price_amount > 0);
      } else {
        result[field.key] = [];
      }
    } else if (
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

// ─── Custom Page Registry ────────────────────────────────────────────────────
// Maps plugin_slug/pagePath → lazy-loaded custom React component.
// When a match is found, the custom component is rendered instead of generic CRUD.

const CUSTOM_PAGES: Record<
  string,
  React.LazyExoticComponent<React.ComponentType>
> = {
  "remnawave-provider/dashboard": lazy(
    () => import("../../components/remnawave/RemnawaveDashboard.js"),
  ),
  "remnawave-provider/nodes": lazy(
    () => import("../../components/remnawave/RemnawaveNodes.js"),
  ),
  "remnawave-provider/users": lazy(
    () => import("../../components/remnawave/RemnawaveUsers.js"),
  ),
  "remnawave-provider/ip-management": lazy(
    () => import("../../components/remnawave/RemnawaveIPManagement.js"),
  ),
  "remnawave-provider/squads-internal": lazy(
    () => import("../../components/remnawave/RemnawaveSquadsInternal.js"),
  ),
  "remnawave-provider/squads-external": lazy(
    () => import("../../components/remnawave/RemnawaveSquadsExternal.js"),
  ),
  "remnawave-provider/hosts": lazy(
    () => import("../../components/remnawave/RemnawaveHosts.js"),
  ),
  "remnawave-provider/subscription-configs": lazy(
    () => import("../../components/remnawave/RemnawaveSubPageConfigs.js"),
  ),
};

// ─── Main Component ───────────────────────────────────────────────────────────
// Split into two components to avoid React hooks rule violation:
// PluginPageView handles the custom/generic routing decision WITHOUT hooks
// that differ between paths. GenericPluginPage has all the CRUD hooks.

export function PluginPageView() {
  const { slug, pagePath } = useParams({ strict: false }) as {
    slug: string;
    pagePath: string;
  };

  const customKey = `${slug}/${pagePath}`;
  const CustomComponent = CUSTOM_PAGES[customKey];

  if (CustomComponent) {
    return (
      <Suspense fallback={<LoadingSpinner />}>
        <CustomComponent />
      </Suspense>
    );
  }

  return <GenericPluginPage slug={slug} pagePath={pagePath} />;
}

function GenericPluginPage({
  slug,
  pagePath,
}: {
  slug: string;
  pagePath: string;
}) {
  const { t } = useTranslation();
  const { data: pluginPages, isLoading: pagesLoading } = usePluginPages();

  // Resolve page info to get collection name and form field schema.
  // Try exact match first, then fallback to path-only match.
  const pageInfo =
    pluginPages?.find((p) => p.plugin_slug === slug && p.path === pagePath) ??
    pluginPages?.find((p) => p.path === pagePath);
  const collectionName = pageInfo?.collection || pagePath;
  const crudUrl = pageInfo?.crud_url;
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

  const { list, create, update, remove } = usePluginCollection(
    slug,
    collectionName,
    crudUrl,
  );

  const [formMode, setFormMode] = useState<"closed" | "create" | "edit">(
    "closed",
  );
  const [editingDocId, setEditingDocId] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<
    Record<string, { loading: boolean; success?: boolean; message?: string }>
  >({});

  // Determine if this page supports a "test connection" action.
  // Currently: remnawave-provider panels page.
  const supportsTestConnection =
    slug === "remnawave-provider" && collectionName === "panel_connections";

  const handleTestConnection = useCallback(async (docId: string) => {
    setTestResults((prev) => ({ ...prev, [docId]: { loading: true } }));
    try {
      const result = await apiPost<{
        success: boolean;
        node_count?: number;
        version?: string;
        error?: string;
      }>(`/api/remnawave/panels/${docId}/test`, {});
      setTestResults((prev) => ({
        ...prev,
        [docId]: {
          loading: false,
          success: result.success,
          message: result.success
            ? `v${result.version ?? "?"} · ${result.node_count ?? 0} nodes`
            : (result.error ?? "Connection failed"),
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

  // When fields are declared, show a compact subset of columns in the table.
  // Priority: required fields first, then first few non-textarea/non-multiselect fields.
  // Max 6 data columns to fit on screen.
  const columnKeys = (() => {
    if (!pageFields) return dataKeys.slice(0, 6);
    const compactTypes = new Set(["text", "number", "select", "boolean"]);
    const candidates = pageFields.filter((f) => compactTypes.has(f.type));
    const required = candidates.filter((f) => f.required);
    const rest = candidates.filter((f) => !f.required);
    const picked = [...required, ...rest].slice(0, 6);
    return picked.map((f) => f.key);
  })();

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
          if (
            fieldDef?.type === "secret" &&
            typeof value === "string" &&
            value.length > 0
          ) {
            const masked =
              value.length <= 4 ? "••••" : "••••••••" + value.slice(-4);
            return (
              <span className="font-mono text-[12px] tabular-nums text-t6">
                {masked}
              </span>
            );
          }
          return (
            <span className="block max-w-[200px] truncate text-[12px] text-t2">
              {formatCellValue(value, t)}
            </span>
          );
        },
      }),
    ),
    {
      id: "created_at",
      header: t("admin.pluginPage.fieldCreatedAt"),
      cell: ({ row }) => (
        <span className="font-mono text-[11px] tabular-nums text-t5">
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
                  "flex items-center gap-1.5 border px-2.5 py-1.5 text-[10px] uppercase tracking-[1px] transition-colors",
                  testState?.loading
                    ? "border-line text-t6"
                    : testState?.success === true
                      ? "border-accent text-accent"
                      : testState?.success === false
                        ? "border-danger text-danger"
                        : "border-line text-t5 hover:text-t2",
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
              className="p-1.5 text-t6 transition-colors hover:text-accent"
            >
              <Pencil size={14} />
            </button>
            <button
              type="button"
              onClick={() => handleDelete(row.original.id)}
              className="p-1.5 text-t6 transition-colors hover:text-danger"
            >
              <Trash2 size={14} />
            </button>
          </div>
        );
      },
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
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("admin.pluginPage.errorLoading")}
      </div>
    );
  }

  const editingDoc = editingDocId
    ? documents.find((d) => d.id === editingDocId)
    : undefined;

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="min-w-0 space-y-3.5 overflow-hidden">
      {/* Header */}
      <PageHeader
        title={(
          pageInfo?.title ?? t("admin.pluginPage.unknownPage")
        ).toUpperCase()}
        breadcrumb={
          pageInfo
            ? `${pageInfo.plugin_name} / ${pageInfo.path}`.toUpperCase()
            : undefined
        }
        right={
          <TermButton type="button" variant="primary" onClick={handleCreate}>
            <Plus size={14} />
            {t("admin.pluginPage.createItem")}
          </TermButton>
        }
      />

      {/* Form (create/edit) */}
      {formMode !== "closed" &&
        (pageFields ? (
          <SchemaForm
            fields={pageFields}
            mode={formMode}
            initialValues={
              formMode === "edit" && editingDoc ? editingDoc.data : undefined
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
        <div className="border border-line bg-surface px-4 py-10 text-center text-[11px] uppercase tracking-[2px] text-t7">
          {t("admin.pluginPage.noDocuments")}
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
    <Panel className="min-w-0">
      <PanelHeader
        title={
          mode === "create"
            ? t("admin.pluginPage.createTitle")
            : t("admin.pluginPage.editTitle")
        }
        right={
          <button
            type="button"
            onClick={onCancel}
            aria-label={t("common.cancel")}
            className="p-1 text-t7 transition-colors hover:text-t2"
          >
            <X size={16} />
          </button>
        }
      />

      <form
        onSubmit={handleSubmit(handleFormSubmit)}
        className="space-y-4 px-4 py-5 sm:px-5"
      >
        <GroupedFieldLayout
          fields={fields}
          register={register}
          setValue={setValue}
          watch={watch}
          errors={errors}
        />

        <div className="flex items-center gap-2.5 pt-1">
          <TermButton type="submit" variant="primary" disabled={isPending}>
            {isPending ? (
              <>
                <Loader2 size={14} className="animate-spin" />
                {t("admin.pluginPage.saving")}
              </>
            ) : (
              t("common.save")
            )}
          </TermButton>
          <TermButton type="button" variant="ghost" onClick={onCancel}>
            {t("common.cancel")}
          </TermButton>
        </div>
      </form>
    </Panel>
  );
}

// ─── Grouped Field Layout ───────────────────────────────────────────────────
// Groups fields by their `group` property into collapsible sections.
// Fields without a group are rendered in a flat grid at the top.

function GroupedFieldLayout({
  fields,
  register,
  setValue,
  watch,
  errors,
}: {
  fields: PluginPageField[];
  register: ReturnType<typeof useForm>["register"];
  setValue: ReturnType<typeof useForm>["setValue"];
  watch: ReturnType<typeof useForm>["watch"];
  errors: Record<string, { message?: string } | undefined>;
}) {
  const hasGroups = fields.some((f) => f.group);

  // No groups — flat 2-column grid (backward compat).
  if (!hasGroups) {
    return (
      <div className="grid gap-4 sm:grid-cols-2">
        {fields.map((field) => (
          <SchemaFormField
            key={field.key}
            field={field}
            register={register}
            setValue={setValue}
            watch={watch}
            error={errors[field.key]?.message as string | undefined}
          />
        ))}
      </div>
    );
  }

  // Group fields preserving order.
  const groups: { name: string; fields: PluginPageField[] }[] = [];
  const seen = new Set<string>();
  for (const field of fields) {
    const g = field.group || "_ungrouped";
    if (!seen.has(g)) {
      seen.add(g);
      groups.push({ name: g, fields: [] });
    }
    groups.find((x) => x.name === g)!.fields.push(field);
  }

  return (
    <div className="space-y-3">
      {groups.map((group) => (
        <FieldSection
          key={group.name}
          title={group.name === "_ungrouped" ? undefined : group.name}
          defaultOpen={group.name === "_ungrouped" || group.name === "Basic"}
        >
          <div className="grid gap-x-4 gap-y-3 sm:grid-cols-2">
            {group.fields.map((field) => (
              <SchemaFormField
                key={field.key}
                field={field}
                register={register}
                setValue={setValue}
                watch={watch}
                error={errors[field.key]?.message as string | undefined}
              />
            ))}
          </div>
        </FieldSection>
      ))}
    </div>
  );
}

function FieldSection({
  title,
  defaultOpen = false,
  children,
}: {
  title?: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  if (!title) {
    return <>{children}</>;
  }

  return (
    <div className="border border-line bg-input/40">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-4 py-2.5 text-left"
      >
        <span className="text-[10px] uppercase tracking-[1.5px] text-t6">
          {title}
        </span>
        <ChevronDown
          size={14}
          className={cn("text-t6 transition-transform", open && "rotate-180")}
        />
      </button>
      {open && <div className="px-4 pb-4">{children}</div>}
    </div>
  );
}

// ─── Pricing Periods Field ──────────────────────────────────────────────────
// Custom UX for adding/removing pricing periods instead of raw JSON textarea.

type PricingPeriod = {
  duration_days: number;
  price_amount: number;
  label: string;
  save_percent: number;
  is_default: boolean;
};

function PricingPeriodsField({
  watch,
  setValue,
}: {
  watch: ReturnType<typeof useForm>["watch"];
  setValue: ReturnType<typeof useForm>["setValue"];
}) {
  const rawValue = watch("pricing_periods");

  // Parse from string (textarea legacy) or array.
  const periods: PricingPeriod[] = (() => {
    if (Array.isArray(rawValue)) return rawValue;
    if (typeof rawValue === "string" && rawValue.trim()) {
      try {
        const parsed = JSON.parse(rawValue);
        return Array.isArray(parsed) ? parsed : [];
      } catch {
        return [];
      }
    }
    return [];
  })();

  const update = (next: PricingPeriod[]) => {
    setValue("pricing_periods", next);
  };

  const addPeriod = () => {
    update([
      ...periods,
      {
        duration_days: 30,
        price_amount: 0,
        label: "",
        save_percent: 0,
        is_default: false,
      },
    ]);
  };

  const removePeriod = (index: number) => {
    update(periods.filter((_, i) => i !== index));
  };

  const updatePeriod = (index: number, partial: Partial<PricingPeriod>) => {
    update(periods.map((p, i) => (i === index ? { ...p, ...partial } : p)));
  };

  const inputCls =
    "w-full border border-[var(--line-strong)] bg-input px-2.5 py-2 font-mono text-[12px] tabular-nums text-t1 outline-none transition-colors focus:border-accent/45";

  return (
    <div className="sm:col-span-2 space-y-3">
      {periods.length > 0 && (
        <div className="space-y-2">
          {/* Header */}
          <div className="hidden sm:grid sm:grid-cols-[1fr_1fr_1fr_80px_60px_40px] gap-2 px-1 text-[9px] uppercase tracking-[1.5px] text-t7">
            <span>Days</span>
            <span>Price (cents)</span>
            <span>Label</span>
            <span>Discount %</span>
            <span>Default</span>
            <span />
          </div>
          {periods.map((period, i) => (
            <div
              key={`period-${i}`}
              className="grid gap-2 grid-cols-2 sm:grid-cols-[1fr_1fr_1fr_80px_60px_40px] items-center border border-line p-2 sm:p-0 sm:border-0"
            >
              <input
                type="number"
                value={period.duration_days}
                onChange={(e) =>
                  updatePeriod(i, { duration_days: Number(e.target.value) })
                }
                placeholder="Days"
                className={inputCls}
              />
              <input
                type="number"
                value={period.price_amount}
                onChange={(e) =>
                  updatePeriod(i, { price_amount: Number(e.target.value) })
                }
                placeholder="Price"
                className={inputCls}
              />
              <input
                type="text"
                value={period.label}
                onChange={(e) => updatePeriod(i, { label: e.target.value })}
                placeholder="e.g. 1 month"
                className={cn(inputCls, "col-span-2 sm:col-span-1")}
              />
              <input
                type="number"
                value={period.save_percent}
                onChange={(e) =>
                  updatePeriod(i, { save_percent: Number(e.target.value) })
                }
                placeholder="%"
                min={0}
                max={100}
                className={inputCls}
              />
              <div className="flex items-center justify-center">
                <button
                  type="button"
                  onClick={() => {
                    const next = periods.map((p, j) => ({
                      ...p,
                      is_default: j === i,
                    }));
                    update(next);
                  }}
                  className={cn(
                    "h-5 w-5 rounded-full border-2 transition-colors",
                    period.is_default
                      ? "border-accent bg-accent"
                      : "border-line hover:border-accent/50",
                  )}
                >
                  {period.is_default && (
                    <Check size={12} className="mx-auto text-on-accent" />
                  )}
                </button>
              </div>
              <button
                type="button"
                onClick={() => removePeriod(i)}
                className="flex items-center justify-center p-1.5 text-t6 transition-colors hover:text-danger"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
      <button
        type="button"
        onClick={addPeriod}
        className="flex items-center gap-1.5 border border-dashed border-line px-3 py-2 text-[10px] uppercase tracking-[1px] text-t6 transition-colors hover:border-accent/50 hover:text-t2"
      >
        <Plus size={13} />
        Add period
      </button>
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
  const spanClass = field.span === 2 ? "sm:col-span-2" : "";

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
    case "pricing_periods":
      return (
        <div className="sm:col-span-2">
          <PricingPeriodsField watch={watch} setValue={setValue} />
        </div>
      );
    case "number":
      return (
        <div className={spanClass}>
          <NumberField
            field={field}
            fieldId={fieldId}
            register={register}
            error={error}
          />
        </div>
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
        <div className={spanClass}>
          <TextField
            field={field}
            fieldId={fieldId}
            register={register}
            error={error}
          />
        </div>
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
    <label htmlFor={fieldId} className={FIELD_LABEL_CLS}>
      {field.label}
      {field.required && <span className="ml-0.5 text-accent">*</span>}
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
        className={FIELD_INPUT_CLS}
      />
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
        className={FIELD_INPUT_CLS}
      />
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
        className={cn(FIELD_INPUT_CLS, "tabular-nums")}
      />
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
        className={FIELD_TEXTAREA_CLS}
      />
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
          "relative h-5 w-9 transition-colors",
          currentValue ? "bg-accent" : "bg-[var(--line-strong)]",
        )}
      >
        <span
          className={cn(
            "absolute left-0.5 top-0.5 h-4 w-4 transition-transform",
            currentValue ? "translate-x-4 bg-on-accent" : "bg-t6",
          )}
        />
      </button>
      <span className="text-[12px] text-t3">
        {field.label}
        {field.required && <span className="ml-0.5 text-accent">*</span>}
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

  // Support dynamic options from URL (e.g., vpn_panel_id)
  const { data: remoteOptions } = useQuery({
    queryKey: ["plugin-select-options", field.options_url],
    queryFn: () =>
      apiGet<Record<string, unknown>[]>(field.options_url as string),
    enabled: !!field.options_url,
  });

  const valueKey = field.options_value_key ?? "value";
  const labelKey = field.options_label_key ?? "label";

  return (
    <div>
      <FieldLabel field={field} fieldId={fieldId} />
      <select id={fieldId} {...register(field.key)} className={FIELD_INPUT_CLS}>
        <option value="">{t("admin.pluginPage.selectPlaceholder")}</option>
        {field.options_url && remoteOptions
          ? remoteOptions.map((opt) => {
              const v = String(opt[valueKey] ?? "");
              const l = String(opt[labelKey] ?? v);
              return (
                <option key={v} value={v}>
                  {l}
                </option>
              );
            })
          : field.options?.map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
      </select>
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
        watch={watch}
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
                "flex cursor-pointer items-center gap-2 border px-3 py-2 text-[12px] transition-colors",
                isChecked
                  ? "border-accent/40 bg-accent/5 text-t2"
                  : "border-line bg-input text-t5 hover:border-[var(--line-strong)]",
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
                className="h-3.5 w-3.5 accent-[var(--accent)]"
              />
              <span className="font-mono text-[12px]">{opt}</span>
            </label>
          );
        })}
      </div>
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
  watch?: ReturnType<typeof useForm>["watch"];
};

function DynamicMultiselect({
  field,
  fieldId,
  selectedValues,
  onToggle,
  error,
  watch: formWatch,
}: DynamicMultiselectProps) {
  const { t } = useTranslation();

  // For squad fields, append ?panel_id= from the vpn_panel_id form field
  let optionsUrl = field.options_url as string;
  const panelId = formWatch?.("vpn_panel_id") as string | undefined;
  if (
    panelId &&
    (field.key === "internal_squad_uuids" ||
      field.key === "external_squad_uuids")
  ) {
    optionsUrl = `${optionsUrl}?panel_id=${panelId}`;
  }

  const { data: remoteOptions, isLoading: optionsLoading } = useQuery({
    queryKey: ["plugin-options", optionsUrl],
    queryFn: () => apiGet<Record<string, unknown>[]>(optionsUrl),
    enabled: !!field.options_url,
  });

  const valueKey = field.options_value_key ?? "value";
  const labelKey = field.options_label_key ?? "label";

  return (
    <div className="sm:col-span-2">
      <FieldLabel field={field} fieldId={fieldId} />
      {optionsLoading ? (
        <p className="text-[10px] uppercase tracking-[1px] text-t6">
          {t("admin.pluginPage.loadingOptions")}
        </p>
      ) : !remoteOptions || remoteOptions.length === 0 ? (
        <p className="text-[10px] uppercase tracking-[1px] text-t6">
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
                  "flex cursor-pointer items-center gap-2 border px-3 py-2 text-[12px] transition-colors",
                  isChecked
                    ? "border-accent/40 bg-accent/5 text-t2"
                    : "border-line bg-input text-t5 hover:border-[var(--line-strong)]",
                )}
              >
                <input
                  type="checkbox"
                  checked={isChecked}
                  onChange={() => onToggle(optValue)}
                  className="h-3.5 w-3.5 accent-[var(--accent)]"
                />
                <span className="font-mono text-[12px]">{optLabel}</span>
              </label>
            );
          })}
        </div>
      )}
      {error && <p className={FIELD_ERROR_CLS}>{String(error)}</p>}
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
      if (
        typeof parsed !== "object" ||
        parsed === null ||
        Array.isArray(parsed)
      ) {
        setRawJsonError(t("admin.pluginPage.jsonMustBeObject"));
        return;
      }
      onSubmit(parsed as Record<string, unknown>);
    } catch {
      setRawJsonError(t("admin.pluginPage.invalidJson"));
    }
  };

  return (
    <Panel className="min-w-0">
      <PanelHeader
        title={
          mode === "create"
            ? t("admin.pluginPage.createTitle")
            : t("admin.pluginPage.editTitle")
        }
        right={
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.cancel")}
            className="p-1 text-t7 transition-colors hover:text-t2"
          >
            <X size={16} />
          </button>
        }
      />

      {useRawJsonMode ? (
        <div className="space-y-4 px-4 py-5 sm:px-5">
          <div>
            <label htmlFor="raw_json" className={FIELD_LABEL_CLS}>
              {t("admin.pluginPage.documentJson")}
            </label>
            <textarea
              id="raw_json"
              rows={6}
              value={rawJson}
              onChange={(e) => setRawJson(e.target.value)}
              className={FIELD_TEXTAREA_CLS}
              placeholder='{"name": "value", "price": 100}'
            />
            {rawJsonError && <p className={FIELD_ERROR_CLS}>{rawJsonError}</p>}
          </div>
          <div className="flex items-center gap-2.5 pt-1">
            <TermButton
              type="button"
              variant="primary"
              onClick={handleRawJsonSubmit}
              disabled={isPending}
            >
              {isPending ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  {t("admin.pluginPage.saving")}
                </>
              ) : (
                t("common.save")
              )}
            </TermButton>
            <TermButton type="button" variant="ghost" onClick={onClose}>
              {t("common.cancel")}
            </TermButton>
          </div>
        </div>
      ) : (
        <form
          onSubmit={handleSubmit(handleFormSubmit)}
          className="space-y-4 px-4 py-5 sm:px-5"
        >
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
                        "relative h-5 w-9 transition-colors",
                        currentValue ? "bg-accent" : "bg-[var(--line-strong)]",
                      )}
                    >
                      <span
                        className={cn(
                          "absolute left-0.5 top-0.5 h-4 w-4 transition-transform",
                          currentValue ? "translate-x-4 bg-on-accent" : "bg-t6",
                        )}
                      />
                    </button>
                    <span className="text-[12px] text-t3">
                      {prettifyKey(key)}
                    </span>
                  </div>
                );
              }

              return (
                <div key={key}>
                  <label htmlFor={`field_${key}`} className={FIELD_LABEL_CLS}>
                    {prettifyKey(key)}
                  </label>
                  <input
                    id={`field_${key}`}
                    type={fieldType === "number" ? "number" : "text"}
                    step={fieldType === "number" ? "any" : undefined}
                    {...register(key as keyof FormValues, {
                      valueAsNumber: fieldType === "number",
                    })}
                    className={cn(
                      FIELD_INPUT_CLS,
                      fieldType === "number" && "tabular-nums",
                    )}
                  />
                  {errors[key as keyof FormValues] && (
                    <p className={FIELD_ERROR_CLS}>
                      {String(errors[key as keyof FormValues]?.message)}
                    </p>
                  )}
                </div>
              );
            })}
          </div>

          <div className="flex items-center gap-2.5 pt-1">
            <TermButton type="submit" variant="primary" disabled={isPending}>
              {isPending ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  {t("admin.pluginPage.saving")}
                </>
              ) : (
                t("common.save")
              )}
            </TermButton>
            <TermButton type="button" variant="ghost" onClick={onClose}>
              {t("common.cancel")}
            </TermButton>
          </div>
        </form>
      )}
    </Panel>
  );
}
