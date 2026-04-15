import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
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
      <div className="flex min-h-screen items-center justify-center px-4 ambient-bg">
        <div className="w-full max-w-md space-y-6 text-center relative z-[1] animate-fade-up">
          <h1 className="text-xl font-bold text-foreground tracking-tight">
            {t("auth.resetSuccess")}
          </h1>
          <Link
            to="/login"
            className="inline-block rounded-[10px] bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground hover:brightness-110 transition-all"
          >
            {t("auth.signIn")}
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4 ambient-bg">
      <div className="w-full max-w-md space-y-6 relative z-[1] animate-fade-up">
        <div className="flex flex-col items-center gap-3 mb-2">
          <div className="w-[34px] h-[34px] rounded-[10px] bg-gradient-to-br from-primary to-[#5eead4] flex items-center justify-center">
            <svg width="18" height="18" viewBox="0 0 16 16" fill="none">
              <path d="M8 1L2 4.5V8.5C2 12 4.5 14.5 8 15.5C11.5 14.5 14 12 14 8.5V4.5L8 1Z" fill="var(--primary-foreground)" />
            </svg>
          </div>
          <span className="text-lg font-bold tracking-tight text-foreground">Remna</span>
        </div>

        <div className="text-center">
          <h1 className="text-xl font-bold text-foreground tracking-tight">
            {t("auth.resetPasswordTitle")}
          </h1>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <input type="hidden" {...register("token")} />

            <div>
              <label
                htmlFor="new_password"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                {t("auth.newPassword")}
              </label>
              <input
                id="new_password"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("new_password")}
                className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
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
              className="w-full rounded-[10px] bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground hover:brightness-110 transition-all disabled:opacity-50"
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
      </div>
    </div>
  );
}
