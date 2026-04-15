import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, ArrowLeft } from "lucide-react";
import { useForgotPassword } from "@remnacore/shared";

const forgotSchema = z.object({
  email: z.string().email(),
});

type ForgotFormValues = z.infer<typeof forgotSchema>;

export function ForgotPasswordPage() {
  const { t } = useTranslation();
  const mutation = useForgotPassword();
  const [sent, setSent] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ForgotFormValues>({
    resolver: zodResolver(forgotSchema),
  });

  const onSubmit = (data: ForgotFormValues) => {
    mutation.mutate(data, {
      onSuccess: () => setSent(true),
    });
  };

  return (
    <div className="flex min-h-screen items-center justify-center px-4 ambient-bg">
      <div className="w-full max-w-md space-y-6 relative z-[1] animate-fade-up">
        <Link
          to="/login"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft size={14} />
          {t("common.back")}
        </Link>

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
            {t("auth.forgotPasswordTitle")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("auth.forgotPasswordDescription")}
          </p>
        </div>

        {sent ? (
          <div className="bg-card border border-border rounded-lg p-4 text-sm text-foreground">
            {t("auth.resetSent")}
          </div>
        ) : (
          <div className="bg-card border border-border rounded-lg p-6">
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
              <div>
                <label
                  htmlFor="email"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  {t("common.email")}
                </label>
                <input
                  id="email"
                  type="email"
                  placeholder={t("auth.emailPlaceholder")}
                  {...register("email")}
                  className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
                />
                {errors.email && (
                  <p className="mt-1 text-sm text-destructive">
                    {errors.email.message}
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
        )}
      </div>
    </div>
  );
}
