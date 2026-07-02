import { zodResolver } from "@hookform/resolvers/zod";
import { emailSchema, useForgotPassword } from "@remnacore/shared";
import { Link } from "@tanstack/react-router";
import { ArrowLeft, Loader2, Shield } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

const forgotSchema = z.object({
  email: emailSchema,
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
    <div className="flex min-h-screen items-center justify-center p-6 ambient-bg">
      <div className="w-full max-w-[380px] animate-fade-up">
        {/* Back link */}
        <Link
          to="/login"
          className="mb-6 inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft size={14} />
          {t("common.back")}
        </Link>

        {/* Brand */}
        <div className="mb-8 flex flex-col items-center gap-3">
          <div
            className="flex h-10 w-10 items-center justify-center rounded-xl"
            style={{ background: "linear-gradient(135deg, #2dd4bf, #0d9488)" }}
          >
            <Shield size={20} className="text-[#110f0d]" />
          </div>
          <h1 className="text-xl font-bold text-foreground">
            {t("auth.forgotPasswordTitle")}
          </h1>
          <p className="text-sm text-muted-foreground text-center">
            {t("auth.forgotPasswordDescription")}
          </p>
        </div>

        {/* Form card */}
        {sent ? (
          <div className="rounded-[var(--radius)] border border-border bg-card p-6 text-center">
            <p className="text-sm text-foreground">{t("auth.resetSent")}</p>
          </div>
        ) : (
          <div className="rounded-[var(--radius)] border border-border bg-card p-6 space-y-4">
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
              <div>
                <label
                  htmlFor="email"
                  className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-muted-foreground"
                >
                  {t("common.email")}
                </label>
                <input
                  id="email"
                  type="email"
                  placeholder={t("auth.emailPlaceholder")}
                  {...register("email")}
                  className="w-full rounded-xl border border-border bg-[rgba(255,235,210,0.03)] px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50"
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
        )}

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
