import { zodResolver } from "@hookform/resolvers/zod";
import { passwordSchema, useSetupAdmin } from "@remnacore/shared";
import { useNavigate } from "@tanstack/react-router";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import {
  Bar,
  GridBackdrop,
  Panel,
  ScanLine,
  TermButton,
  TermInput,
} from "@/components/ui";
import { markSetupComplete } from "@/lib/setupGate";

// Reuses the shared password policy (mirrors the Go backend) so client and
// server validation never drift.
const setupSchema = z
  .object({
    email: z.email(),
    password: passwordSchema,
    confirm: z.string(),
  })
  .refine((d) => d.password === d.confirm, {
    message: "setup.passwordMismatch",
    path: ["confirm"],
  });

type SetupFormValues = z.infer<typeof setupSchema>;

// Strength is derived from the same rules encoded in `passwordSchema`:
// length >= 8, an uppercase letter, a lowercase letter and a digit.
// Four satisfied rules => four lit segments.
function passwordStrength(pw: string): number {
  if (!pw) return 0;
  let score = 0;
  if (pw.length >= 8) score++;
  if (/[A-Z]/.test(pw)) score++;
  if (/[a-z]/.test(pw)) score++;
  if (/[0-9]/.test(pw)) score++;
  return score;
}

// Decorative copy that has no i18n key (matches the mockup terminal chrome).
const STRENGTH_LABELS = ["", "TOO WEAK", "FAIR", "GOOD", "STRONG"];

function strengthColor(score: number): string {
  if (score <= 1) return "var(--danger)";
  if (score === 2) return "var(--warn)";
  return "var(--accent)";
}

const STATUS_LINES = [
  "database connection established",
  "migrations applied",
  "remnawave panel reachable",
] as const;

export function AdminSetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setupMutation = useSetupAdmin();
  const [agree, setAgree] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<SetupFormValues>({ resolver: zodResolver(setupSchema) });

  const password = watch("password") ?? "";
  const score = passwordStrength(password);
  const segColor = strengthColor(score);
  const strengthLabel = score > 0 ? STRENGTH_LABELS[score] : "";

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
    <div className="relative flex min-h-screen items-center justify-center bg-bg px-8 py-8">
      <GridBackdrop />
      <ScanLine />

      <Panel className="relative grid w-[980px] max-w-full grid-cols-1 md:grid-cols-[1fr_1.05fr]">
        {/* Left column — brand + heading + status checklist */}
        <div className="flex flex-col border-b border-line p-10 md:border-b-0 md:border-r">
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
            <div>
              <div className="text-[16px] font-extrabold tracking-[1.5px] text-t1">
                REMNA<span className="text-accent">CORE</span>
              </div>
              <div className="mt-[3px] text-[9px] tracking-[3px] text-t7">
                NERVE CENTER
              </div>
            </div>
          </div>

          <div className="mt-12">
            <div className="mb-3 text-[10px] tracking-[3px] text-accent">
              {t("setup.subtitle")}
            </div>
            <h1 className="text-[30px] font-extrabold leading-[1.15] tracking-[.5px] text-t1">
              {t("setup.title")}
              <span className="text-accent">_</span>
            </h1>
            <p className="mt-4 max-w-[300px] text-[12px] leading-[1.8] text-t5">
              {t("setup.description")}
            </p>
          </div>

          <div className="mt-auto flex flex-col gap-[9px] border-t border-line pt-5 text-[11px]">
            {STATUS_LINES.map((line) => (
              <div key={line} className="flex items-center gap-[10px] text-t4">
                <span className="text-accent">✓</span>
                {line}
              </div>
            ))}
            <div className="flex items-center gap-[10px] text-t2">
              <span className="text-warn">◇</span>
              awaiting super admin
              <span
                className="inline-block h-[13px] w-[7px] bg-accent"
                style={{ animation: "blink 1.1s step-end infinite" }}
                aria-hidden
              />
            </div>
          </div>
        </div>

        {/* Right column — the form */}
        <div className="flex min-h-[560px] flex-col justify-center p-10">
          <div className="mb-7 flex items-center justify-between">
            <div className="text-[10px] tracking-[2px] text-t7">
              STEP 01 / 01 · CREATE ACCOUNT
            </div>
            <div className="flex gap-[6px]">
              <span className="h-[3px] w-6 bg-accent" />
              <span className="h-[3px] w-[10px] bg-line" />
            </div>
          </div>

          <form onSubmit={handleSubmit(onSubmit)} className="space-y-[18px]">
            <TermInput
              id="email"
              type="email"
              autoComplete="username"
              label={t("setup.emailLabel")}
              placeholder={t("auth.emailPlaceholder")}
              error={errors.email ? t(errors.email.message ?? "") : undefined}
              {...register("email")}
            />

            <div>
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[9px] uppercase tracking-[1.5px] text-t6">
                  {t("setup.passwordLabel")}
                </span>
                {strengthLabel && (
                  <span
                    className="text-[9px] uppercase tracking-[1px]"
                    style={{ color: segColor }}
                  >
                    {strengthLabel}
                  </span>
                )}
              </div>
              <input
                id="password"
                type="password"
                autoComplete="new-password"
                placeholder={t("auth.passwordPlaceholder")}
                {...register("password")}
                className="w-full border bg-input px-3 py-3 text-[13px] tracking-[1px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45"
                style={{
                  borderColor: errors.password
                    ? "var(--danger)"
                    : "var(--line-strong)",
                }}
              />
              {/* Strength meter — 4 segments mapped to the 4 passwordSchema rules */}
              <div className="mt-2 flex gap-1">
                {[0, 1, 2, 3].map((i) => (
                  <Bar
                    key={i}
                    pct={100}
                    color={i < score ? segColor : "rgba(255,255,255,.07)"}
                  />
                ))}
              </div>
              {errors.password && (
                <div className="mt-1 text-[9px] uppercase tracking-[1px] text-danger">
                  {t(errors.password.message ?? "")}
                </div>
              )}
            </div>

            <TermInput
              id="confirm"
              type="password"
              autoComplete="new-password"
              label={t("setup.confirmLabel")}
              error={
                errors.confirm ? t(errors.confirm.message ?? "") : undefined
              }
              {...register("confirm")}
            />

            {/* Visual confirmation only — does NOT block submit. */}
            <label className="mt-[2px] flex cursor-pointer items-start gap-[11px]">
              <span
                onClick={() => setAgree((v) => !v)}
                className="mt-[1px] flex h-[18px] w-[18px] flex-shrink-0 items-center justify-center border text-[11px] font-bold"
                style={{
                  borderColor: agree ? "var(--accent)" : "var(--line-strong)",
                  background: agree ? "var(--accent)" : "transparent",
                  color: "var(--on-accent)",
                }}
                aria-hidden
              >
                {agree ? "✓" : ""}
              </span>
              <input
                type="checkbox"
                checked={agree}
                onChange={(e) => setAgree(e.target.checked)}
                className="sr-only"
              />
              <span className="text-[11px] leading-[1.6] text-t4">
                I understand this account has full root access and cannot be
                recovered without the master key.
              </span>
            </label>

            <TermButton
              type="submit"
              variant="primary"
              disabled={setupMutation.isPending}
              className="w-full py-[14px] tracking-[2px]"
            >
              {setupMutation.isPending ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  {t("setup.creating")}
                </>
              ) : (
                `▸ ${t("setup.submit")}`
              )}
            </TermButton>

            {setupMutation.isError && (
              <p className="text-center text-[10px] uppercase tracking-[1px] text-danger">
                {t("setup.failed")}
              </p>
            )}
          </form>

          <div className="mt-7 flex items-center justify-between border-t border-line pt-4 text-[9px] tracking-[1.5px] text-t8">
            <span>REMNACORE v4.2.1</span>
            <span>SECURE · TLS 1.3 · AES-256</span>
          </div>
        </div>
      </Panel>
    </div>
  );
}
