import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { useLogin, useAuthStore, USER_ROLES } from "@remnacore/shared";

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

type LoginFormValues = z.infer<typeof loginSchema>;

export function AdminLoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const loginMutation = useLogin();
  const { logout } = useAuthStore();
  const [accessDenied, setAccessDenied] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = (data: LoginFormValues) => {
    setAccessDenied(false);
    loginMutation.mutate(
      { email: data.email, password: data.password },
      {
        onSuccess: (result) => {
          if (result.user.role !== USER_ROLES.admin) {
            logout();
            setAccessDenied(true);
            return;
          }
          navigate({ to: "/" });
        },
      },
    );
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-background px-4">
      {/* Subtle animated gradient line at top */}
      <div className="absolute inset-x-0 top-0 h-[2px] bg-gradient-to-r from-primary via-amber-500 to-primary animate-pulse" />

      <div className="w-full max-w-sm space-y-8">
        {/* Logo */}
        <div className="text-center">
          <div className="mx-auto flex items-center justify-center">
            <svg
              width="40"
              height="40"
              viewBox="0 0 20 20"
              fill="none"
              aria-hidden="true"
            >
              <defs>
                <linearGradient
                  id="login-logo-grad"
                  x1="0"
                  y1="0"
                  x2="20"
                  y2="20"
                  gradientUnits="userSpaceOnUse"
                >
                  <stop stopColor="hsl(187 86% 53%)" />
                  <stop offset="1" stopColor="hsl(38 92% 50%)" />
                </linearGradient>
              </defs>
              <path
                d="M10 1L18.66 10L10 19L1.34 10L10 1Z"
                fill="url(#login-logo-grad)"
              />
            </svg>
          </div>
          <h1 className="mt-4 text-[20px] font-bold text-foreground tracking-tight">
            RemnaCore
          </h1>
          <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
            admin console
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label
              htmlFor="email"
              className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
            >
              {t("common.email")}
            </label>
            <input
              id="email"
              type="email"
              placeholder={t("auth.emailPlaceholder")}
              {...register("email")}
              className="w-full rounded-lg border border-border bg-card px-3 py-2.5 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
            />
            {errors.email && (
              <p className="mt-1 text-[12px] text-red-500 font-mono">
                {errors.email.message}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="password"
              className="mb-1.5 block text-[12px] font-medium text-muted-foreground"
            >
              {t("common.password")}
            </label>
            <input
              id="password"
              type="password"
              placeholder={t("auth.passwordPlaceholder")}
              {...register("password")}
              className="w-full rounded-lg border border-border bg-card px-3 py-2.5 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors"
            />
            {errors.password && (
              <p className="mt-1 text-[12px] text-red-500 font-mono">
                {errors.password.message}
              </p>
            )}
          </div>

          <button
            type="submit"
            disabled={loginMutation.isPending}
            className="w-full rounded-lg bg-primary py-2.5 text-[13px] font-semibold text-background hover:bg-primary/90 transition-colors disabled:opacity-40"
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
            <p className="text-[12px] text-red-500 font-mono text-center">
              {t("auth.invalidCredentials")}
            </p>
          )}

          {accessDenied && (
            <p className="text-[12px] text-red-500 font-mono text-center">
              {t("auth.accessDeniedAdmin")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
