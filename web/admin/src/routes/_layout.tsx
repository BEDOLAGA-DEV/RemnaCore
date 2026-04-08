import { Outlet } from "@tanstack/react-router";
import { LogOut } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import {
  ErrorBoundary,
  useAuthStore,
  useLogout,
} from "@remnacore/shared";
import { AdminSidebar } from "../components/AdminSidebar.js";

function formatUtcClock(date: Date): string {
  const h = String(date.getUTCHours()).padStart(2, "0");
  const m = String(date.getUTCMinutes()).padStart(2, "0");
  const s = String(date.getUTCSeconds()).padStart(2, "0");
  return `${h}:${m}:${s}`;
}

function getInitial(user: { email: string; display_name: string | null }): string {
  if (user.display_name && user.display_name.length > 0) {
    return user.display_name.charAt(0).toUpperCase();
  }
  return user.email.charAt(0).toUpperCase();
}

export function AdminLayout() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const logout = useLogout();
  const [utcTime, setUtcTime] = useState(() => formatUtcClock(new Date()));

  useEffect(() => {
    const interval = setInterval(() => {
      setUtcTime(formatUtcClock(new Date()));
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="flex min-h-screen">
      <AdminSidebar />

      <div className="flex flex-1 flex-col">
        {/* Top bar */}
        <header className="flex h-[52px] items-center justify-between border-b border-border bg-card px-5">
          <div className="flex items-center gap-3">
            <h1 className="text-sm font-medium text-foreground">
              {t("admin.title")}
            </h1>
            <div className="flex items-center gap-1.5 rounded-full bg-success/15 px-2 py-0.5">
              <span className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-success" />
              <span className="font-mono text-[10px] font-medium uppercase tracking-wider text-success">
                {t("admin.live")}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-4">
            <span className="font-mono text-[12px] text-dim">
              {utcTime} UTC
            </span>

            {user ? (
              <span className="text-[12px] text-muted-foreground">
                {user.email}
              </span>
            ) : null}

            {user ? (
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-primary/40 to-primary/20 text-[11px] font-semibold text-primary">
                {getInitial(user)}
              </div>
            ) : null}

            <button
              type="button"
              onClick={logout}
              className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
            >
              <LogOut size={15} />
            </button>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-auto bg-background p-5">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  );
}
