import { Outlet } from "@tanstack/react-router";
import { ErrorBoundary } from "@remnacore/shared";
import { Navbar } from "../components/Navbar.js";
import { Sidebar } from "../components/Sidebar.js";

export function Layout() {
  return (
    <div className="flex min-h-screen flex-col">
      <Navbar />
      <div className="flex flex-1">
        <Sidebar />
        <main className="flex-1 overflow-auto p-4 lg:p-6">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  );
}
