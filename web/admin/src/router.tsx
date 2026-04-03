import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { useAuthStore, USER_ROLES } from "@remnacore/shared";
import { AdminLayout } from "./routes/_layout.js";
import { AdminDashboardPage } from "./routes/index.js";
import { AdminLoginPage } from "./routes/login.js";
import { UsersPage } from "./routes/users/index.js";
import { UserDetailPage } from "./routes/users/[id].js";
import { AdminSubscriptionsPage } from "./routes/subscriptions/index.js";
import { AdminSubscriptionDetailPage } from "./routes/subscriptions/[id].js";
import { AdminInvoicesPage } from "./routes/invoices.js";
import { PluginsPage } from "./routes/plugins/index.js";
import { InstallPluginPage } from "./routes/plugins/install.js";
import { PluginDetailPage } from "./routes/plugins/[id].js";
import { TenantsPage } from "./routes/tenants/index.js";
import { TenantDetailPage } from "./routes/tenants/[id].js";
import { NodesPage } from "./routes/nodes.js";

const rootRoute = createRootRoute({
  component: Outlet,
});

function requireAdmin() {
  const { isAuthenticated, user } = useAuthStore.getState();
  if (!isAuthenticated) {
    throw redirect({ to: "/login" });
  }
  if (user?.role !== USER_ROLES.admin) {
    throw redirect({ to: "/login" });
  }
}

function requireGuest() {
  const { isAuthenticated, user } = useAuthStore.getState();
  if (isAuthenticated && user?.role === USER_ROLES.admin) {
    throw redirect({ to: "/" });
  }
}

// ─── Public ─────────────────────────────────────────────────────────────────

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  beforeLoad: requireGuest,
  component: AdminLoginPage,
});

// ─── Protected (admin layout) ──────────────────────────────────────────────

const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "admin-layout",
  beforeLoad: requireAdmin,
  component: AdminLayout,
});

const dashboardRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/",
  component: AdminDashboardPage,
});

const usersRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/users",
  component: UsersPage,
});

const userDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/users/$id",
  component: UserDetailPage,
});

const subscriptionsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/subscriptions",
  component: AdminSubscriptionsPage,
});

const subscriptionDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/subscriptions/$id",
  component: AdminSubscriptionDetailPage,
});

const invoicesRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/invoices",
  component: AdminInvoicesPage,
});

const pluginsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/plugins",
  component: PluginsPage,
});

const installPluginRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/plugins/install",
  component: InstallPluginPage,
});

const pluginDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/plugins/$id",
  component: PluginDetailPage,
});

const tenantsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/tenants",
  component: TenantsPage,
});

const tenantDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/tenants/$id",
  component: TenantDetailPage,
});

const nodesRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/nodes",
  component: NodesPage,
});

// ─── Tree ───────────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  loginRoute,
  layoutRoute.addChildren([
    dashboardRoute,
    usersRoute,
    userDetailRoute,
    subscriptionsRoute,
    subscriptionDetailRoute,
    invoicesRoute,
    pluginsRoute,
    installPluginRoute,
    pluginDetailRoute,
    tenantsRoute,
    tenantDetailRoute,
    nodesRoute,
  ]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
