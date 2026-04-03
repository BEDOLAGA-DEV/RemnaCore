import { type ReactNode } from "react";
import { Navigate } from "@tanstack/react-router";
import { useAuthStore } from "../stores/authStore.js";
import type { UserRole } from "../types/index.js";

type ProtectedRouteProps = {
  children: ReactNode;
  requiredRole?: UserRole;
  redirectTo?: string;
};

export function ProtectedRoute({
  children,
  requiredRole,
  redirectTo = "/login",
}: ProtectedRouteProps) {
  const { isAuthenticated, user } = useAuthStore();

  if (!isAuthenticated) {
    return <Navigate to={redirectTo} />;
  }

  if (requiredRole && user?.role !== requiredRole) {
    return <Navigate to={redirectTo} />;
  }

  return <>{children}</>;
}
