import { Outlet } from "@tanstack/react-router";
import { ErrorBoundary } from "@remnacore/shared";
import { Navbar } from "../components/Navbar.js";

export function Layout() {
  return (
    <div className="min-h-screen ambient-bg">
      <Navbar />
      <main className="relative z-[1] mx-auto max-w-[1200px] px-6 pb-16 pt-7 lg:px-6 md:px-4 sm:px-4">
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
}
