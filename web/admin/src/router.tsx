import { USER_ROLES, useAuthStore } from "@remnacore/shared";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { checkSetupNeeded } from "./lib/setupGate.js";
import { AdminLayout } from "./routes/_layout.js";
import { AdminDashboardPage } from "./routes/index.js";
import { AdminInvoicesPage } from "./routes/invoices.js";
import { AdminLoginPage } from "./routes/login.js";
import { NodesPage } from "./routes/nodes.js";
import { PluginDetailPage } from "./routes/plugins/[id].js";
import { PluginsPage } from "./routes/plugins/index.js";
import { InstallPluginPage } from "./routes/plugins/install.js";
import { PluginPageView } from "./routes/plugins/page.js";
import { ResellerLayout } from "./routes/reseller/_layout.js";
import { ResellerBotPage } from "./routes/reseller/bot.js";
import { SettingsPage } from "./routes/settings.js";
import { AdminSetupPage } from "./routes/setup.js";
import { AdminSubscriptionDetailPage } from "./routes/subscriptions/[id].js";
import { AdminSubscriptionsPage } from "./routes/subscriptions/index.js";
import { TenantDetailPage } from "./routes/tenants/[id].js";
import { TenantsPage } from "./routes/tenants/index.js";
import { UserDetailPage } from "./routes/users/[id].js";
import { UsersPage } from "./routes/users/index.js";

const rootRoute = createRootRoute({
  component: Outlet,
});

async function requireAdmin() {
  if (await checkSetupNeeded()) {
    throw redirect({ to: "/setup" });
  }
  const { isAuthenticated, user } = useAuthStore.getState();
  if (!isAuthenticated) {
    throw redirect({ to: "/login" });
  }
  if (user?.role !== USER_ROLES.admin) {
    throw redirect({ to: "/login" });
  }
}

async function requireGuest() {
  if (await checkSetupNeeded()) {
    throw redirect({ to: "/setup" });
  }
  const { isAuthenticated, user } = useAuthStore.getState();
  if (isAuthenticated) {
    if (user?.role === USER_ROLES.admin) {
      throw redirect({ to: "/" });
    }
    if (user?.role === USER_ROLES.reseller) {
      throw redirect({ to: "/reseller/bot" });
    }
  }
}

async function requireReseller() {
  if (await checkSetupNeeded()) {
    throw redirect({ to: "/setup" });
  }
  const { isAuthenticated, user } = useAuthStore.getState();
  if (!isAuthenticated) {
    throw redirect({ to: "/login" });
  }
  if (user?.role !== USER_ROLES.admin && user?.role !== USER_ROLES.reseller) {
    throw redirect({ to: "/login" });
  }
}

// Setup is reachable only while no admin exists; afterwards it bounces to login.
async function requireSetup() {
  if (!(await checkSetupNeeded())) {
    throw redirect({ to: "/login" });
  }
}

// ─── Public ─────────────────────────────────────────────────────────────────

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/setup",
  beforeLoad: requireSetup,
  component: AdminSetupPage,
});

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

const pluginPageRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/plugins/$slug/page/$pagePath",
  component: PluginPageView,
});

const settingsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/settings",
  component: SettingsPage,
});

// ─── Protected (reseller layout) ──────────────────────────────────────────

const resellerLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "reseller-layout",
  beforeLoad: requireReseller,
  component: ResellerLayout,
});

const resellerBotRoute = createRoute({
  getParentRoute: () => resellerLayoutRoute,
  path: "/reseller/bot",
  component: ResellerBotPage,
});

// ─── Tree ───────────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  setupRoute,
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
    pluginPageRoute,
    pluginDetailRoute,
    tenantsRoute,
    tenantDetailRoute,
    nodesRoute,
    settingsRoute,
  ]),
  resellerLayoutRoute.addChildren([resellerBotRoute]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
