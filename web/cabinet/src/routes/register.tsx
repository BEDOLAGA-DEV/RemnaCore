import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Shield, Mail } from "lucide-react";
import { useRegister, passwordSchema } from "@remnacore/shared";

const registerSchema = z
  .object({
    email: z.string().email(),
    password: passwordSchema,
    confirmPassword: z.string().min(1),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
  });

type RegisterFormValues = z.infer<typeof registerSchema>;

export function RegisterPage() {
  const { t } = useTranslation();
  const registerMutation = useRegister();
  const [registered, setRegistered] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
  });

  const onSubmit = (data: RegisterFormValues) => {
    registerMutation.mutate(
      { email: data.email, password: data.password },
      {
        onSuccess: () => setRegistered(true),
      },
    );
  };

  if (registered) {
    return (
      <div className="flex min-h-screen items-center justify-center p-6 ambient-bg">
        <div className="w-full max-w-[380px] animate-fade-up">
          <div className="mb-8 flex flex-col items-center gap-3">
            <Mail size={48} className="text-primary" />
            <h1 className="text-xl font-bold text-foreground">
              {t("auth.verifyEmail")}
            </h1>
            <p className="text-sm text-muted-foreground text-center">
              {t("auth.verifyEmailDescription")}
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
            {t("auth.registerTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("auth.createAccountSubtitle", "Create your account")}
          </p>
        </div>

        {/* Form card */}
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

            <div>
              <label
                htmlFor="password"
                className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-muted-foreground"
              >
                {t("common.password")}
              </label>
              <input
                id="password"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("password")}
                className="w-full rounded-xl border border-border bg-[rgba(255,235,210,0.03)] px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50"
              />
              {errors.password && (
                <p className="mt-1 text-sm text-destructive">
                  {t(errors.password.message ?? "common.error")}
                </p>
              )}
            </div>

            <div>
              <label
                htmlFor="confirmPassword"
                className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-muted-foreground"
              >
                {t("auth.confirmPassword")}
              </label>
              <input
                id="confirmPassword"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("confirmPassword")}
                className="w-full rounded-xl border border-border bg-[rgba(255,235,210,0.03)] px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50"
              />
              {errors.confirmPassword && (
                <p className="mt-1 text-sm text-destructive">
                  {errors.confirmPassword.message}
                </p>
              )}
            </div>

            <button
              type="submit"
              disabled={isSubmitting || registerMutation.isPending}
              className="w-full rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground transition-all hover:brightness-110 disabled:opacity-50"
            >
              {registerMutation.isPending ? (
                <span className="flex items-center justify-center gap-2">
                  <Loader2 size={16} className="animate-spin" />
                  {t("common.loading")}
                </span>
              ) : (
                t("auth.signUp")
              )}
            </button>

            {registerMutation.isError && (
              <p className="text-sm text-destructive text-center">
                {t("auth.registrationFailed")}
              </p>
            )}
          </form>
        </div>

        {/* Links below card */}
        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.hasAccount")}{" "}
          <Link to="/login" className="text-primary hover:underline">
            {t("auth.signIn")}
          </Link>
        </p>
      </div>
    </div>
  );
}
