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
      <div className="flex min-h-screen items-center justify-center px-4">
        <div className="w-full max-w-md space-y-6 text-center">
          <h1 className="text-2xl font-bold text-foreground">
            {t("auth.resetSuccess")}
          </h1>
          <Link
            to="/login"
            className="inline-block rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
          >
            {t("auth.signIn")}
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-md space-y-6">
        <h1 className="text-2xl font-bold text-foreground">
          {t("auth.resetPasswordTitle")}
        </h1>

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
              className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
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
            className="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
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
  );
}
