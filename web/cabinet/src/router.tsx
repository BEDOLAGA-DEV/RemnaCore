import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { useAuthStore } from "@remnacore/shared";
import { Layout } from "./routes/_layout.js";
import { DashboardPage } from "./routes/index.js";
import { LoginPage } from "./routes/login.js";
import { RegisterPage } from "./routes/register.js";
import { ForgotPasswordPage } from "./routes/forgot-password.js";
import { ResetPasswordPage } from "./routes/reset-password.js";
import { PlansPage } from "./routes/plans.js";
import { CheckoutPage } from "./routes/checkout.js";
import { SubscriptionsPage } from "./routes/subscriptions/index.js";
import { SubscriptionDetailPage } from "./routes/subscriptions/[id].js";
import { FamilyPage } from "./routes/family.js";
import { TrafficPage } from "./routes/traffic.js";
import { ProfilePage } from "./routes/profile.js";

const rootRoute = createRootRoute({
  component: Outlet,
});

// Auth guard for protected routes
function requireAuth() {
  const { isAuthenticated } = useAuthStore.getState();
  if (!isAuthenticated) {
    throw redirect({ to: "/login" });
  }
}

// Redirect if already logged in
function requireGuest() {
  const { isAuthenticated } = useAuthStore.getState();
  if (isAuthenticated) {
    throw redirect({ to: "/" });
  }
}

// ─── Public routes ──────────────────────────────────────────────────────────

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  beforeLoad: requireGuest,
  component: LoginPage,
});

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/register",
  beforeLoad: requireGuest,
  component: RegisterPage,
});

const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/forgot-password",
  beforeLoad: requireGuest,
  component: ForgotPasswordPage,
});

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/reset-password",
  beforeLoad: requireGuest,
  component: ResetPasswordPage,
});

// ─── Protected routes (inside layout) ──────────────────────────────────────

const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "layout",
  beforeLoad: requireAuth,
  component: Layout,
});

const dashboardRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/",
  component: DashboardPage,
});

const plansRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/plans",
  component: PlansPage,
});

const checkoutRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/checkout",
  component: CheckoutPage,
});

const subscriptionsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/subscriptions",
  component: SubscriptionsPage,
});

const subscriptionDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/subscriptions/$id",
  component: SubscriptionDetailPage,
});

const familyRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/family",
  component: FamilyPage,
});

const trafficRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/traffic",
  component: TrafficPage,
});

const profileRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/profile",
  component: ProfilePage,
});

// ─── Router tree ────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  loginRoute,
  registerRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  layoutRoute.addChildren([
    dashboardRoute,
    plansRoute,
    checkoutRoute,
    subscriptionsRoute,
    subscriptionDetailRoute,
    familyRoute,
    trafficRoute,
    profileRoute,
  ]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
