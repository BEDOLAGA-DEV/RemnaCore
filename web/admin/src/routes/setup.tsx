import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { passwordSchema, useSetupAdmin } from "@remnacore/shared";
import { markSetupComplete } from "@/lib/setupGate";

// Reuses the shared password policy (mirrors the Go backend) so client and
// server validation never drift.
const setupSchema = z
  .object({
    email: z.string().email(),
    password: passwordSchema,
    confirm: z.string(),
  })
  .refine((d) => d.password === d.confirm, {
    message: "setup.passwordMismatch",
    path: ["confirm"],
  });

type SetupFormValues = z.infer<typeof setupSchema>;

const FIELD_CLASS =
  "w-full rounded-lg border border-border bg-card px-3 py-2.5 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-1 focus:ring-primary/50 focus:border-primary/50 transition-colors";
const LABEL_CLASS =
  "mb-1.5 block text-[12px] font-medium text-muted-foreground";
const ERROR_CLASS = "mt-1 text-[12px] text-red-500 font-mono";

export function AdminSetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setupMutation = useSetupAdmin();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SetupFormValues>({ resolver: zodResolver(setupSchema) });

  const onSubmit = (data: SetupFormValues) => {
    setupMutation.mutate(
      { email: data.email, password: data.password },
      {
        onSuccess: () => {
          markSetupComplete();
          navigate({ to: "/" });
        },
        onError: (err: unknown) => {
          // 409 = another operator finished setup first; send them to login.
          const status = (err as { response?: { status?: number } })?.response
            ?.status;
          if (status === 409) {
            markSetupComplete();
            navigate({ to: "/login" });
          }
        },
      },
    );
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-background px-4">
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
                  id="setup-logo-grad"
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
                fill="url(#setup-logo-grad)"
              />
            </svg>
          </div>
          <h1 className="mt-4 text-[20px] font-bold text-foreground tracking-tight">
            {t("setup.title")}
          </h1>
          <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
            {t("setup.subtitle")}
          </p>
          <p className="mt-3 text-[12px] text-muted-foreground/80 leading-relaxed">
            {t("setup.description")}
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label htmlFor="email" className={LABEL_CLASS}>
              {t("setup.emailLabel")}
            </label>
            <input
              id="email"
              type="email"
              autoComplete="username"
              placeholder={t("auth.emailPlaceholder")}
              {...register("email")}
              className={FIELD_CLASS}
            />
            {errors.email && (
              <p className={ERROR_CLASS}>{t(errors.email.message ?? "")}</p>
            )}
          </div>

          <div>
            <label htmlFor="password" className={LABEL_CLASS}>
              {t("setup.passwordLabel")}
            </label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              placeholder={t("auth.passwordPlaceholder")}
              {...register("password")}
              className={FIELD_CLASS}
            />
            {errors.password && (
              <p className={ERROR_CLASS}>{t(errors.password.message ?? "")}</p>
            )}
          </div>

          <div>
            <label htmlFor="confirm" className={LABEL_CLASS}>
              {t("setup.confirmLabel")}
            </label>
            <input
              id="confirm"
              type="password"
              autoComplete="new-password"
              {...register("confirm")}
              className={FIELD_CLASS}
            />
            {errors.confirm && (
              <p className={ERROR_CLASS}>{t(errors.confirm.message ?? "")}</p>
            )}
          </div>

          <button
            type="submit"
            disabled={setupMutation.isPending}
            className="w-full rounded-lg bg-primary py-2.5 text-[13px] font-semibold text-background hover:bg-primary/90 transition-colors disabled:opacity-40"
          >
            {setupMutation.isPending ? (
              <span className="flex items-center justify-center gap-2">
                <Loader2 size={16} className="animate-spin" />
                {t("setup.creating")}
              </span>
            ) : (
              t("setup.submit")
            )}
          </button>

          {setupMutation.isError && (
            <p className="text-[12px] text-red-500 font-mono text-center">
              {t("setup.failed")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
