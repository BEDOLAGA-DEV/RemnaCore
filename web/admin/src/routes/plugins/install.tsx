import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft, Loader2 } from "lucide-react";
import { useInstallPlugin } from "@remnacore/shared";
import { PageHeader, Panel, PanelHeader, TermButton } from "@/components/ui";

const installSchema = z.object({
  manifest: z.string().min(1),
  wasm: z.string().min(1),
});

type InstallFormValues = z.infer<typeof installSchema>;

const textareaCls =
  "w-full resize-none border bg-input px-3 py-3 font-mono text-[12px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45";

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
    <div className="space-y-3.5">
      <PageHeader
        title={t("admin.plugins.install").toUpperCase()}
        breadcrumb="REMNAWAVE PROVIDER / EXTENSIONS / INSTALL"
        right={
          <Link to="/plugins">
            <TermButton type="button" variant="ghost">
              <ArrowLeft size={14} />
              {t("common.back")}
            </TermButton>
          </Link>
        }
      />

      <Panel className="mx-auto max-w-2xl">
        <PanelHeader title="CREATE ITEM" />
        <form
          onSubmit={handleSubmit(onSubmit)}
          className="space-y-4 px-4 py-5"
        >
          <div>
            <label
              htmlFor="manifest"
              className="mb-2 block text-[9px] uppercase tracking-[1.5px] text-t6"
            >
              {t("admin.plugins.manifest")}{" "}
              <span className="text-accent">*</span>
            </label>
            <textarea
              id="manifest"
              rows={8}
              placeholder='[plugin]&#10;slug = "my-plugin"&#10;name = "My Plugin"&#10;version = "1.0.0"'
              {...register("manifest")}
              className={textareaCls}
              style={{
                borderColor: errors.manifest
                  ? "var(--danger)"
                  : "var(--line-strong)",
              }}
            />
            {errors.manifest && (
              <p className="mt-1 text-[9px] uppercase tracking-[1px] text-danger">
                {errors.manifest.message}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="wasm"
              className="mb-2 block text-[9px] uppercase tracking-[1.5px] text-t6"
            >
              {t("admin.plugins.wasmFile")} (BASE64){" "}
              <span className="text-accent">*</span>
            </label>
            <textarea
              id="wasm"
              rows={4}
              placeholder="base64-encoded WASM bytes..."
              {...register("wasm")}
              className={textareaCls}
              style={{
                borderColor: errors.wasm
                  ? "var(--danger)"
                  : "var(--line-strong)",
              }}
            />
            {errors.wasm && (
              <p className="mt-1 text-[9px] uppercase tracking-[1px] text-danger">
                {errors.wasm.message}
              </p>
            )}
          </div>

          <div className="flex items-center gap-2.5 pt-1">
            <TermButton
              type="submit"
              variant="primary"
              disabled={installMutation.isPending}
            >
              {installMutation.isPending ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  {t("common.loading")}
                </>
              ) : (
                t("admin.plugins.install")
              )}
            </TermButton>
            <Link to="/plugins">
              <TermButton type="button" variant="ghost">
                {t("common.cancel")}
              </TermButton>
            </Link>
          </div>

          {installMutation.isError && (
            <p className="text-[9px] uppercase tracking-[1px] text-danger">
              {t("common.error")}
            </p>
          )}
        </form>
      </Panel>
    </div>
  );
}
