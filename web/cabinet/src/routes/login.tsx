import { zodResolver } from "@hookform/resolvers/zod";
import { emailSchema, useLogin } from "@remnacore/shared";
import { Link, useNavigate } from "@tanstack/react-router";
import { Loader2, Shield } from "lucide-react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

const loginSchema = z.object({
  email: emailSchema,
  password: z.string().min(1),
});

type LoginFormValues = z.infer<typeof loginSchema>;

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const loginMutation = useLogin();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = (data: LoginFormValues) => {
    loginMutation.mutate(
      { email: data.email, password: data.password },
      {
        onSuccess: () => {
          navigate({ to: "/" });
        },
      },
    );
  };

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
            {t("auth.loginTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("auth.welcomeBack", "Welcome back")}
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
                  {errors.password.message}
                </p>
              )}
            </div>

            <div className="flex justify-end">
              <Link
                to="/forgot-password"
                className="text-xs text-primary hover:underline"
              >
                {t("auth.forgotPassword")}
              </Link>
            </div>

            <button
              type="submit"
              disabled={isSubmitting || loginMutation.isPending}
              className="w-full rounded-xl bg-primary py-2.5 text-sm font-semibold text-primary-foreground transition-all hover:brightness-110 disabled:opacity-50"
            >
              {loginMutation.isPending ? (
                <span className="flex items-center justify-center gap-2">
                  <Loader2 size={16} className="animate-spin" />
                  {t("common.loading")}
                </span>
              ) : (
                t("auth.signIn")
              )}
            </button>

            {loginMutation.isError && (
              <p className="text-sm text-destructive text-center">
                {t("auth.invalidCredentials")}
              </p>
            )}
          </form>
        </div>

        {/* Links below card */}
        <p className="mt-4 text-center text-sm text-muted-foreground">
          {t("auth.noAccount")}{" "}
          <Link to="/register" className="text-primary hover:underline">
            {t("auth.signUp")}
          </Link>
        </p>
      </div>
    </div>
  );
}
