import { zodResolver } from "@hookform/resolvers/zod";
import {
  emailSchema,
  USER_ROLES,
  useAuthStore,
  useLogin,
} from "@remnacore/shared";
import { useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { GridBackdrop, ScanLine, TermButton, TermInput } from "@/components/ui";

const loginSchema = z.object({
  email: emailSchema,
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
          if (result.user.role === USER_ROLES.admin) {
            navigate({ to: "/" });
            return;
          }
          if (result.user.role === USER_ROLES.reseller) {
            navigate({ to: "/reseller/bot" });
            return;
          }
          logout();
          setAccessDenied(true);
        },
      },
    );
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-bg px-4">
      <GridBackdrop />
      <ScanLine />

      <div className="relative w-full max-w-sm space-y-10">
        {/* Brand */}
        <div className="flex flex-col items-center text-center">
          <div className="flex items-center gap-3">
            <div
              className="relative h-[26px] w-[26px] rotate-45 border border-accent"
              style={{
                boxShadow:
                  "0 0 18px color-mix(in srgb,var(--accent) 55%,transparent)",
              }}
              aria-hidden
            >
              <div className="absolute inset-[6px] bg-accent" />
            </div>
            <div className="text-left">
              <div className="text-[16px] font-extrabold tracking-[1.5px] text-t1">
                REMNA<span className="text-accent">CORE</span>
              </div>
              <div className="mt-[3px] text-[9px] tracking-[3px] text-t7">
                NERVE CENTER
              </div>
            </div>
          </div>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
          <TermInput
            id="email"
            type="email"
            label={t("common.email")}
            placeholder={t("auth.emailPlaceholder")}
            error={errors.email ? t("common.email") : undefined}
            {...register("email")}
          />

          <TermInput
            id="password"
            type="password"
            label={t("common.password")}
            placeholder={t("auth.passwordPlaceholder")}
            error={errors.password ? t("common.password") : undefined}
            {...register("password")}
          />

          <TermButton
            type="submit"
            variant="primary"
            disabled={loginMutation.isPending}
            className="w-full py-3 tracking-[2px]"
          >
            {loginMutation.isPending ? (
              <>
                <Loader2 size={14} className="animate-spin" />
                {t("common.loading")}
              </>
            ) : (
              `▸ ${t("auth.signIn")}`
            )}
          </TermButton>

          {loginMutation.isError && (
            <p className="text-center text-[10px] uppercase tracking-[1px] text-danger">
              {t("auth.invalidCredentials")}
            </p>
          )}

          {accessDenied && (
            <p className="text-center text-[10px] uppercase tracking-[1px] text-danger">
              {t("auth.accessDeniedAdmin")}
            </p>
          )}
        </form>
      </div>
    </div>
  );
}
