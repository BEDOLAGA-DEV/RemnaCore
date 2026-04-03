import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Moon, Sun, LogOut, Menu, X } from "lucide-react";
import { useState } from "react";
import {
  useAuthStore,
  useThemeStore,
  useLogout,
  cn,
} from "@remnacore/shared";

export function Navbar() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();
  const logout = useLogout();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <header className="sticky top-0 z-50 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="flex h-14 items-center px-4 lg:px-6">
        <Link to="/" className="flex items-center gap-2 font-semibold">
          <span className="text-primary text-lg">RemnaCore</span>
        </Link>

        {/* Desktop nav */}
        <nav className="ml-8 hidden items-center gap-6 md:flex">
          <Link
            to="/"
            className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground [&.active]:text-foreground"
          >
            {t("nav.dashboard")}
          </Link>
          <Link
            to="/plans"
            className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground [&.active]:text-foreground"
          >
            {t("nav.plans")}
          </Link>
          <Link
            to="/subscriptions"
            className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground [&.active]:text-foreground"
          >
            {t("nav.subscriptions")}
          </Link>
          <Link
            to="/family"
            className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground [&.active]:text-foreground"
          >
            {t("nav.family")}
          </Link>
          <Link
            to="/traffic"
            className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground [&.active]:text-foreground"
          >
            {t("nav.traffic")}
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-3">
          <button
            type="button"
            onClick={toggleTheme}
            className="rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            aria-label={theme === "dark" ? t("common.lightMode") : t("common.darkMode")}
          >
            {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
          </button>

          <Link
            to="/profile"
            className="hidden text-sm font-medium text-muted-foreground hover:text-foreground md:block"
          >
            {user?.display_name ?? user?.email ?? t("common.profile")}
          </Link>

          <button
            type="button"
            onClick={logout}
            className="rounded-lg p-2 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors"
            aria-label={t("common.logout")}
          >
            <LogOut size={18} />
          </button>

          {/* Mobile menu button */}
          <button
            type="button"
            onClick={() => setMobileOpen(!mobileOpen)}
            className="rounded-lg p-2 text-muted-foreground hover:bg-accent md:hidden"
          >
            {mobileOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>
      </div>

      {/* Mobile nav */}
      {mobileOpen && (
        <nav className="border-t border-border px-4 py-3 md:hidden">
          <div className="flex flex-col gap-2">
            {[
              { to: "/" as const, label: t("nav.dashboard") },
              { to: "/plans" as const, label: t("nav.plans") },
              { to: "/subscriptions" as const, label: t("nav.subscriptions") },
              { to: "/family" as const, label: t("nav.family") },
              { to: "/traffic" as const, label: t("nav.traffic") },
              { to: "/profile" as const, label: t("nav.profile") },
            ].map((item) => (
              <Link
                key={item.to}
                to={item.to}
                onClick={() => setMobileOpen(false)}
                className={cn(
                  "rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
                  "[&.active]:bg-accent [&.active]:text-foreground",
                )}
              >
                {item.label}
              </Link>
            ))}
          </div>
        </nav>
      )}
    </header>
  );
}
