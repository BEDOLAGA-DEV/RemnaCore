/**
 * Central registry of every backend API endpoint.
 * Frontend code MUST import paths from here — never hardcode URLs.
 */
export const ENDPOINTS = {
  // ─── Auth (Public) ──────────────────────────────────────────────────────
  auth: {
    register: "/api/auth/register",
    login: "/api/auth/login",
    verifyEmail: "/api/auth/verify-email",
    refresh: "/api/auth/refresh",
    forgotPassword: "/api/auth/forgot-password",
    resetPassword: "/api/auth/reset-password",
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
  },

  // ─── Routing (Protected) ───────────────────────────────────────────────
  routing: {
    selectNode: "/api/routing/select",
  },
} as const;
