import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Pencil, Plus, Trash2, X } from "lucide-react";
import {
  cn,
  formatMoney,
  LoadingSpinner,
  useAdminTariffs,
  useCreateTariff,
  useUpdateTariff,
  useDeleteTariff,
} from "@remnacore/shared";
import type { Tariff } from "@remnacore/shared";

// ─── Constants ──────────────────────────────────────────────────────────────

const SUPPORTED_CURRENCIES = [
  "USD",
  "EUR",
  "RUB",
  "GBP",
  "TRY",
  "UAH",
  "KZT",
] as const;

const UUID_TRUNCATE_LENGTH = 8;

// ─── Schema ─────────────────────────────────────────────────────────────────

const tariffFormSchema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string(),
  price_amount: z.number().min(0, "Price must be >= 0"),
  price_currency: z.string().length(3, "Currency must be 3 characters"),
  duration_days: z.number().min(1, "Duration must be > 0"),
  traffic_limit_gb: z.number().min(0),
  device_limit: z.number().min(0),
  max_purchases_per_user: z.number().min(0),
  internal_squad_uuids: z.string(),
  external_squad_uuids: z.string(),
  features: z.string(),
  is_active: z.boolean(),
  sort_order: z.number(),
});

type TariffFormValues = z.infer<typeof tariffFormSchema>;

// ─── Helpers ────────────────────────────────────────────────────────────────

function parseCommaSeparated(value: string): string[] {
  return value
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function parseLinesSeparated(value: string): string[] {
  return value
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

function formatLimit(value: number, unit: string, t: (key: string) => string): string {
  return value === 0 ? t("admin.tariffs.unlimited") : `${value} ${unit}`;
}

function truncateUuid(uuid: string): string {
  return uuid.length > UUID_TRUNCATE_LENGTH
    ? `${uuid.slice(0, UUID_TRUNCATE_LENGTH)}...`
    : uuid;
}

function tariffToFormValues(tariff: Tariff): TariffFormValues {
  return {
    name: tariff.name,
    description: tariff.description,
    price_amount: tariff.price_amount,
    price_currency: tariff.price_currency,
    duration_days: tariff.duration_days,
    traffic_limit_gb: tariff.traffic_limit_gb,
    device_limit: tariff.device_limit,
    max_purchases_per_user: tariff.max_purchases_per_user,
    internal_squad_uuids: tariff.internal_squad_uuids.join(", "),
    external_squad_uuids: tariff.external_squad_uuids.join(", "),
    features: tariff.features.join("\n"),
    is_active: tariff.is_active,
    sort_order: tariff.sort_order,
  };
}

// ─── Input classes ──────────────────────────────────────────────────────────

const INPUT_CLASSES =
  "w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground focus:outline-none focus:ring-1 focus:ring-primary/50";

const LABEL_CLASSES = "mb-1.5 block text-[12px] font-medium text-muted-foreground";

const TEXTAREA_CLASSES = cn(INPUT_CLASSES, "min-h-[72px] resize-y");

// ─── TariffForm ─────────────────────────────────────────────────────────────

type TariffFormProps = {
  initialValues?: TariffFormValues;
  onSubmit: (data: TariffFormValues) => void;
  onCancel: () => void;
  isPending: boolean;
  title: string;
};

function TariffForm({
  initialValues,
  onSubmit,
  onCancel,
  isPending,
  title,
}: TariffFormProps) {
  const { t } = useTranslation();

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<TariffFormValues>({
    resolver: zodResolver(tariffFormSchema),
    defaultValues: initialValues ?? {
      name: "",
      description: "",
      price_amount: 0,
      price_currency: "USD",
      duration_days: 30,
      traffic_limit_gb: 0,
      device_limit: 0,
      max_purchases_per_user: 0,
      internal_squad_uuids: "",
      external_squad_uuids: "",
      features: "",
      is_active: true,
      sort_order: 0,
    },
  });

  return (
    <div className="rounded-xl border border-primary/20 bg-card p-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-[15px] font-semibold text-foreground">{title}</h2>
        <button
          type="button"
          onClick={onCancel}
          className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          <X size={16} />
        </button>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        {/* Row 1: Name + Currency + Price */}
        <div className="grid gap-4 sm:grid-cols-3">
          <div>
            <label htmlFor="tariff-name" className={LABEL_CLASSES}>
              {t("admin.tariffs.name")} *
            </label>
            <input
              id="tariff-name"
              {...register("name")}
              className={INPUT_CLASSES}
            />
            {errors.name && (
              <p className="mt-1 text-[12px] text-destructive">
                {errors.name.message}
              </p>
            )}
          </div>

          <div>
            <label htmlFor="tariff-price" className={LABEL_CLASSES}>
              {t("admin.tariffs.price")} *
            </label>
            <input
              id="tariff-price"
              type="number"
              step="1"
              min="0"
              {...register("price_amount", { valueAsNumber: true })}
              className={INPUT_CLASSES}
            />
            {errors.price_amount && (
              <p className="mt-1 text-[12px] text-destructive">
                {errors.price_amount.message}
              </p>
            )}
          </div>

          <div>
            <label htmlFor="tariff-currency" className={LABEL_CLASSES}>
              {t("admin.tariffs.currency")}
            </label>
            <select
              id="tariff-currency"
              {...register("price_currency")}
              className={INPUT_CLASSES}
            >
              {SUPPORTED_CURRENCIES.map((cur) => (
                <option key={cur} value={cur}>
                  {cur}
                </option>
              ))}
            </select>
            {errors.price_currency && (
              <p className="mt-1 text-[12px] text-destructive">
                {errors.price_currency.message}
              </p>
            )}
          </div>
        </div>

        {/* Row 2: Duration + Traffic + Devices + Max Purchases */}
        <div className="grid gap-4 sm:grid-cols-4">
          <div>
            <label htmlFor="tariff-duration" className={LABEL_CLASSES}>
              {t("admin.tariffs.durationDays")} *
            </label>
            <input
              id="tariff-duration"
              type="number"
              min="1"
              {...register("duration_days", { valueAsNumber: true })}
              className={INPUT_CLASSES}
            />
            {errors.duration_days && (
              <p className="mt-1 text-[12px] text-destructive">
                {errors.duration_days.message}
              </p>
            )}
          </div>

          <div>
            <label htmlFor="tariff-traffic" className={LABEL_CLASSES}>
              {t("admin.tariffs.trafficLimitGb")}
            </label>
            <input
              id="tariff-traffic"
              type="number"
              min="0"
              {...register("traffic_limit_gb", { valueAsNumber: true })}
              className={INPUT_CLASSES}
              placeholder={t("admin.tariffs.zeroUnlimited")}
            />
          </div>

          <div>
            <label htmlFor="tariff-devices" className={LABEL_CLASSES}>
              {t("admin.tariffs.deviceLimit")}
            </label>
            <input
              id="tariff-devices"
              type="number"
              min="0"
              {...register("device_limit", { valueAsNumber: true })}
              className={INPUT_CLASSES}
              placeholder={t("admin.tariffs.zeroUnlimited")}
            />
          </div>

          <div>
            <label htmlFor="tariff-max-purchases" className={LABEL_CLASSES}>
              {t("admin.tariffs.maxPurchasesPerUser")}
            </label>
            <input
              id="tariff-max-purchases"
              type="number"
              min="0"
              {...register("max_purchases_per_user", { valueAsNumber: true })}
              className={INPUT_CLASSES}
              placeholder={t("admin.tariffs.zeroUnlimited")}
            />
          </div>
        </div>

        {/* Row 3: Description (full width) */}
        <div>
          <label htmlFor="tariff-description" className={LABEL_CLASSES}>
            {t("admin.tariffs.description")}
          </label>
          <textarea
            id="tariff-description"
            {...register("description")}
            className={TEXTAREA_CLASSES}
            rows={2}
          />
        </div>

        {/* Row 4: Features (full width) */}
        <div>
          <label htmlFor="tariff-features" className={LABEL_CLASSES}>
            {t("admin.tariffs.features")}
            <span className="ml-2 font-normal text-dim">
              ({t("admin.tariffs.onePerLine")})
            </span>
          </label>
          <textarea
            id="tariff-features"
            {...register("features")}
            className={TEXTAREA_CLASSES}
            rows={3}
          />
        </div>

        {/* Row 5: Squad UUIDs */}
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="tariff-internal-squads" className={LABEL_CLASSES}>
              {t("admin.tariffs.internalSquadUuids")}
              <span className="ml-2 font-normal text-dim">
                ({t("admin.tariffs.commaSeparated")})
              </span>
            </label>
            <textarea
              id="tariff-internal-squads"
              {...register("internal_squad_uuids")}
              className={TEXTAREA_CLASSES}
              rows={2}
            />
          </div>

          <div>
            <label htmlFor="tariff-external-squads" className={LABEL_CLASSES}>
              {t("admin.tariffs.externalSquadUuids")}
              <span className="ml-2 font-normal text-dim">
                ({t("admin.tariffs.commaSeparated")})
              </span>
            </label>
            <textarea
              id="tariff-external-squads"
              {...register("external_squad_uuids")}
              className={TEXTAREA_CLASSES}
              rows={2}
            />
          </div>
        </div>

        {/* Row 6: Sort order + Active toggle */}
        <div className="grid gap-4 sm:grid-cols-3">
          <div>
            <label htmlFor="tariff-sort-order" className={LABEL_CLASSES}>
              {t("admin.tariffs.sortOrder")}
            </label>
            <input
              id="tariff-sort-order"
              type="number"
              {...register("sort_order", { valueAsNumber: true })}
              className={INPUT_CLASSES}
            />
          </div>

          <div className="flex items-end pb-1">
            <Controller
              control={control}
              name="is_active"
              render={({ field }) => (
                <label
                  htmlFor="tariff-active"
                  className="flex cursor-pointer items-center gap-2.5"
                >
                  <button
                    id="tariff-active"
                    type="button"
                    role="switch"
                    aria-checked={field.value}
                    onClick={() => field.onChange(!field.value)}
                    className={cn(
                      "relative h-5 w-9 rounded-full transition-colors",
                      field.value ? "bg-primary" : "bg-border",
                    )}
                  >
                    <span
                      className={cn(
                        "absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white transition-transform",
                        field.value && "translate-x-4",
                      )}
                    />
                  </button>
                  <span className="text-[12px] font-medium text-muted-foreground">
                    {field.value
                      ? t("admin.tariffs.active")
                      : t("admin.tariffs.inactive")}
                  </span>
                </label>
              )}
            />
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3 pt-2">
          <button
            type="submit"
            disabled={isPending}
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-[12px] font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
          >
            {isPending && <Loader2 size={14} className="animate-spin" />}
            {t("common.save")}
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-border bg-card px-4 py-2 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          >
            {t("common.cancel")}
          </button>
        </div>
      </form>
    </div>
  );
}

// ─── TariffCard ─────────────────────────────────────────────────────────────

type TariffCardProps = {
  tariff: Tariff;
  onEdit: (tariff: Tariff) => void;
  onDelete: (tariff: Tariff) => void;
};

function TariffCard({ tariff, onEdit, onDelete }: TariffCardProps) {
  const { t } = useTranslation();

  const allSquads = [
    ...tariff.internal_squad_uuids,
    ...tariff.external_squad_uuids,
  ];

  return (
    <div className="rounded-xl border border-border bg-card p-5">
      {/* Header: Name + Status */}
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-[15px] font-semibold text-foreground">
          {tariff.name}
        </h3>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 font-mono text-[11px] font-medium",
            tariff.is_active
              ? "bg-green-500/10 text-green-500"
              : "bg-red-500/10 text-red-500",
          )}
        >
          {tariff.is_active
            ? t("admin.tariffs.active")
            : t("admin.tariffs.inactive")}
        </span>
      </div>

      {/* Metrics line */}
      <p className="mt-1 font-mono text-[12px] text-muted-foreground">
        {formatLimit(tariff.traffic_limit_gb, "GB", t)}
        {" \u00B7 "}
        {tariff.duration_days} {t("admin.tariffs.days")}
        {" \u00B7 "}
        {formatLimit(tariff.device_limit, t("admin.tariffs.devices"), t)}
      </p>

      {/* Description */}
      {tariff.description && (
        <p className="mt-2 text-[12px] text-muted-foreground">
          {tariff.description}
        </p>
      )}

      {/* Price */}
      <p className="mt-3 font-mono text-xl font-bold text-primary">
        {formatMoney(tariff.price_amount, tariff.price_currency)}
      </p>

      {/* Features */}
      {tariff.features.length > 0 && (
        <div className="mt-3">
          <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {t("admin.tariffs.features")}
          </p>
          <ul className="space-y-0.5">
            {tariff.features.map((feature) => (
              <li
                key={feature}
                className="flex items-start gap-1.5 text-[12px] text-muted-foreground"
              >
                <span className="mt-1.5 inline-block h-1 w-1 shrink-0 rounded-full bg-primary/60" />
                {feature}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Squads */}
      {allSquads.length > 0 && (
        <div className="mt-3">
          <p className="mb-1 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {t("admin.tariffs.squads")}
          </p>
          <p className="font-mono text-[11px] text-muted-foreground">
            {allSquads.map((uuid) => truncateUuid(uuid)).join(", ")}
          </p>
        </div>
      )}

      {/* Max purchases */}
      {tariff.max_purchases_per_user > 0 && (
        <p className="mt-2 font-mono text-[11px] text-muted-foreground">
          {t("admin.tariffs.maxPurchasesPerUser")}:{" "}
          {tariff.max_purchases_per_user} {t("admin.tariffs.perUser")}
        </p>
      )}

      {/* Actions */}
      <div className="mt-4 flex items-center justify-end gap-2">
        <button
          type="button"
          onClick={() => onEdit(tariff)}
          className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/10"
        >
          <Pencil size={13} />
          {t("common.edit")}
        </button>
        <button
          type="button"
          onClick={() => onDelete(tariff)}
          className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[12px] font-medium text-destructive transition-colors hover:bg-destructive/10"
        >
          <Trash2 size={13} />
          {t("common.delete")}
        </button>
      </div>
    </div>
  );
}

// ─── TariffsPage ────────────────────────────────────────────────────────────

type FormMode =
  | { kind: "closed" }
  | { kind: "create" }
  | { kind: "edit"; tariff: Tariff };

export function TariffsPage() {
  const { t } = useTranslation();
  const { data: tariffs, isLoading, isError } = useAdminTariffs();
  const createTariff = useCreateTariff();
  const updateTariff = useUpdateTariff();
  const deleteTariff = useDeleteTariff();

  const [formMode, setFormMode] = useState<FormMode>({ kind: "closed" });
  const [deleteTarget, setDeleteTarget] = useState<Tariff | null>(null);

  const handleCreate = (values: TariffFormValues) => {
    createTariff.mutate(
      {
        name: values.name,
        description: values.description,
        price_amount: values.price_amount,
        price_currency: values.price_currency,
        duration_days: values.duration_days,
        traffic_limit_gb: values.traffic_limit_gb,
        device_limit: values.device_limit,
        max_purchases_per_user: values.max_purchases_per_user,
        internal_squad_uuids: parseCommaSeparated(values.internal_squad_uuids),
        external_squad_uuids: parseCommaSeparated(values.external_squad_uuids),
        features: parseLinesSeparated(values.features),
        is_active: values.is_active,
        sort_order: values.sort_order,
      },
      {
        onSuccess: () => setFormMode({ kind: "closed" }),
      },
    );
  };

  const handleUpdate = (values: TariffFormValues) => {
    if (formMode.kind !== "edit") return;

    updateTariff.mutate(
      {
        id: formMode.tariff.id,
        data: {
          name: values.name,
          description: values.description,
          price_amount: values.price_amount,
          price_currency: values.price_currency,
          duration_days: values.duration_days,
          traffic_limit_gb: values.traffic_limit_gb,
          device_limit: values.device_limit,
          max_purchases_per_user: values.max_purchases_per_user,
          internal_squad_uuids: parseCommaSeparated(values.internal_squad_uuids),
          external_squad_uuids: parseCommaSeparated(values.external_squad_uuids),
          features: parseLinesSeparated(values.features),
          is_active: values.is_active,
          sort_order: values.sort_order,
        },
      },
      {
        onSuccess: () => setFormMode({ kind: "closed" }),
      },
    );
  };

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    deleteTariff.mutate(deleteTarget.id, {
      onSuccess: () => setDeleteTarget(null),
    });
  };

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-destructive/30 p-12">
        <p className="text-[13px] text-destructive">
          {t("common.errorLoading")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold text-foreground">
          {t("admin.tariffs.title")}
        </h1>
        {formMode.kind === "closed" && (
          <button
            type="button"
            onClick={() => setFormMode({ kind: "create" })}
            className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
          >
            <Plus size={14} />
            {t("admin.tariffs.create")}
          </button>
        )}
      </div>

      {/* Create / Edit form */}
      {formMode.kind === "create" && (
        <TariffForm
          title={t("admin.tariffs.create")}
          onSubmit={handleCreate}
          onCancel={() => setFormMode({ kind: "closed" })}
          isPending={createTariff.isPending}
        />
      )}

      {formMode.kind === "edit" && (
        <TariffForm
          key={formMode.tariff.id}
          title={t("admin.tariffs.edit")}
          initialValues={tariffToFormValues(formMode.tariff)}
          onSubmit={handleUpdate}
          onCancel={() => setFormMode({ kind: "closed" })}
          isPending={updateTariff.isPending}
        />
      )}

      {/* Delete confirmation */}
      {deleteTarget && (
        <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-5">
          <p className="text-[13px] text-foreground">
            {t("admin.tariffs.deleteConfirm")}
          </p>
          <p className="mt-1 font-mono text-[12px] text-muted-foreground">
            {deleteTarget.name}
          </p>
          <div className="mt-4 flex items-center gap-3">
            <button
              type="button"
              onClick={handleDeleteConfirm}
              disabled={deleteTariff.isPending}
              className="flex items-center gap-2 rounded-lg bg-destructive px-4 py-2 text-[12px] font-medium text-destructive-foreground transition-colors hover:bg-destructive/90 disabled:opacity-50"
            >
              {deleteTariff.isPending && (
                <Loader2 size={14} className="animate-spin" />
              )}
              {t("common.delete")}
            </button>
            <button
              type="button"
              onClick={() => setDeleteTarget(null)}
              className="rounded-lg border border-border bg-card px-4 py-2 text-[12px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >
              {t("common.cancel")}
            </button>
          </div>
        </div>
      )}

      {/* Tariff cards grid */}
      {tariffs && tariffs.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {tariffs.map((tariff) => (
            <TariffCard
              key={tariff.id}
              tariff={tariff}
              onEdit={(t) => setFormMode({ kind: "edit", tariff: t })}
              onDelete={setDeleteTarget}
            />
          ))}
        </div>
      ) : (
        formMode.kind === "closed" && (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
            <p className="mb-4 text-[13px] text-muted-foreground">
              {t("admin.tariffs.noTariffs")}
            </p>
            <button
              type="button"
              onClick={() => setFormMode({ kind: "create" })}
              className="flex items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-1.5 text-[12px] font-medium text-primary transition-colors hover:bg-primary/20"
            >
              <Plus size={14} />
              {t("admin.tariffs.create")}
            </button>
          </div>
        )
      )}
    </div>
  );
}
