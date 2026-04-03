import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  LayoutDashboard,
  CreditCard,
  Package,
  Users,
  Activity,
  User,
} from "lucide-react";
import { cn } from "@remnacore/shared";

const navItems = [
  { to: "/" as const, icon: LayoutDashboard, labelKey: "nav.dashboard" },
  { to: "/plans" as const, icon: CreditCard, labelKey: "nav.plans" },
  { to: "/subscriptions" as const, icon: Package, labelKey: "nav.subscriptions" },
  { to: "/family" as const, icon: Users, labelKey: "nav.family" },
  { to: "/traffic" as const, icon: Activity, labelKey: "nav.traffic" },
  { to: "/profile" as const, icon: User, labelKey: "nav.profile" },
] as const;

export function Sidebar() {
  const { t } = useTranslation();

  return (
    <aside className="hidden w-64 shrink-0 border-r border-border lg:block">
      <nav className="flex flex-col gap-1 p-4">
        {navItems.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={cn(
              "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
              "[&.active]:bg-accent [&.active]:text-foreground",
            )}
          >
            <item.icon size={18} />
            {t(item.labelKey)}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
