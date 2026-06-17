/**
 * Central registry of every backend API endpoint.
 * Frontend code MUST import paths from here — never hardcode URLs.
 */
export const ENDPOINTS = {
  // ─── First-run setup (Public) ──────────────────────────────────────────
  setup: {
    status: "/api/setup/status",
    admin: "/api/setup/admin",
  },

  // ─── Auth (Public) ──────────────────────────────────────────────────────
  auth: {
    register: "/api/auth/register",
    login: "/api/auth/login",
    verifyEmail: "/api/auth/verify-email",
    refresh: "/api/auth/refresh",
    forgotPassword: "/api/auth/forgot-password",
    resetPassword: "/api/auth/reset-password",
    // Public: invitation acceptance (Phase B). Rate-limited; no JWT required.
    acceptInvitation: "/api/auth/accept-invitation",
  },

  // ─── Plans (Public) ────────────────────────────────────────────────────
  plans: {
    list: "/api/plans",
    detail: (planId: string) => `/api/plans/${planId}`,
  },

  // ─── Profile (Protected) ───────────────────────────────────────────────
  me: {
    get: "/api/me",
    update: "/api/me",
    linkTelegram: "/api/me/link-telegram",
    unlinkTelegram: "/api/me/link-telegram",
  },

  // ─── Subscriptions (Protected) ─────────────────────────────────────────
  subscriptions: {
    create: "/api/subscriptions",
    list: "/api/subscriptions",
    detail: (subId: string) => `/api/subscriptions/${subId}`,
    cancel: (subId: string) => `/api/subscriptions/${subId}/cancel`,
    bindings: (subId: string) => `/api/subscriptions/${subId}/bindings`,
    addAddon: (subId: string) => `/api/subscriptions/${subId}/addons`,
    removeAddon: (subId: string, addonId: string) =>
      `/api/subscriptions/${subId}/addons/${addonId}`,
  },

  // ─── Checkout (Protected) ──────────────────────────────────────────────
  checkout: {
    start: "/api/checkout",
  },

  // ─── Invoices (Protected) ──────────────────────────────────────────────
  invoices: {
    list: "/api/invoices",
    pay: (invoiceId: string) => `/api/invoices/${invoiceId}/pay`,
  },

  // ─── Bindings (Protected) ──────────────────────────────────────────────
  bindings: {
    list: "/api/bindings",
  },

  // ─── Family (Protected) ────────────────────────────────────────────────
  family: {
    create: "/api/family",
    get: "/api/family",
    addMember: "/api/family/members",
    removeMember: (userId: string) => `/api/family/members/${userId}`,
  },

  // ─── IAM: Users + Invitations + Roles (Protected, Phase B) ───────────
  users: {
    // Direct user creation (users.invite permission).
    create: "/api/users",
    invitations: {
      list: "/api/users/invitations",
      create: "/api/users/invitations",
      revoke: (invitationId: string) => `/api/users/invitations/${invitationId}`,
    },
    roles: {
      list: (userId: string) => `/api/users/${userId}/roles`,
      assign: (userId: string) => `/api/users/${userId}/roles`,
      revoke: (userId: string) => `/api/users/${userId}/roles`,
    },
  },

  // ─── Admin ─────────────────────────────────────────────────────────────
  admin: {
    users: {
      list: "/api/admin/users",
      detail: (userId: string) => `/api/admin/users/${userId}`,
    },
    subscriptions: {
      list: "/api/admin/subscriptions",
    },
    invoices: {
      list: "/api/admin/invoices",
    },
    plugins: {
      list: "/api/admin/plugins",
      install: "/api/admin/plugins",
      detail: (pluginId: string) => `/api/admin/plugins/${pluginId}`,
      enable: (pluginId: string) => `/api/admin/plugins/${pluginId}/enable`,
      disable: (pluginId: string) => `/api/admin/plugins/${pluginId}/disable`,
      uninstall: (pluginId: string) => `/api/admin/plugins/${pluginId}`,
      updateConfig: (pluginId: string) =>
        `/api/admin/plugins/${pluginId}/config`,
    },
    tenants: {
      list: "/api/admin/tenants",
      create: "/api/admin/tenants",
      detail: (tenantId: string) => `/api/admin/tenants/${tenantId}`,
      updateBranding: (tenantId: string) =>
        `/api/admin/tenants/${tenantId}/branding`,
    },
    // Shop provisioning: platform-admin only (shops.manage + service-level guard).
    shops: {
      create: "/api/admin/shops",
    },
    pluginPages: "/api/admin/plugin-pages",
    sessions: "/api/admin/sessions",
    settings: "/api/admin/settings",
    stats: "/api/admin/stats",
    metrics: "/api/admin/metrics",
    history: "/api/admin/metrics/history",
    activity: "/api/admin/activity",
  },

  // ─── Remnawave Provider ────────────────────────────────────────────────
  remnawave: {
    overview: "/api/remnawave/dashboard/overview",
    realtime: "/api/remnawave/dashboard/realtime",
    nodes: "/api/remnawave/nodes",
    trafficByNode: "/api/remnawave/analytics/traffic-by-node",
  },

  // ─── Plugin RPC & Collections ──────────────────────────────────────────
  plugins: {
    rpc: (slug: string, functionName: string) =>
      `/api/plugins/${slug}/rpc/${functionName}`,
    collections: {
      list: (slug: string, collection: string) =>
        `/api/plugins/${slug}/collections/${collection}`,
      item: (slug: string, collection: string, id: string) =>
        `/api/plugins/${slug}/collections/${collection}/${id}`,
    },
  },

  // ─── Routing (Protected) ───────────────────────────────────────────────
  routing: {
    selectNode: "/api/routing/select",
  },

  // ─── Infrastructure ──────────────────────────────────────────────────
  infra: {
    readyz: "/readyz",
  },
} as const;
