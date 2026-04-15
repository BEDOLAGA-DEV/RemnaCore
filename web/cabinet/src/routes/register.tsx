import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Mail } from "lucide-react";
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
      <div className="flex min-h-screen items-center justify-center px-4 ambient-bg">
        <div className="w-full max-w-md space-y-6 text-center relative z-[1] animate-fade-up">
          <Mail size={48} className="mx-auto text-primary" />
          <h1 className="text-xl font-bold text-foreground tracking-tight">
            {t("auth.verifyEmail")}
          </h1>
          <p className="text-muted-foreground">
            {t("auth.verifyEmailDescription")}
          </p>
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
            {t("auth.registerTitle")}
          </h1>
        </div>

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

            <div>
              <label
                htmlFor="password"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                {t("common.password")}
              </label>
              <input
                id="password"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("password")}
                className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
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
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                {t("auth.confirmPassword")}
              </label>
              <input
                id="confirmPassword"
                type="password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("confirmPassword")}
                className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring transition-colors"
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
              className="w-full rounded-[10px] bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground hover:brightness-110 transition-all disabled:opacity-50"
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

        <p className="text-center text-sm text-muted-foreground">
          {t("auth.hasAccount")}{" "}
          <Link to="/login" className="text-primary hover:underline">
            {t("auth.signIn")}
          </Link>
        </p>
      </div>
    </div>
  );
}
