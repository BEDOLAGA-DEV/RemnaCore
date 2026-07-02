import { Link } from "@tanstack/react-router";
import {
  ArrowUpCircle,
  BarChart3,
  BookOpen,
  Boxes,
  Building2,
  Calculator,
  ChevronDown,
  Clock,
  CreditCard,
  FileText,
  Globe,
  LayoutDashboard,
  List,
  type LucideIcon,
  Network,
  Package,
  Plus,
  Puzzle,
  Search,
  Server,
  Settings,
  ShoppingCart,
  Smartphone,
  Tag,
  TrendingUp,
  Users,
  Wallet,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PulseDot } from "../ui/index.js";
import { STATIC_NAV, usePluginNavGroups } from "./nav.js";

const ICON_MAP: Record<string, LucideIcon> = {
  ArrowUpCircle,
  BarChart3,
  BookOpen,
  Boxes,
  Building2,
  Calculator,
  Clock,
  CreditCard,
  FileText,
  Globe,
  LayoutDashboard,
  List,
  Network,
  Package,
  Plus,
  Puzzle,
  Search,
  Server,
  Settings,
  ShoppingCart,
  Smartphone,
  Tag,
  TrendingUp,
  Users,
  Wallet,
};

const LINK_BASE =
  "flex items-center gap-2.5 border-l-2 border-l-transparent px-2 py-2 text-[12px] tracking-[.5px] text-t4 transition-colors hover:bg-white/[.025] hover:text-t2 [&.active]:border-l-accent [&.active]:bg-white/[.03] [&.active]:font-medium [&.active]:text-accent";

const SUB_BASE =
  "flex items-center gap-2 border-l-2 border-l-transparent py-[6px] pl-9 pr-2 text-[11px] tracking-[.5px] text-t5 transition-colors hover:bg-white/[.025] hover:text-t2 [&.active]:border-l-accent [&.active]:bg-white/[.03] [&.active]:text-accent";

function GroupHeader({ children }: { children: string }) {
  return (
    <div className="px-2 pb-[7px] pt-3 text-[9px] uppercase tracking-[2px] text-t8">
      {children}
    </div>
  );
}

export function Sidebar() {
  const { t } = useTranslation();
  const pluginGroups = usePluginNavGroups();
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  return (
    <aside className="sticky top-0 flex h-screen w-[236px] flex-col border-r border-line bg-raised">
      {/* Logo */}
      <div className="flex items-center gap-2.5 border-b border-line px-[18px] py-4">
        <span className="relative h-[22px] w-[22px] rotate-45 border border-accent">
          <span className="absolute inset-[5px] bg-accent" />
        </span>
        <div>
          <div className="text-[14px] font-extrabold tracking-[1px]">
            REMNA<span className="text-accent">CORE</span>
          </div>
          <div className="mt-0.5 text-[9px] uppercase tracking-[3px] text-t7">
            nerve center
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex flex-1 flex-col overflow-y-auto px-2.5 py-3">
        <GroupHeader>System</GroupHeader>
        {STATIC_NAV.map((item) => (
          <Link key={item.to} to={item.to} className={LINK_BASE}>
            <item.icon size={15} className="shrink-0" />
            <span>{t(item.labelKey)}</span>
          </Link>
        ))}

        {pluginGroups.map((group) => {
          const GroupIcon = group.icon;
          // Single-page plugin → direct link.
          if (group.pages.length === 1) {
            const page = group.pages[0];
            if (!page) return null;
            return (
              <div key={group.slug}>
                <GroupHeader>{group.label}</GroupHeader>
                <Link
                  to="/plugins/$slug/page/$pagePath"
                  params={{ slug: group.slug, pagePath: page.path }}
                  className={LINK_BASE}
                >
                  <GroupIcon size={15} className="shrink-0" />
                  <span>{page.title}</span>
                </Link>
              </div>
            );
          }
          const isOpen = expanded[group.slug] ?? false;
          return (
            <div key={group.slug}>
              <button
                type="button"
                onClick={() =>
                  setExpanded((p) => ({ ...p, [group.slug]: !p[group.slug] }))
                }
                className={`${LINK_BASE} w-full justify-between`}
              >
                <span className="flex items-center gap-2.5">
                  <GroupIcon size={15} className="shrink-0" />
                  <span>{group.label}</span>
                </span>
                <ChevronDown
                  size={13}
                  className={`shrink-0 text-t6 transition-transform ${isOpen ? "rotate-180" : ""}`}
                />
              </button>
              {isOpen &&
                group.pages.map((page) => {
                  const Icon = ICON_MAP[page.icon] ?? Puzzle;
                  return (
                    <Link
                      key={`${group.slug}-${page.path}`}
                      to="/plugins/$slug/page/$pagePath"
                      params={{ slug: group.slug, pagePath: page.path }}
                      className={SUB_BASE}
                    >
                      <Icon size={13} className="shrink-0" />
                      <span>{page.title}</span>
                    </Link>
                  );
                })}
            </div>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="flex items-center gap-2 border-t border-line px-4 py-3 text-[10px] uppercase tracking-[1px] text-t7">
        <PulseDot />
        {t("admin.allSystemsNominal")}
      </div>
    </aside>
  );
}
