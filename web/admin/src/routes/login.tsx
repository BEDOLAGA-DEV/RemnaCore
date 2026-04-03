import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Shield } from "lucide-react";
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
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <Shield size={48} className="mx-auto text-primary" />
          <h1 className="mt-4 text-2xl font-bold text-foreground">
            {t("admin.title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("auth.loginTitle")}
          </p>
        </div>

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
              className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
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
              className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {errors.password && (
              <p className="mt-1 text-sm text-destructive">
                {errors.password.message}
              </p>
            )}
          </div>

          <button
            type="submit"
            disabled={loginMutation.isPending}
            className="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
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

          {accessDenied && (
            <p className="text-sm text-destructive text-center">
              {t("auth.accessDeniedAdmin")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
