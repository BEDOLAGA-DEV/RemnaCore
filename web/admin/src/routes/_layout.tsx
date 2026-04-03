import { Outlet } from "@tanstack/react-router";
import { Moon, Sun, LogOut } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  ErrorBoundary,
  useAuthStore,
  useThemeStore,
  useLogout,
} from "@remnacore/shared";
import { AdminSidebar } from "../components/AdminSidebar.js";

export function AdminLayout() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();
  const logout = useLogout();

  return (
    <div className="flex min-h-screen">
      <AdminSidebar />

      <div className="flex flex-1 flex-col">
        {/* Top bar */}
        <header className="flex h-14 items-center justify-between border-b border-border bg-background px-6">
          <h1 className="text-sm font-medium text-muted-foreground">
            {t("admin.title")}
          </h1>
          <div className="flex items-center gap-3">
            <span className="text-sm text-muted-foreground">
              {user?.email}
            </span>
            <button
              type="button"
              onClick={toggleTheme}
              className="rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            >
              {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
            </button>
            <button
              type="button"
              onClick={logout}
              className="rounded-lg p-2 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors"
            >
              <LogOut size={16} />
            </button>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-auto p-6">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  );
}
