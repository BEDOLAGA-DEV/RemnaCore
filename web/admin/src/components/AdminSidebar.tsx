import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  LayoutDashboard,
  Users,
  Package,
  FileText,
  Puzzle,
  Building2,
  Server,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { useState } from "react";
import { cn } from "@remnacore/shared";

const navItems = [
  { to: "/" as const, icon: LayoutDashboard, labelKey: "admin.dashboard.title" },
  { to: "/users" as const, icon: Users, labelKey: "admin.users.title" },
  { to: "/subscriptions" as const, icon: Package, labelKey: "admin.subscriptions.title" },
  { to: "/invoices" as const, icon: FileText, labelKey: "admin.invoices.title" },
  { to: "/plugins" as const, icon: Puzzle, labelKey: "admin.plugins.title" },
  { to: "/tenants" as const, icon: Building2, labelKey: "admin.tenants.title" },
  { to: "/nodes" as const, icon: Server, labelKey: "admin.nodes.title" },
] as const;

export function AdminSidebar() {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(false);

  return (
    <aside
      className={cn(
        "hidden border-r border-border bg-card transition-all duration-200 lg:flex lg:flex-col",
        collapsed ? "w-16" : "w-64",
      )}
    >
      <div className="flex h-14 items-center justify-between border-b border-border px-4">
        {!collapsed && (
          <span className="text-lg font-semibold text-primary">
            Admin
          </span>
        )}
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          className="rounded-lg p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
        </button>
      </div>

      <nav className="flex flex-1 flex-col gap-1 p-2">
        {navItems.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={cn(
              "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
              "[&.active]:bg-accent [&.active]:text-foreground",
              collapsed && "justify-center px-2",
            )}
            title={collapsed ? t(item.labelKey) : undefined}
          >
            <item.icon size={18} />
            {!collapsed && <span>{t(item.labelKey)}</span>}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
