import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Loader2 } from "lucide-react";
import { useInstallPlugin } from "@remnacore/shared";

const installSchema = z.object({
  manifest: z.string().min(1),
  wasm: z.string().min(1),
});

type InstallFormValues = z.infer<typeof installSchema>;

export function InstallPluginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const installMutation = useInstallPlugin();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<InstallFormValues>({
    resolver: zodResolver(installSchema),
  });

  const onSubmit = (data: InstallFormValues) => {
    installMutation.mutate(data, {
      onSuccess: () => {
        navigate({ to: "/plugins" });
      },
    });
  };

  return (
    <div className="mx-auto max-w-lg space-y-6">
      <Link
        to="/plugins"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <h1 className="text-2xl font-bold text-foreground">
        {t("admin.plugins.install")}
      </h1>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div>
          <label
            htmlFor="manifest"
            className="mb-1.5 block text-sm font-medium text-foreground"
          >
            {t("admin.plugins.manifest")}
          </label>
          <textarea
            id="manifest"
            rows={8}
            placeholder="[plugin]&#10;slug = &quot;my-plugin&quot;&#10;name = &quot;My Plugin&quot;&#10;version = &quot;1.0.0&quot;"
            {...register("manifest")}
            className="w-full rounded-lg border border-input bg-background px-3 py-2 font-mono text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          />
          {errors.manifest && (
            <p className="mt-1 text-sm text-destructive">
              {errors.manifest.message}
            </p>
          )}
        </div>

        <div>
          <label
            htmlFor="wasm"
            className="mb-1.5 block text-sm font-medium text-foreground"
          >
            {t("admin.plugins.wasmFile")} (base64)
          </label>
          <textarea
            id="wasm"
            rows={4}
            placeholder="base64-encoded WASM bytes..."
            {...register("wasm")}
            className="w-full rounded-lg border border-input bg-background px-3 py-2 font-mono text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          />
          {errors.wasm && (
            <p className="mt-1 text-sm text-destructive">
              {errors.wasm.message}
            </p>
          )}
        </div>

        <button
          type="submit"
          disabled={installMutation.isPending}
          className="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
        >
          {installMutation.isPending ? (
            <span className="flex items-center justify-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("common.loading")}
            </span>
          ) : (
            t("admin.plugins.install")
          )}
        </button>

        {installMutation.isError && (
          <p className="text-sm text-destructive text-center">
            {t("common.error")}
          </p>
        )}
      </form>
    </div>
  );
}
