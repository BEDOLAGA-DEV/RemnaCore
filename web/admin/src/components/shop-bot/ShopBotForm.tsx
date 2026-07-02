import { zodResolver } from "@hookform/resolvers/zod";
import {
  type BotPluginInfo,
  httpsURLSchema,
  type ShopBotConfig,
  type UpdateShopBotInput,
} from "@remnacore/shared";
import { Check, Loader2 } from "lucide-react";
import { useEffect } from "react";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Segmented } from "@/components/ui/Segmented";
import { TermButton } from "@/components/ui/TermButton";
import { TermInput } from "@/components/ui/TermInput";

const botSchema = z.object({
  bot_token: z.string(),
  cabinet_url: httpsURLSchema.or(z.literal("")),
  bot_plugin: z.string(),
  enabled: z.boolean(),
});

type BotFormValues = z.infer<typeof botSchema>;

export type ShopBotFormProps = {
  config: ShopBotConfig | undefined;
  plugins: BotPluginInfo[] | undefined;
  onSubmit: (data: UpdateShopBotInput) => void;
  isPending: boolean;
  isSuccess: boolean;
};

export function ShopBotForm({
  config,
  plugins,
  onSubmit,
  isPending,
  isSuccess,
}: ShopBotFormProps) {
  const { t } = useTranslation();

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors },
  } = useForm<BotFormValues>({
    resolver: zodResolver(botSchema),
    values: {
      bot_token: "",
      cabinet_url: config?.cabinet_url ?? "",
      bot_plugin: config?.bot_plugin ?? "",
      enabled: config?.enabled ?? false,
    },
  });

  // Reset bot_token to empty whenever config changes (never prefill the token)
  useEffect(() => {
    reset({
      bot_token: "",
      cabinet_url: config?.cabinet_url ?? "",
      bot_plugin: config?.bot_plugin ?? "",
      enabled: config?.enabled ?? false,
    });
  }, [config, reset]);

  const handleFormSubmit = (values: BotFormValues) => {
    onSubmit({
      bot_token: values.bot_token,
      cabinet_url: values.cabinet_url,
      bot_plugin: values.bot_plugin,
      enabled: values.enabled,
    });
  };

  const selectClassName =
    "w-full border bg-input px-3 py-3 text-[13px] tracking-[.5px] text-t1 outline-none focus:border-accent/45";

  return (
    <form onSubmit={handleSubmit(handleFormSubmit)} className="space-y-4 p-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <TermInput
          id="bot_token"
          type="password"
          label={t("admin.tenantBot.token")}
          placeholder={
            config?.has_token
              ? t("admin.tenantBot.tokenUnchanged")
              : t("admin.tenantBot.tokenPlaceholder")
          }
          error={errors.bot_token?.message}
          {...register("bot_token")}
        />

        <TermInput
          id="cabinet_url"
          label={t("admin.tenantBot.cabinetUrl")}
          placeholder="https://"
          error={errors.cabinet_url?.message}
          {...register("cabinet_url")}
        />

        <div>
          <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
            {t("admin.tenantBot.plugin")}
          </div>
          <select
            {...register("bot_plugin")}
            className={selectClassName}
            style={{ borderColor: "var(--line-strong)" }}
          >
            <option value="">{t("admin.tenantBot.defaultPlugin")}</option>
            {plugins?.map((p) => (
              <option key={p.slug} value={p.slug}>
                {p.name} ({p.kind})
              </option>
            ))}
          </select>
        </div>

        <div>
          <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
            {t("common.status")}
          </div>
          <Controller
            name="enabled"
            control={control}
            render={({ field }) => (
              <Segmented<"true" | "false">
                value={field.value ? "true" : "false"}
                options={[
                  { value: "true", label: t("admin.tenantBot.enabled") },
                  { value: "false", label: t("admin.tenantBot.disabled") },
                ]}
                onChange={(v) => field.onChange(v === "true")}
              />
            )}
          />
        </div>
      </div>

      {config?.has_token && (
        <p className="text-[10px] uppercase tracking-[1px] text-t5">
          {t("admin.tenantBot.tokenHint")}
        </p>
      )}

      <TermButton type="submit" variant="primary" disabled={isPending}>
        {isPending ? (
          <Loader2 size={14} className="animate-spin" />
        ) : (
          t("common.save")
        )}
      </TermButton>

      {isSuccess && (
        <p className="flex items-center gap-1.5 text-[10px] uppercase tracking-[1px] text-accent">
          <Check size={14} />
          {t("admin.tenantBot.saveSuccess")}
        </p>
      )}
    </form>
  );
}
