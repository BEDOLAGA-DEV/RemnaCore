/**
 * Centralized TanStack Query key factory.
 * Follows the pattern: entity > scope > params.
 */
export const QUERY_KEYS = {
  auth: {
    me: ["auth", "me"] as const,
  },
  plans: {
    all: ["plans"] as const,
    detail: (id: string) => ["plans", id] as const,
  },
  subscriptions: {
    all: ["subscriptions"] as const,
    detail: (id: string) => ["subscriptions", id] as const,
    bindings: (id: string) => ["subscriptions", id, "bindings"] as const,
  },
  invoices: {
    all: ["invoices"] as const,
  },
  bindings: {
    all: ["bindings"] as const,
  },
  family: {
    mine: ["family"] as const,
  },
  admin: {
    users: {
      all: (limit?: number, offset?: number) =>
        ["admin", "users", { limit, offset }] as const,
      detail: (id: string) => ["admin", "users", id] as const,
    },
    subscriptions: {
      all: (limit?: number, offset?: number) =>
        ["admin", "subscriptions", { limit, offset }] as const,
    },
    invoices: {
      all: (limit?: number, offset?: number) =>
        ["admin", "invoices", { limit, offset }] as const,
    },
    plugins: {
      all: ["admin", "plugins"] as const,
      detail: (id: string) => ["admin", "plugins", id] as const,
    },
    tenants: {
      root: ["admin", "tenants"] as const,
      all: (limit?: number, offset?: number) =>
        ["admin", "tenants", { limit, offset }] as const,
      detail: (id: string) => ["admin", "tenants", id] as const,
    },
  },
} as const;
