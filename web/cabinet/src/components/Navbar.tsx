import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Moon, Sun, LogOut, Menu, X, Shield } from "lucide-react";
import { useState, useCallback } from "react";
import {
  useAuthStore,
  useThemeStore,
  useLogout,
} from "@remnacore/shared";

const NAV_ITEMS = [
  { to: "/" as const, labelKey: "nav.dashboard", hasBadge: false },
  { to: "/plans" as const, labelKey: "nav.plans", hasBadge: false },
  { to: "/subscriptions" as const, labelKey: "nav.subscriptions", hasBadge: true },
  { to: "/family" as const, labelKey: "nav.family", hasBadge: false },
  { to: "/traffic" as const, labelKey: "nav.traffic", hasBadge: false },
  { to: "/profile" as const, labelKey: "nav.profile", hasBadge: false },
] as const;

function BrandMark() {
  return (
    <Link to="/" className="flex items-center gap-2.5 shrink-0">
      <div
        className="flex h-[26px] w-[26px] items-center justify-center rounded-[7px]"
        style={{
          background: "linear-gradient(135deg, #2dd4bf 0%, #0d9488 100%)",
        }}
      >
        <Shield size={14} className="text-[#110f0d]" strokeWidth={2.5} />
      </div>
      <span className="text-base font-bold tracking-tighter text-foreground">
        Remna
      </span>
    </Link>
  );
}

function getUserInitial(user: { display_name?: string | null; email?: string | null } | null): string {
  if (!user) return "?";
  const name = user.display_name ?? user.email ?? "";
  return name.charAt(0).toUpperCase() || "?";
}

export function Navbar() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();
  const logout = useLogout();
  const [mobileOpen, setMobileOpen] = useState(false);

  const closeMobile = useCallback(() => setMobileOpen(false), []);

  const subscriptionCount = (user as Record<string, unknown> | null)?.subscription_count as
    | number
    | undefined;

  return (
    <>
      {/* ── Sticky top bar ── */}
      <header
        className="sticky top-0 z-50 w-full border-b border-border glass"
        style={{ background: "rgba(17,15,13,0.9)" }}
      >
        <div className="mx-auto flex h-14 max-w-[1200px] items-center px-6 md:px-4 sm:px-4">
          {/* Left: brand */}
          <BrandMark />

          {/* Center: pill nav (desktop) */}
          <nav className="ml-auto mr-auto hidden items-center gap-1 md:flex overflow-x-auto scrollbar-none">
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                className="pill-nav"
                activeProps={{ className: "pill-nav pill-nav-active" }}
              >
                {t(item.labelKey)}
                {item.hasBadge && subscriptionCount != null && subscriptionCount > 0 && (
                  <span
                    className="ml-1 inline-flex items-center justify-center rounded-full bg-[rgba(0,0,0,0.35)] px-1.5"
                    style={{ fontSize: "9px", lineHeight: "16px" }}
                  >
                    {subscriptionCount}
                  </span>
                )}
              </Link>
            ))}
          </nav>

          {/* Right: actions */}
          <div className="ml-auto flex items-center gap-2 md:ml-0">
            {/* Theme toggle */}
            <button
              type="button"
              onClick={toggleTheme}
              className="flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-[rgba(245,236,227,0.06)] hover:text-foreground"
              aria-label={
                theme === "dark" ? t("common.lightMode") : t("common.darkMode")
              }
            >
              {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
            </button>

            {/* Logout */}
            <button
              type="button"
              onClick={logout}
              className="hidden h-8 w-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive md:flex"
              aria-label={t("common.logout")}
            >
              <LogOut size={16} />
            </button>

            {/* Avatar */}
            <Link
              to="/profile"
              className="flex h-[30px] w-[30px] items-center justify-center rounded-full text-xs font-semibold"
              style={{
                background: "var(--primary)",
                color: "var(--primary-foreground)",
              }}
              title={user?.display_name ?? user?.email ?? t("common.profile")}
            >
              {getUserInitial(user)}
            </Link>

            {/* Mobile hamburger */}
            <button
              type="button"
              onClick={() => setMobileOpen((prev) => !prev)}
              className="flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-[rgba(245,236,227,0.06)] md:hidden"
              aria-label={mobileOpen ? "Close" : "Menu"}
            >
              {mobileOpen ? <X size={18} /> : <Menu size={18} />}
            </button>
          </div>
        </div>
      </header>

      {/* ── Mobile full-screen overlay ── */}
      {mobileOpen && (
        <div className="fixed inset-0 z-40 md:hidden" aria-modal="true">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-[rgba(17,15,13,0.85)] glass"
            onClick={closeMobile}
            onKeyDown={(e) => {
              if (e.key === "Escape") closeMobile();
            }}
            role="button"
            tabIndex={-1}
            aria-label={t("common.close")}
          />

          {/* Nav content */}
          <nav className="relative z-10 flex flex-col items-center gap-3 px-6 pt-20">
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                onClick={closeMobile}
                className="pill-nav w-full max-w-[280px] justify-center py-3 text-sm"
                activeProps={{ className: "pill-nav pill-nav-active w-full max-w-[280px] justify-center py-3 text-sm" }}
              >
                {t(item.labelKey)}
                {item.hasBadge && subscriptionCount != null && subscriptionCount > 0 && (
                  <span
                    className="ml-1.5 inline-flex items-center justify-center rounded-full bg-[rgba(0,0,0,0.35)] px-1.5"
                    style={{ fontSize: "9px", lineHeight: "16px" }}
                  >
                    {subscriptionCount}
                  </span>
                )}
              </Link>
            ))}

            {/* Mobile logout */}
            <button
              type="button"
              onClick={() => {
                closeMobile();
                logout();
              }}
              className="mt-4 flex w-full max-w-[280px] items-center justify-center gap-2 rounded-full px-3.5 py-3 text-sm font-medium text-destructive transition-colors hover:bg-destructive/10"
            >
              <LogOut size={16} />
              {t("common.logout")}
            </button>
          </nav>
        </div>
      )}
    </>
  );
}
