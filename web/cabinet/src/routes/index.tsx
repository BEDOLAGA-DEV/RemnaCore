import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  Package,
  Wifi,
  WifiOff,
  ChevronRight,
  Sparkles,
  ArrowUpRight,
  Monitor,
  Smartphone,
  Tablet,
  Shield,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  useSubscriptions,
  useInvoices,
  useBindings,
  useMe,
  LoadingSpinner,
  formatDate,
  formatBytes,
  formatMoney,
  cn,
} from "@remnacore/shared";
import type { SubscriptionStatus, InvoiceStatus } from "@remnacore/shared";

/* ─── Status helpers ────────────────────────────────────────────────── */

const STATUS_DOT_COLOR: Record<SubscriptionStatus, string> = {
  active: "bg-primary",
  pending: "bg-yellow-500",
  cancelled: "bg-muted-foreground",
  expired: "bg-muted-foreground",
  paused: "bg-warm",
};

const STATUS_TEXT_COLOR: Record<SubscriptionStatus, string> = {
  active: "text-primary",
  pending: "text-yellow-500",
  cancelled: "text-muted-foreground",
  expired: "text-muted-foreground",
  paused: "text-warm",
};

const INVOICE_TAG_STYLE: Record<InvoiceStatus, string> = {
  paid: "bg-primary/15 text-primary",
  pending: "bg-yellow-500/15 text-yellow-500",
  cancelled: "bg-muted text-muted-foreground",
  refunded: "bg-warm/15 text-warm",
};

/* Invoice tag labels are resolved via i18n at render time */

/* ─── Connection ring indicator ─────────────────────────────────────── */

function ConnectionRing({ isActive }: { isActive: boolean }) {
  const RING_SIZE = 88;
  const STROKE = 3;
  const RADIUS = (RING_SIZE - STROKE * 2) / 2;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

  return (
    <div className="relative flex items-center justify-center mx-auto" style={{ width: RING_SIZE, height: RING_SIZE }}>
      <svg
        width={RING_SIZE}
        height={RING_SIZE}
        className="absolute inset-0 -rotate-90"
      >
        <circle
          cx={RING_SIZE / 2}
          cy={RING_SIZE / 2}
          r={RADIUS}
          fill="none"
          stroke="currentColor"
          strokeWidth={STROKE}
          className="text-border"
        />
        {isActive && (
          <circle
            cx={RING_SIZE / 2}
            cy={RING_SIZE / 2}
            r={RADIUS}
            fill="none"
            stroke="currentColor"
            strokeWidth={STROKE}
            strokeDasharray={CIRCUMFERENCE}
            strokeDashoffset={CIRCUMFERENCE * 0.15}
            strokeLinecap="round"
            className="text-primary"
          />
        )}
      </svg>
      <div className="flex flex-col items-center gap-1">
        {isActive && (
          <span className="block h-2 w-2 rounded-full bg-primary animate-pulse-glow" />
        )}
        {!isActive && (
          <span className="block h-2 w-2 rounded-full bg-muted-foreground" />
        )}
        <span className={cn(
          "text-[11px] font-semibold tracking-wide uppercase",
          isActive ? "text-primary" : "text-muted-foreground",
        )}>
          {isActive ? "ON" : "OFF"}
        </span>
      </div>
    </div>
  );
}

/* ─── Device icon helper (cycles through device types) ──────────────── */

const DEVICE_ICONS: LucideIcon[] = [Monitor, Smartphone, Tablet];

/* ─── Main page component ───────────────────────────────────────────── */

export function DashboardPage() {
  const { t } = useTranslation();
  const { data: user } = useMe();
  const { data: subscriptions, isLoading: subsLoading } = useSubscriptions();
  const { data: invoices, isLoading: invoicesLoading } = useInvoices();
  const { data: bindings, isLoading: bindingsLoading } = useBindings();

  const isLoading = subsLoading || invoicesLoading || bindingsLoading;

  if (isLoading) {
    return <LoadingSpinner />;
  }

  const allSubs = subscriptions ?? [];
  const allInvoices = invoices ?? [];
  const allBindings = bindings ?? [];

  const activeSubs = allSubs.filter((s) => s.status === "active");
  const syncedBindings = allBindings.filter((b) => b.status === "synced");
  const totalTrafficBytes = allBindings.reduce((sum, b) => sum + b.traffic_limit_bytes, 0);
  const BYTES_PER_GB = 1_073_741_824;
  const totalTrafficGB = (totalTrafficBytes / BYTES_PER_GB).toFixed(1);
  const isVpnActive = syncedBindings.length > 0;

  /* ── KPI cards data ────────── */
  const kpiCards = [
    {
      label: t("nav.subscriptions"),
      value: activeSubs.length,
      sub: t("subscriptions.status.active").toLowerCase(),
      glowColor: "bg-primary",
    },
    {
      label: t("nav.traffic"),
      value: totalTrafficGB,
      sub: "GB",
      glowColor: "bg-warm",
    },
    {
      label: t("bindings.title"),
      value: syncedBindings.length,
      sub: t("bindings.status.synced").toLowerCase(),
      glowColor: "bg-[#5eead4]",
    },
    {
      label: t("nav.invoices"),
      value: allInvoices.length,
      sub: t("invoices.title").toLowerCase(),
      glowColor: "bg-yellow-500",
    },
  ];

  /* ── Determine greeting based on hour ── */
  const hour = new Date().getHours();
  const greetingKey =
    hour < 6 ? "night" : hour < 12 ? "morning" : hour < 18 ? "afternoon" : "evening";
  const greetingMap: Record<string, string> = {
    night: t("common.goodNight"),
    morning: t("common.goodMorning"),
    afternoon: t("common.goodAfternoon"),
    evening: t("common.goodEvening"),
  };
  const greeting = greetingMap[greetingKey];

  return (
    <div className="space-y-8">
      {/* ════════ Hero ════════ */}
      <section className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between animate-fade-up">
        <div>
          <h1
            className="font-bold leading-[1.05] tracking-tight text-foreground"
            style={{ fontSize: "clamp(32px, 5vw, 56px)" }}
          >
            {t("common.dashboard")}{"\n"}
            <span className="italic text-primary">{t("nav.traffic").toLowerCase()}.</span>
          </h1>
          {user && (
            <p className="mt-3 text-sm text-muted-foreground">
              {greeting},{" "}
              <span className="font-semibold text-foreground">
                {user.display_name ?? user.email}
              </span>{" "}
              — {activeSubs.length} {t("subscriptions.status.active").toLowerCase()}{" "}
              {t("nav.subscriptions").toLowerCase()}
            </p>
          )}
        </div>

        {/* VPN status pill */}
        <div
          className={cn(
            "inline-flex items-center gap-2 rounded-full border px-4 py-2 text-xs font-semibold tracking-wide uppercase shrink-0",
            isVpnActive
              ? "border-primary/30 bg-primary/10 text-primary"
              : "border-border bg-card text-muted-foreground",
          )}
        >
          <span
            className={cn(
              "block h-2 w-2 rounded-full",
              isVpnActive ? "bg-primary animate-pulse-glow" : "bg-muted-foreground",
            )}
          />
          {isVpnActive ? t("common.vpnActive") : t("common.vpnOffline")}
        </div>
      </section>

      {/* ════════ KPI Row ════════ */}
      <section className="grid grid-cols-2 gap-2.5 lg:grid-cols-4">
        {kpiCards.map((kpi, idx) => (
          <div
            key={kpi.label}
            className="relative overflow-hidden rounded-lg border border-border bg-card p-[18px_20px] animate-fade-up"
            style={{ animationDelay: `${idx * 80}ms` }}
          >
            <p className="text-[10px] font-semibold uppercase tracking-widest text-tertiary">
              {kpi.label}
            </p>
            <p className="mt-1.5 text-2xl font-bold text-foreground font-mono animate-count-up">
              {kpi.value}
            </p>
            <p className="mt-0.5 text-xs font-mono text-muted-foreground">{kpi.sub}</p>
            {/* glow orb */}
            <div className={cn("kpi-glow", kpi.glowColor)} />
          </div>
        ))}
      </section>

      {/* ════════ Main content 2-column grid ════════ */}
      <div className="grid gap-6 lg:grid-cols-[1fr_370px]">
        {/* ── Left column ── */}
        <div className="space-y-6">
          {/* Subscriptions section */}
          {allSubs.length > 0 ? (
            <div className="glass-card p-5 animate-fade-up" style={{ animationDelay: "320ms" }}>
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2.5">
                  <h2 className="text-sm font-semibold text-foreground">
                    {t("subscriptions.title")}
                  </h2>
                  <span className="rounded-full bg-primary/15 px-2 py-0.5 text-[10px] font-bold text-primary tabular-nums">
                    {allSubs.length}
                  </span>
                </div>
                <Link
                  to="/subscriptions"
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
                >
                  {t("common.view")}
                  <ArrowUpRight size={12} />
                </Link>
              </div>

              <div className="space-y-2.5">
                {allSubs.map((sub, idx) => {
                  const subBindings = allBindings.filter(
                    (b) => b.subscription_id === sub.id,
                  );
                  const trafficUsed = subBindings.reduce(
                    (sum, b) => sum + b.traffic_limit_bytes,
                    0,
                  );
                  /* We don't have plan data directly, so show subscription ID as name */
                  const planLabel = sub.plan_id.slice(0, 8);

                  return (
                    <Link
                      key={sub.id}
                      to="/subscriptions/$id"
                      params={{ id: sub.id }}
                      className="group flex items-center gap-3.5 rounded-lg border border-border bg-background/50 p-3.5 transition-all hover:border-primary/20 hover:bg-accent/50"
                      style={{ animationDelay: `${(idx + 4) * 60}ms` }}
                    >
                      {/* Icon wrap */}
                      <div
                        className={cn(
                          "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
                          sub.status === "active"
                            ? "bg-primary/15 text-primary"
                            : sub.status === "pending"
                              ? "bg-yellow-500/15 text-yellow-500"
                              : "bg-muted text-muted-foreground",
                        )}
                      >
                        <Package size={18} />
                      </div>

                      {/* Plan info */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-semibold text-foreground truncate">
                            {planLabel}
                          </span>
                          <span
                            className={cn(
                              "inline-flex items-center gap-1 text-[10px] font-medium",
                              STATUS_TEXT_COLOR[sub.status],
                            )}
                          >
                            <span
                              className={cn(
                                "block h-1.5 w-1.5 rounded-full",
                                STATUS_DOT_COLOR[sub.status],
                              )}
                            />
                            {t(`subscriptions.status.${sub.status}`)}
                          </span>
                        </div>
                        <p className="text-[11px] font-mono text-muted-foreground mt-0.5">
                          {t("subscriptions.periodEnd")}:{" "}
                          {formatDate(sub.period_end)}
                        </p>
                        {/* Mini device icons */}
                        {subBindings.length > 0 && (
                          <div className="flex items-center gap-1 mt-1.5">
                            {subBindings.slice(0, 3).map((b, i) => {
                              const Icon = DEVICE_ICONS[i % DEVICE_ICONS.length] ?? Monitor;
                              return (
                                <div
                                  key={b.id}
                                  className={cn(
                                    "flex h-5 w-5 items-center justify-center rounded",
                                    b.status === "synced"
                                      ? "bg-primary/10 text-primary"
                                      : "bg-muted text-muted-foreground",
                                  )}
                                >
                                  <Icon size={11} />
                                </div>
                              );
                            })}
                            {subBindings.length > 3 && (
                              <span className="text-[10px] font-mono text-muted-foreground ml-0.5">
                                +{subBindings.length - 3}
                              </span>
                            )}
                          </div>
                        )}
                      </div>

                      {/* Traffic gauge + chevron */}
                      <div className="flex items-center gap-2 shrink-0">
                        {trafficUsed > 0 && (
                          <div className="flex flex-col items-center">
                            <span className="text-[10px] font-mono text-muted-foreground">
                              {formatBytes(trafficUsed)}
                            </span>
                          </div>
                        )}
                        <ChevronRight
                          size={14}
                          className="text-muted-foreground group-hover:text-foreground transition-colors"
                        />
                      </div>
                    </Link>
                  );
                })}
              </div>
            </div>
          ) : (
            /* Empty state */
            <div
              className="glass-card flex flex-col items-center justify-center p-12 animate-fade-up"
              style={{ animationDelay: "320ms" }}
            >
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-muted">
                <Shield size={28} className="text-muted-foreground" />
              </div>
              <h3 className="mt-4 text-sm font-semibold text-foreground">
                {t("subscriptions.empty")}
              </h3>
              <Link
                to="/plans"
                className="mt-4 rounded-lg bg-primary px-5 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
              >
                {t("subscriptions.browsePlans")}
              </Link>
            </div>
          )}

          {/* Activity log */}
          {allInvoices.length > 0 && (
            <div
              className="glass-card p-5 animate-fade-up"
              style={{ animationDelay: "400ms" }}
            >
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-sm font-semibold text-foreground">
                  {t("invoices.title")}
                </h2>
                <span className="text-[10px] font-mono text-muted-foreground">
                  {allInvoices.length} {t("invoices.title").toLowerCase()}
                </span>
              </div>

              <div className="space-y-1">
                {allInvoices.slice(0, 6).map((inv) => (
                  <div
                    key={inv.id}
                    className="flex items-center gap-3 rounded-lg px-3 py-2.5 transition-colors hover:bg-accent/50"
                  >
                    {/* Tag badge */}
                    <span
                      className={cn(
                        "inline-flex shrink-0 items-center rounded-md px-2 py-0.5 text-[10px] font-bold tracking-wide",
                        INVOICE_TAG_STYLE[inv.status],
                      )}
                    >
                      {t(`invoices.status.${inv.status}`).toUpperCase()}
                    </span>

                    {/* Message */}
                    <span className="flex-1 truncate text-xs text-foreground">
                      {t("invoices.amount")}:{" "}
                      <span className="font-mono">
                        {formatMoney(inv.total_amount, inv.currency)}
                      </span>
                    </span>

                    {/* Date */}
                    <span className="shrink-0 text-[10px] font-mono text-muted-foreground">
                      {formatDate(inv.paid_at ?? inv.created_at)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* ── Right column ── */}
        <div className="space-y-6 lg:space-y-6">
          {/* Connection panel */}
          <div
            className="glass-card p-5 animate-fade-up"
            style={{ animationDelay: "480ms" }}
          >
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-sm font-semibold text-foreground">
                {t("bindings.title")}
              </h2>
              <span
                className={cn(
                  "rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase",
                  isVpnActive
                    ? "bg-primary/15 text-primary"
                    : "bg-muted text-muted-foreground",
                )}
              >
                {isVpnActive ? t("common.live") : t("common.offline")}
              </span>
            </div>

            {/* Ring indicator */}
            <div className="mb-5">
              <ConnectionRing isActive={isVpnActive} />
            </div>

            {/* Connection detail rows */}
            <div className="space-y-2.5">
              {isVpnActive ? (
                <>
                  {syncedBindings.slice(0, 3).map((b) => (
                    <div
                      key={b.id}
                      className="flex items-center justify-between rounded-lg bg-background/50 px-3 py-2.5"
                    >
                      <div className="flex items-center gap-2">
                        <Wifi size={12} className="text-primary" />
                        <span className="text-xs text-muted-foreground">
                          {b.remnawave_username}
                        </span>
                      </div>
                      <span className="text-[10px] font-mono text-primary">
                        {t("bindings.status.synced")}
                      </span>
                    </div>
                  ))}
                  <div className="flex items-center justify-between rounded-lg bg-background/50 px-3 py-2.5">
                    <span className="text-xs text-muted-foreground">
                      {t("traffic.title")}
                    </span>
                    <span className="text-[10px] font-mono text-foreground">
                      {formatBytes(totalTrafficBytes)}
                    </span>
                  </div>
                </>
              ) : (
                <div className="flex flex-col items-center gap-2 py-4">
                  <WifiOff size={20} className="text-muted-foreground" />
                  <p className="text-xs text-muted-foreground">
                    {t("bindings.empty")}
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* CTA upgrade card */}
          <div
            className="relative overflow-hidden rounded-[var(--radius)] bg-gradient-to-br from-primary to-[#5eead4] p-6 animate-fade-up"
            style={{ animationDelay: "560ms" }}
          >
            {/* Decorative orb */}
            <div className="absolute -top-8 -right-8 h-32 w-32 rounded-full bg-white/10 blur-2xl pointer-events-none" />

            <Sparkles size={20} className="text-primary-foreground mb-3" />
            <h3 className="text-base font-bold text-primary-foreground leading-snug">
              {t("plans.title")}
            </h3>
            <p className="mt-1.5 text-xs text-primary-foreground/70 leading-relaxed">
              {t("plans.subtitle")}
            </p>
            <Link
              to="/plans"
              className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-primary-foreground/20 px-4 py-2 text-xs font-semibold text-primary-foreground backdrop-blur-xs transition-colors hover:bg-primary-foreground/30"
            >
              {t("subscriptions.browsePlans")}
              <ArrowUpRight size={12} />
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
