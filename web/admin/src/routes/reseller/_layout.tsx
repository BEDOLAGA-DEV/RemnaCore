import { ErrorBoundary } from "@remnacore/shared";
import { Link, Outlet } from "@tanstack/react-router";
import { Bot, LayoutDashboard, Users, Wallet } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PulseDot } from "@/components/ui";
import { ShopPicker } from "../../components/reseller/ShopPicker.js";
import { CommandPalette } from "../../components/shell/CommandPalette.js";
import { Header } from "../../components/shell/Header.js";

const LINK_BASE =
  "flex items-center gap-2.5 border-l-2 border-l-transparent px-2 py-2 text-[12px] tracking-[.5px] text-t4 transition-colors hover:bg-white/[.025] hover:text-t2 [&.active]:border-l-accent [&.active]:bg-white/[.03] [&.active]:font-medium [&.active]:text-accent";

function GroupHeader({ children }: { children: string }) {
  return (
    <div className="px-2 pb-[7px] pt-3 text-[9px] uppercase tracking-[2px] text-t8">
      {children}
    </div>
  );
}

export function ResellerLayout() {
  const { t } = useTranslation();
  const [cmdOpen, setCmdOpen] = useState(false);

  return (
    <div
      className="grid min-h-screen bg-bg"
      style={{ gridTemplateColumns: "236px 1fr" }}
    >
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
              reseller
            </div>
          </div>
        </div>

        {/* Active-shop picker (X-Shop-Id source for all reseller pages) */}
        <ShopPicker />

        {/* Navigation */}
        <nav className="flex flex-1 flex-col overflow-y-auto px-2.5 py-3">
          <GroupHeader>{t("reseller.nav.overview")}</GroupHeader>
          <Link to="/reseller/dashboard" className={LINK_BASE}>
            <LayoutDashboard size={15} className="shrink-0" />
            <span>{t("reseller.dashboard.title")}</span>
          </Link>
          <Link to="/reseller/customers" className={LINK_BASE}>
            <Users size={15} className="shrink-0" />
            <span>{t("reseller.customers.title")}</span>
          </Link>
          <Link to="/reseller/commissions" className={LINK_BASE}>
            <Wallet size={15} className="shrink-0" />
            <span>{t("reseller.commissions.title")}</span>
          </Link>
          <GroupHeader>{t("reseller.nav.settings")}</GroupHeader>
          <Link to="/reseller/bot" className={LINK_BASE}>
            <Bot size={15} className="shrink-0" />
            <span>{t("reseller.bot.title")}</span>
          </Link>
        </nav>

        {/* Footer */}
        <div className="flex items-center gap-2 border-t border-line px-4 py-3 text-[10px] uppercase tracking-[1px] text-t7">
          <PulseDot />
          {t("admin.allSystemsNominal")}
        </div>
      </aside>

      <main className="flex min-w-0 flex-col">
        <Header onOpenSearch={() => setCmdOpen(true)} />
        <div className="flex flex-col gap-3.5 p-[22px]">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </div>
      </main>

      <CommandPalette open={cmdOpen} onOpenChange={setCmdOpen} />
    </div>
  );
}
