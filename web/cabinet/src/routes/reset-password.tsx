import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Shield } from "lucide-react";
import { useResetPassword, passwordSchema } from "@remnacore/shared";

const resetSchema = z.object({
  token: z.string().min(1),
  new_password: passwordSchema,
});

type ResetFormValues = z.infer<typeof resetSchema>;

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const mutation = useResetPassword();
  const [success, setSuccess] = useState(false);

  // Extract token from URL query parameter
  const urlParams = new URLSearchParams(window.location.search);
  const tokenFromUrl = urlParams.get("token") ?? "";

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ResetFormValues>({
    resolver: zodResolver(resetSchema),
    defaultValues: { token: tokenFromUrl },
  });

  const onSubmit = (data: ResetFormValues) => {
    mutation.mutate(
      { token: data.token, new_password: data.new_password },
      {
        onSuccess: () => setSuccess(true),
      },
    );
  };

  if (success) {
    return (
      <div className="flex min-h-screen items-center justify-center p-6 ambient-bg">
        <div className="w-full max-w-[380px] animate-fade-up">
          <div className="mb-8 flex flex-col items-center gap-3">
            <div
              className="flex h-10 w-10 items-center justify-center rounded-xl"
              style={{ background: "linear-gradient(135deg, #2dd4bf, #0d9488)" }}
            >
              <Shield size={20} className="text-[#110f0d]" />
            </div>
            <h1 className="text-xl font-bold text-foreground">
              {t("auth.resetSuccess")}
            </h1>
            <p className="text-sm text-muted-foreground text-center">
              {t("auth.resetSuccessDescription", "Your password has been updated")}
            </p>
          </div>

          <div className="rounded-[var(--radius)] border border-border bg-card p-6 text-center">
            <Link
              to="/login"
              className="inline-block w-full rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground transition-all hover:brightness-110"
            >
              {t("auth.signIn")}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-6 ambient-bg">
      <div className="w-full max-w-[380px] animate-fade-up">
        {/* Brand */}
        <div className="mb-8 flex flex-col items-center gap-3">
          <div
            className="flex h-10 w-10 items-center justify-center rounded-xl"
            style={{ background: "linear-gradient(135deg, #2dd4bf, #0d9488)" }}
          >
            <Shield size={20} className="text-[#110f0d]" />
          </div>
          <h1 className="text-xl font-bold text-foreground">
            {t("auth.resetPasswordTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("auth.resetPasswordSubtitle", "Choose a new password")}
          </p>
        </div>

        {/* Form card */}
        <div className="rounded-[var(--radius)] border border-border bg-card p-6 space-y-4">
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <input type="hidden" {...register("token")} />

            <div>
              <label
                htmlFor="new_password"
                className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-muted-foreground"
              >
                {t("auth.newPassword")}
              </label>
              <input
                id="new_password"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("new_password")}
                className="w-full rounded-xl border border-border bg-[rgba(255,235,210,0.03)] px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50"
              />
              {errors.new_password && (
                <p className="mt-1 text-sm text-destructive">
                  {t(errors.new_password.message ?? "common.error")}
                </p>
              )}
            </div>

            <button
              type="submit"
              disabled={mutation.isPending}
              className="w-full rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground transition-all hover:brightness-110 disabled:opacity-50"
            >
              {mutation.isPending ? (
                <Loader2 size={16} className="mx-auto animate-spin" />
              ) : (
                t("common.submit")
              )}
            </button>

            {mutation.isError && (
              <p className="text-sm text-destructive text-center">
                {t("common.error")}
              </p>
            )}
          </form>
        </div>

        {/* Link below card */}
        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.rememberPassword", "Remember your password?")}{" "}
          <Link to="/login" className="text-primary hover:underline">
            {t("auth.signIn")}
          </Link>
        </p>
      </div>
    </div>
  );
}
