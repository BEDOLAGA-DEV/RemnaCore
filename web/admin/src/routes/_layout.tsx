import { ErrorBoundary } from "@remnacore/shared";
import { Outlet } from "@tanstack/react-router";
import { useState } from "react";
import { CommandPalette } from "../components/shell/CommandPalette.js";
import { Header } from "../components/shell/Header.js";
import { Sidebar } from "../components/shell/Sidebar.js";

export function AdminLayout() {
  const [cmdOpen, setCmdOpen] = useState(false);

  return (
    <div
      className="grid min-h-screen bg-bg"
      style={{ gridTemplateColumns: "236px 1fr" }}
    >
      <Sidebar />

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
