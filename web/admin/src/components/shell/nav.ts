import { usePluginPages } from "@remnacore/shared";
import {
  Building2,
  CreditCard,
  FileText,
  Globe,
  LayoutDashboard,
  type LucideIcon,
  Puzzle,
  Server,
  Settings,
  ShoppingCart,
  Users,
  Wallet,
} from "lucide-react";
import { useMemo } from "react";

export type NavItem = {
  to: string;
  params?: Record<string, string>;
  icon: LucideIcon;
  labelKey: string; // i18n key, or literal if `literal` is true
  literal?: boolean;
};

export type NavGroup = { name: string; items: NavItem[] };

// Primary navigation (static routes). Group header: SYSTEM.
export const STATIC_NAV: NavItem[] = [
  { to: "/", icon: LayoutDashboard, labelKey: "admin.dashboard.title" },
  { to: "/users", icon: Users, labelKey: "admin.users.title" },
  { to: "/subscriptions", icon: CreditCard, labelKey: "admin.subscriptions.title" },
  { to: "/invoices", icon: FileText, labelKey: "admin.invoices.title" },
  { to: "/plugins", icon: Puzzle, labelKey: "admin.plugins.title" },
  { to: "/nodes", icon: Globe, labelKey: "admin.nodes.title" },
  { to: "/tenants", icon: Building2, labelKey: "admin.tenants.title" },
  { to: "/settings", icon: Settings, labelKey: "admin.settings.title" },
];

const PLUGIN_LABELS: Record<string, string> = {
  "tariff-manager": "Tariffs",
  "remnawave-provider": "Remnawave",
  "user-balance": "Balance",
  checkout: "Checkout",
};

const PLUGIN_ICONS: Record<string, LucideIcon> = {
  "tariff-manager": CreditCard,
  "remnawave-provider": Server,
  "user-balance": Wallet,
  checkout: ShoppingCart,
};

export type PluginGroup = {
  slug: string;
  label: string;
  icon: LucideIcon;
  pages: Array<{ path: string; title: string; icon: string }>;
};

/** Builds the dynamic plugin-page nav groups from the admin plugin pages. */
export function usePluginNavGroups(): PluginGroup[] {
  const { data: pluginPages } = usePluginPages();
  return useMemo<PluginGroup[]>(() => {
    const adminPages = pluginPages?.filter((p) => p.menu === "admin") ?? [];
    const groups = new Map<string, PluginGroup>();
    for (const page of adminPages) {
      if (!groups.has(page.plugin_slug)) {
        groups.set(page.plugin_slug, {
          slug: page.plugin_slug,
          label: PLUGIN_LABELS[page.plugin_slug] ?? page.plugin_slug,
          icon: PLUGIN_ICONS[page.plugin_slug] ?? Puzzle,
          pages: [],
        });
      }
      groups.get(page.plugin_slug)?.pages.push({
        path: page.path,
        title: page.title,
        icon: page.icon,
      });
    }
    return Array.from(groups.values());
  }, [pluginPages]);
}
